package models

import "time"

// RocketState represents the current state of a rocket
type RocketState struct {
	ID        string    `json:"id"`               // Same as the channel ID
	Type      string    `json:"type"`             // Type of rocket (e.g., "Falcon-9")
	Speed     int       `json:"speed"`            // Current speed
	Mission   string    `json:"mission"`          // Current mission
	Exploded  bool      `json:"exploded"`         // Whether the rocket has exploded
	Reason    string    `json:"reason,omitempty"` // Reason for explosion, if applicable
	UpdatedAt time.Time `json:"updatedAt"`        // Last updated time
	CreatedAt time.Time `json:"createdAt"`        // Time when the rocket was first launched

	// Used for internal message ordering
	LastProcessedMessageNumber int `json:"-"`
}

// RocketSummary is a simplified version of RocketState for listing purposes
type RocketSummary struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Speed     int       `json:"speed"`
	Mission   string    `json:"mission"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}
