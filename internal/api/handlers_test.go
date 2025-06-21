package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
	"github.com/rah-0/lunar/internal/storage"
)

// setupTestServer creates a test server with all routes registered
func setupTestServer(h *Handler) *httptest.Server {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

// decodeJSON is a generic helper to decode JSON responses in tests
func decodeJSON[T any](t *testing.T, body io.Reader) T {
	var out T
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	return out
}

func TestHandleMessages(t *testing.T) {
	repo := storage.NewInMemoryRepository()
	handler := NewHandler(repo)

	// Create a test message
	rocketID := "test-rocket-api"
	envelope := models.Envelope{
		Metadata: struct {
			Channel       string    `json:"channel"`
			MessageNumber int       `json:"messageNumber"`
			MessageTime   time.Time `json:"messageTime"`
			MessageType   string    `json:"messageType"`
		}{
			Channel:       rocketID,
			MessageNumber: 1,
			MessageTime:   time.Now(),
			MessageType:   models.MessageTypeRocketLaunched,
		},
		Message: models.MessageContent{
			Type:        "Falcon-9",
			LaunchSpeed: 500,
			Mission:     "API-TEST",
		},
	}

	// Convert message to JSON
	payload, _ := json.Marshal(envelope)

	// Create a test server with all routes registered
	testServer := setupTestServer(handler)
	defer testServer.Close()

	// Make an HTTP POST request to the test server
	resp, err := http.Post(
		testServer.URL+"/messages",
		"application/json",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status code %d, got %d", http.StatusAccepted, resp.StatusCode)
	}

	// Verify the message was processed
	rocket, exists := repo.GetRocket(context.Background(), rocketID)
	if !exists {
		t.Errorf("Expected rocket to be created")
	} else {
		if rocket.Type != "Falcon-9" || rocket.Speed != 500 || rocket.Mission != "API-TEST" {
			t.Errorf("Rocket state incorrect: type=%s, speed=%d, mission=%s",
				rocket.Type, rocket.Speed, rocket.Mission)
		}
	}
}

func TestHandleGetRocket(t *testing.T) {
	repo := storage.NewInMemoryRepository()
	handler := NewHandler(repo)

	// Create a unique ID for this test
	rocketID := "test-rocket-" + fmt.Sprint(time.Now().UnixNano())

	// Create a rocket message
	envelope := models.Envelope{
		Metadata: struct {
			Channel       string    `json:"channel"`
			MessageNumber int       `json:"messageNumber"`
			MessageTime   time.Time `json:"messageTime"`
			MessageType   string    `json:"messageType"`
		}{
			Channel:       rocketID,
			MessageNumber: 1,
			MessageTime:   time.Now(),
			MessageType:   models.MessageTypeRocketLaunched,
		},
		Message: models.MessageContent{
			Type:        "Falcon-Heavy",
			LaunchSpeed: 600,
			Mission:     "GET-TEST",
		},
	}

	// Process the message to create the rocket
	repo.ProcessMessage(context.Background(), envelope)

	// Create a test server with all routes registered
	testServer := setupTestServer(handler)
	defer testServer.Close()

	// Make a real HTTP request to the test server
	response, err := http.Get(testServer.URL + "/rockets/" + rocketID)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer response.Body.Close()

	// Check response
	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.StatusCode)
	}

	// Parse the response body using the helper
	rocketData := decodeJSON[map[string]any](t, response.Body)

	// Validate the rocket data
	if id, ok := rocketData["id"].(string); !ok || id != rocketID {
		t.Errorf("Rocket ID incorrect, got %v", rocketData["id"])
	}

	if rocketType, ok := rocketData["type"].(string); !ok || rocketType != "Falcon-Heavy" {
		t.Errorf("Rocket type incorrect, got %v", rocketData["type"])
	}

	if speed, ok := rocketData["speed"].(float64); !ok || int(speed) != 600 {
		t.Errorf("Rocket speed incorrect, got %v", rocketData["speed"])
	}

	if mission, ok := rocketData["mission"].(string); !ok || mission != "GET-TEST" {
		t.Errorf("Rocket mission incorrect, got %v", rocketData["mission"])
	}
}

func TestHandleListRockets(t *testing.T) {
	repo := storage.NewInMemoryRepository()
	handler := NewHandler(repo)

	// Create test rockets with different speeds
	rocketIDs := []string{"list-test-1", "list-test-2", "list-test-3"}
	speeds := []int{300, 100, 200}

	for i, id := range rocketIDs {
		envelope := models.Envelope{
			Metadata: struct {
				Channel       string    `json:"channel"`
				MessageNumber int       `json:"messageNumber"`
				MessageTime   time.Time `json:"messageTime"`
				MessageType   string    `json:"messageType"`
			}{
				Channel:       id,
				MessageNumber: 1,
				MessageTime:   time.Now(),
				MessageType:   models.MessageTypeRocketLaunched,
			},
			Message: models.MessageContent{
				Type:        "Test-Rocket",
				LaunchSpeed: speeds[i],
				Mission:     "LIST-TEST",
			},
		}
		repo.ProcessMessage(context.Background(), envelope)
	}

	// Create a test server with all routes registered
	testServer := setupTestServer(handler)
	defer testServer.Close()

	// Make a request with sort parameters
	response, err := http.Get(testServer.URL + "/rockets?sort=speed&order=asc")
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer response.Body.Close()

	// Check response
	if response.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, response.StatusCode)
	}

	// Parse the response body using the helper
	rockets := decodeJSON[[]models.RocketSummary](t, response.Body)

	// Check that we got all rockets
	if len(rockets) != len(rocketIDs) {
		t.Errorf("Expected %d rockets, got %d", len(rocketIDs), len(rockets))
	}

	// Check that they're sorted by speed (ascending)
	if len(rockets) >= 3 {
		if rockets[0].Speed > rockets[1].Speed || rockets[1].Speed > rockets[2].Speed {
			t.Errorf("Rockets not sorted by speed ascending: %v", rockets)
		}
	}

	// Test sorting in descending order
	responseDesc, err := http.Get(testServer.URL + "/rockets?sort=speed&order=desc")
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer responseDesc.Body.Close()

	// Check response
	if responseDesc.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, responseDesc.StatusCode)
	}

	// Parse the response body for descending order test
	rocketsDesc := decodeJSON[[]models.RocketSummary](t, responseDesc.Body)

	// Check descending order
	if len(rocketsDesc) >= 3 {
		if rocketsDesc[0].Speed < rocketsDesc[1].Speed || rocketsDesc[1].Speed < rocketsDesc[2].Speed {
			t.Errorf("Rockets not sorted by speed descending: %v", rocketsDesc)
		}
	}
}
