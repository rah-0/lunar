package test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
	"github.com/rah-0/lunar/internal/storage"
)

// generateTestMessage creates a test message with the given parameters
func generateTestMessage(rocketID string, msgNum int, msgType string) models.Envelope {
	timestamp := time.Now().Add(time.Duration(rand.Intn(1000)) * time.Millisecond)
	envelope := models.Envelope{
		Metadata: struct {
			Channel       string    `json:"channel"`
			MessageNumber int       `json:"messageNumber"`
			MessageTime   time.Time `json:"messageTime"`
			MessageType   string    `json:"messageType"`
		}{
			Channel:       rocketID,
			MessageNumber: msgNum,
			MessageTime:   timestamp,
			MessageType:   msgType,
		},
		Message: models.MessageContent{
			Type:        "Falcon-9",
			LaunchSpeed: 500,
			Mission:     "BENCHMARK",
		},
	}

	switch msgType {
	case models.MessageTypeRocketSpeedIncreased:
		envelope.Message.By = 100
	case models.MessageTypeRocketSpeedDecreased:
		envelope.Message.By = 50
	case models.MessageTypeRocketExploded:
		envelope.Message.Reason = "Benchmark test explosion"
	case models.MessageTypeRocketMissionChanged:
		envelope.Message.NewMission = "NEW-BENCHMARK"
	}

	return envelope
}

// generateTestMessages creates a slice of test messages for a single rocket
func generateTestMessages(rocketID string, count int) []models.Envelope {
	messages := make([]models.Envelope, 0, count)

	// First message must be a launch
	messages = append(messages, generateTestMessage(rocketID, 1, models.MessageTypeRocketLaunched))

	// Generate remaining messages with random types
	for i := 2; i <= count; i++ {
		// Randomly select a message type (except launch which is only first)
		types := []string{
			models.MessageTypeRocketSpeedIncreased,
			models.MessageTypeRocketSpeedDecreased,
			models.MessageTypeRocketMissionChanged,
		}
		// Small chance to explode (1%)
		if i == count || (i > 10 && rand.Intn(100) == 0) {
			messages = append(messages, generateTestMessage(rocketID, i, models.MessageTypeRocketExploded))
			break
		}
		msgType := types[rand.Intn(len(types))]
		messages = append(messages, generateTestMessage(rocketID, i, msgType))
	}

	return messages
}

// processMessages sends messages to the repository with the specified concurrency
func processMessages(repo *storage.InMemoryRepository, messages []models.Envelope, concurrency int) {
	ctx := context.Background()
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for _, msg := range messages {
		msg := msg // Capture range variable

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func() {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			repo.ProcessMessage(ctx, msg)
		}()
	}

	wg.Wait()
}

// BenchmarkRepository benchmarks the repository with different message loads and concurrency levels
func BenchmarkRepository(b *testing.B) {
	// Test different message volumes
	messageCounts := []struct {
		name          string
		rockets       int
		msgsPerRocket int
	}{
		{"100msgs", 10, 10},     // 100 messages total
		{"1kmsgs", 10, 100},    // 1,000 messages total
		{"10kmsgs", 10, 1000},  // 10,000 messages total
	}

	// Test different concurrency levels
	workerCounts := []int{1, 2, 4}

	for _, msgCount := range messageCounts {
		for _, workers := range workerCounts {
			testName := fmt.Sprintf("%s_%dworkers", msgCount.name, workers)
			b.Run(testName, func(b *testing.B) {
				repo := storage.NewInMemoryRepository()

				// Generate test messages for all rockets
				var allMessages []models.Envelope
				for i := 0; i < msgCount.rockets; i++ {
					rocketID := fmt.Sprintf("rocket-%d-%s", i, msgCount.name)
					messages := generateTestMessages(rocketID, msgCount.msgsPerRocket)
					allMessages = append(allMessages, messages...)
				}

				// Shuffle messages to simulate real-world conditions
				rand.Shuffle(len(allMessages), func(i, j int) {
					allMessages[i], allMessages[j] = allMessages[j], allMessages[i]
				})


				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					processMessages(repo, allMessages, workers)
				}
			})
		}
	}
}

// TestMessageOrder verifies that messages are processed in the correct order
func TestMessageOrder(t *testing.T) {
	repo := storage.NewInMemoryRepository()
	rocketID := "test-rocket"

	// Generate test messages
	messages := generateTestMessages(rocketID, 1000)

	// Process messages with high concurrency
	processMessages(repo, messages, 128)

	// Verify the final state
	rocket, exists := repo.GetRocket(context.Background(), rocketID)
	if !exists {
		t.Fatal("Rocket not found after processing messages")
	}

	// Check if the last processed message number matches the expected count
	expectedLastMsg := len(messages)
	if rocket.LastProcessedMessageNumber != expectedLastMsg {
		t.Errorf("Expected last processed message %d, got %d", expectedLastMsg, rocket.LastProcessedMessageNumber)
	}

	// Verify that the rocket state is consistent
	if rocket.Exploded {
		t.Logf("Rocket exploded as expected with reason: %s", rocket.Reason)
	} else {
		t.Log("Rocket completed all messages without exploding")
	}
}
