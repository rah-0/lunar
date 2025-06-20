package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStringValue(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		data          map[string]any
		key           string
		expectedValue string
	}{
		{
			name: "String value exists",
			data: map[string]any{
				"name": "rocket",
			},
			key:           "name",
			expectedValue: "rocket",
		},
		{
			name:          "Key does not exist",
			data:          map[string]any{},
			key:           "name",
			expectedValue: "", // Default is empty string
		},
		{
			name: "Value is nil",
			data: map[string]any{
				"name": nil,
			},
			key:           "name",
			expectedValue: "", // Nil converts to empty string
		},
		{
			name: "Value is not a string",
			data: map[string]any{
				"name": 123,
			},
			key:           "name",
			expectedValue: "", // Non-string converts to empty string
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetStringValue(tc.data, tc.key)
			assert.Equal(t, tc.expectedValue, result)
		})
	}
}

func TestGetIntValue(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		data          map[string]any
		key           string
		expectedValue int
	}{
		{
			name: "Int value exists",
			data: map[string]any{
				"speed": 100,
			},
			key:           "speed",
			expectedValue: 100,
		},
		{
			name:          "Key does not exist",
			data:          map[string]any{},
			key:           "speed",
			expectedValue: 0, // Default is 0
		},
		{
			name: "Value is nil",
			data: map[string]any{
				"speed": nil,
			},
			key:           "speed",
			expectedValue: 0, // Nil converts to 0
		},
		{
			name: "Value is not a number",
			data: map[string]any{
				"speed": "fast",
			},
			key:           "speed",
			expectedValue: 0, // Non-number converts to 0
		},
		{
			name: "Value is float64",
			data: map[string]any{
				"speed": float64(100),
			},
			key:           "speed",
			expectedValue: 100,
		},
		{
			name: "Value is int64",
			data: map[string]any{
				"speed": int64(50),
			},
			key:           "speed",
			expectedValue: 50,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetIntValue(tc.data, tc.key)
			assert.Equal(t, tc.expectedValue, result)
		})
	}
}
