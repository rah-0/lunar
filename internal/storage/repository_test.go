package storage

import (
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
)

func TestProcessMessageOutOfOrder(t *testing.T) {
	repo := NewInMemoryRepository()

	// Create test messages with different message numbers
	rocketID := "test-rocket-123"
	launchTime := time.Now()

	// First, let's test the correct buffering of an out-of-order message

	// Message 2 - should be buffered since we haven't seen message 1 yet
	speedIncreaseMsg := createSpeedIncreaseMessage(rocketID, 2, launchTime.Add(1*time.Second), 1000)

	// Process message 2 first (out of order)
	processed := repo.ProcessMessage(speedIncreaseMsg)

	// Should be accepted and buffered
	if !processed {
		t.Errorf("Expected speed increase message to be buffered, but it was rejected")
	}

	// Verify rocket was created but speed is NOT increased yet (message is buffered)
	rocket, exists := repo.GetRocket(rocketID)
	if !exists {
		t.Errorf("Expected rocket to exist after processing speed increase message")
		return
	}

	// Speed should be 0 because message 2 is buffered waiting for message 1
	if rocket.Speed != 0 {
		t.Errorf("Expected rocket speed to be 0 while message 2 is buffered, got %d", rocket.Speed)
	}

	// Now send message 1 - should be processed and then message 2 should be applied from the buffer
	launchMsg := createLaunchMessage(rocketID, 1, launchTime, "Falcon-9", 500, "TEST-MISSION")
	processed = repo.ProcessMessage(launchMsg)

	// Should be processed
	if !processed {
		t.Errorf("Expected launch message to be processed, but it was rejected")
	}

	// Verify rocket state is updated from BOTH messages (message 1 and then buffered message 2)
	rocket, _ = repo.GetRocket(rocketID)

	// Check that message 1 was applied (launched rocket)
	if rocket.Type != "Falcon-9" {
		t.Errorf("Expected rocket type to be set to Falcon-9, got %s", rocket.Type)
	}

	// Check that message 2 was applied from buffer (speed increased)
	if rocket.Speed != 1500 { // 500 from launch + 1000 from speed increase
		t.Errorf("Expected rocket speed to be 1500 after buffered message applied, got %d", rocket.Speed)
	}

	// Verify both messages were marked as processed
	if rocket.LastProcessedMessageNumber != 2 {
		t.Errorf("Expected LastProcessedMessageNumber to be 2, got %d", rocket.LastProcessedMessageNumber)
	}

	// Now try to process a duplicate message (same number as already processed)
	duplicateMsg := createLaunchMessage(rocketID, 1, launchTime, "Duplicate-Rocket", 999, "DUPLICATE")
	processed = repo.ProcessMessage(duplicateMsg)

	// Should be ignored as a duplicate
	if processed {
		t.Errorf("Expected duplicate message to be ignored, but it was processed")
	}

	// Verify rocket state was not changed by duplicate message
	rocket, _ = repo.GetRocket(rocketID)
	if rocket.Mission == "DUPLICATE" {
		t.Errorf("Duplicate message incorrectly modified rocket state")
	}
}

func TestProcessDuplicateMessages(t *testing.T) {
	repo := NewInMemoryRepository()

	// Create test messages
	rocketID := "test-rocket-456"
	launchTime := time.Now()

	// Launch rocket
	launchMsg := createLaunchMessage(rocketID, 1, launchTime, "Falcon-Heavy", 700, "DUPLICATE-TEST")

	// Process original message
	processed := repo.ProcessMessage(launchMsg)
	if !processed {
		t.Errorf("Expected launch message to be processed, but it was rejected")
	}

	// Verify initial state
	_, exists := repo.GetRocket(rocketID)
	if !exists {
		t.Errorf("Expected rocket to exist after processing launch message")
		return
	}

	// Try to process the same message again
	processed = repo.ProcessMessage(launchMsg)
	if processed {
		t.Errorf("Expected duplicate message to be rejected, but it was processed")
	}

	// Verify rocket state hasn't changed
	rocketState, _ := repo.GetRocket(rocketID)
	if rocketState.Speed != 700 {
		t.Errorf("Expected rocket speed to remain 700, got %d", rocketState.Speed)
	}

	// Process new message with higher message number
	speedIncreaseMsg := createSpeedIncreaseMessage(rocketID, 2, launchTime.Add(1*time.Second), 300)
	processed = repo.ProcessMessage(speedIncreaseMsg)
	if !processed {
		t.Errorf("Expected speed increase message to be processed, but it was rejected")
	}

	// Verify rocket state updated
	rocketState, _ = repo.GetRocket(rocketID)
	if rocketState.Speed != 1000 { // 700 + 300
		t.Errorf("Expected rocket speed to be 1000, got %d", rocketState.Speed)
	}

	// Try to process a duplicate of the second message
	processed = repo.ProcessMessage(speedIncreaseMsg)
	if processed {
		t.Errorf("Expected duplicate speed increase message to be rejected, but it was processed")
	}
}

