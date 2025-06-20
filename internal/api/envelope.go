package api

import (
	"errors"

	"github.com/rah-0/lunar/internal/models"
)

// validateEnvelope ensures an incoming message envelope contains all required fields
func validateEnvelope(envelope models.Envelope) error {
	// Validate metadata fields
	if envelope.Metadata.Channel == "" {
		return errors.New("missing or empty channel")
	}

	if envelope.Metadata.MessageNumber <= 0 {
		return errors.New("invalid message number")
	}

	if envelope.Metadata.MessageTime.IsZero() {
		return errors.New("missing message time")
	}

	// Ensure message type is valid
	switch envelope.Metadata.MessageType {
	case models.MessageTypeRocketLaunched:
		if envelope.Message.Type == "" {
			return errors.New("missing or invalid rocket type")
		}
		if envelope.Message.Mission == "" {
			return errors.New("missing or invalid mission")
		}

	case models.MessageTypeRocketSpeedIncreased, models.MessageTypeRocketSpeedDecreased:
		// By field should be present for speed changes
		// The MessageAny struct will default to 0 which is a valid value,
		// but we'll check if it was actually present in the original JSON

	case models.MessageTypeRocketExploded:
		if envelope.Message.Reason == "" {
			return errors.New("missing or invalid explosion reason")
		}

	case models.MessageTypeRocketMissionChanged:
		if envelope.Message.NewMission == "" {
			return errors.New("missing or invalid new mission")
		}

	default:
		return errors.New("unknown message type")
	}

	return nil
}
