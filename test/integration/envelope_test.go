package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
	"github.com/stretchr/testify/require"
)

// TestEnvelopeMessageProcessing verifies the envelope message processing
func TestEnvelopeMessageProcessing(t *testing.T) {
	// The server is already started in TestMain
	t.Run("Envelope", func(t *testing.T) {
		// Create a unique channel ID for this test
		channelID := fmt.Sprintf("test-rocket-%d", time.Now().UnixNano())

		// FIRST MESSAGE: RocketLaunched
		launchJSON := fmt.Sprintf(`{
			"metadata": {
				"channel": "%s",
				"messageNumber": 1,
				"messageTime": "2025-06-19T23:38:00Z",
				"messageType": "RocketLaunched"
			},
			"message": {
				"type": "Falcon-9",
				"launchSpeed": 100,
				"mission": "Test Mission"
			}
		}`, channelID)

		// Send launch message
		respLaunch, err := http.Post(
			"http://localhost:8088/messages",
			"application/json",
			strings.NewReader(launchJSON),
		)
		require.NoError(t, err, "HTTP request failed")
		defer respLaunch.Body.Close()

		// Check response
		require.Equal(t, http.StatusAccepted, respLaunch.StatusCode, "Launch message failed")

		// SECOND MESSAGE: Speed increased
		speedJSON := fmt.Sprintf(`{
			"metadata": {
				"channel": "%s",
				"messageNumber": 2,
				"messageTime": "2025-06-19T23:38:10Z",
				"messageType": "RocketSpeedIncreased"
			},
			"message": {
				"by": 50
			}
		}`, channelID)

		// Send speed increase message
		respSpeed, err := http.Post(
			"http://localhost:8088/messages",
			"application/json",
			strings.NewReader(speedJSON),
		)
		require.NoError(t, err, "HTTP request failed")
		defer respSpeed.Body.Close()

		// Check response
		require.Equal(t, http.StatusAccepted, respSpeed.StatusCode, "Speed message failed")

		// Wait a moment for messages to be processed
		time.Sleep(100 * time.Millisecond)

		// Get rocket states to verify processing
		rockets, err := getRocketStates()
		require.NoError(t, err, "Failed to get rocket states")

		// Check that our test rocket has been created and updated
		var testRocket *models.RocketSummary
		for i := range rockets {
			if rockets[i].ID == channelID {
				testRocket = &rockets[i]
				break
			}
		}

		// Verify the rocket was found and has expected values
		require.NotNil(t, testRocket, "Test rocket not found")
		require.Equal(t, 150, testRocket.Speed, "Unexpected rocket speed")
		require.Equal(t, "Falcon-9", testRocket.Type, "Unexpected rocket type")
		require.Equal(t, "Test Mission", testRocket.Mission, "Unexpected mission name")
	})
}