func TestRocketLifecycle(t *testing.T) {
	repo := NewInMemoryRepository()

	// Create test rocket
	rocketID := "test-rocket-789"
	launchTime := time.Now()

	// Launch rocket
	launchMsg := createLaunchMessage(rocketID, 1, launchTime, "Falcon-9", 500, "LIFECYCLE-TEST")
	repo.ProcessMessage(launchMsg)

	// Increase speed
	speedIncreaseMsg := createSpeedIncreaseMessage(rocketID, 2, launchTime.Add(1*time.Second), 500)
	repo.ProcessMessage(speedIncreaseMsg)

	// Change mission
	missionChangeMsg := createMissionChangeMessage(rocketID, 3, launchTime.Add(2*time.Second), "NEW-MISSION")
	repo.ProcessMessage(missionChangeMsg)

	// Check state
	rocket, exists := repo.GetRocket(rocketID)
	if !exists {
		t.Errorf("Expected rocket to exist")
		return
	}

	if rocket.Speed != 1000 || rocket.Mission != "NEW-MISSION" {
		t.Errorf("Rocket state incorrect: speed=%d, mission=%s", rocket.Speed, rocket.Mission)
	}

	// Explode rocket
	explodeMsg := createExplodeMessage(rocketID, 4, launchTime.Add(3*time.Second), "ENGINE_FAILURE")
	repo.ProcessMessage(explodeMsg)

	// Check state
	rocket, _ = repo.GetRocket(rocketID)
	if !rocket.Exploded || rocket.Reason != "ENGINE_FAILURE" {
		t.Errorf("Expected rocket to be exploded with reason ENGINE_FAILURE")
	}

	// Try to increase speed after explosion (should be rejected)
	finalSpeedMsg := createSpeedIncreaseMessage(rocketID, 5, launchTime.Add(4*time.Second), 200)
	processed := repo.ProcessMessage(finalSpeedMsg)

	if processed {
		t.Errorf("Expected message to be rejected after explosion")
	}

	// Check final state
	rocket, _ = repo.GetRocket(rocketID)
	if rocket.Speed != 1000 { // Should remain unchanged
		t.Errorf("Expected rocket speed to remain 1000 after explosion, got %d", rocket.Speed)
	}
}

// Helper functions to create test messages

func createLaunchMessage(rocketID string, msgNum int, msgTime time.Time, rocketType string, speed int, mission string) models.Envelope {
	var env models.Envelope
	env.Metadata.Channel = rocketID
	env.Metadata.MessageNumber = msgNum
	env.Metadata.MessageTime = msgTime
	env.Metadata.MessageType = models.MessageTypeRocketLaunched
	env.Message = models.MessageContent{
		Type:        rocketType,
		LaunchSpeed: speed,
		Mission:     mission,
	}
	return env
}

func createSpeedIncreaseMessage(rocketID string, msgNum int, msgTime time.Time, speedIncrease int) models.Envelope {
	var env models.Envelope
	env.Metadata.Channel = rocketID
	env.Metadata.MessageNumber = msgNum
	env.Metadata.MessageTime = msgTime
	env.Metadata.MessageType = models.MessageTypeRocketSpeedIncreased
	env.Message = models.MessageContent{
		By: speedIncrease,
	}
	return env
}

func createMissionChangeMessage(rocketID string, msgNum int, msgTime time.Time, newMission string) models.Envelope {
	var env models.Envelope
	env.Metadata.Channel = rocketID
	env.Metadata.MessageNumber = msgNum
	env.Metadata.MessageTime = msgTime
	env.Metadata.MessageType = models.MessageTypeRocketMissionChanged
	env.Message = models.MessageContent{
		NewMission: newMission,
	}
	return env
}

func createExplodeMessage(rocketID string, msgNum int, msgTime time.Time, reason string) models.Envelope {
	var env models.Envelope
	env.Metadata.Channel = rocketID
	env.Metadata.MessageNumber = msgNum
	env.Metadata.MessageTime = msgTime
	env.Metadata.MessageType = models.MessageTypeRocketExploded
	env.Message = models.MessageContent{
		Reason: reason,
	}
	return env
}

// Removed unused function createSpeedDecreaseMessage
