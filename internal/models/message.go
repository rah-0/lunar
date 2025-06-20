package models

import "time"

// Envelope is the structure for all rocket messages
type Envelope struct {
	// Message metadata as a nested structure
	Metadata struct {
		Channel       string    `json:"channel"`
		MessageNumber int       `json:"messageNumber"`
		MessageTime   time.Time `json:"messageTime"`
		MessageType   string    `json:"messageType"`
	} `json:"metadata"`

	// Message content with all possible fields
	Message MessageContent `json:"message"`
}

func (e *Envelope) GetChannel() string {
	return e.Metadata.Channel
}

func (e *Envelope) GetMessageNumber() int {
	return e.Metadata.MessageNumber
}

func (e *Envelope) GetMessageTime() time.Time {
	return e.Metadata.MessageTime
}

func (e *Envelope) GetMessageType() string {
	return e.Metadata.MessageType
}

// MessageContent contains all possible fields from any message type
type MessageContent struct {
	// RocketLaunched fields
	Type        string `json:"type,omitempty"`
	LaunchSpeed int    `json:"launchSpeed,omitempty"`
	Mission     string `json:"mission,omitempty"`

	// RocketSpeedIncreased/RocketSpeedDecreased fields
	By int `json:"by,omitempty"`

	// RocketExploded fields
	Reason string `json:"reason,omitempty"`

	// RocketMissionChanged fields
	NewMission string `json:"newMission,omitempty"`
}

// Message types constants
const (
	MessageTypeRocketLaunched       = "RocketLaunched"
	MessageTypeRocketSpeedIncreased = "RocketSpeedIncreased"
	MessageTypeRocketSpeedDecreased = "RocketSpeedDecreased"
	MessageTypeRocketExploded       = "RocketExploded"
	MessageTypeRocketMissionChanged = "RocketMissionChanged"
)

// Rocket status constants
const (
	RocketStatusActive   = "Active"
	RocketStatusExploded = "Exploded"
)
