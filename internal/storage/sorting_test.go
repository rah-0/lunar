package storage

import (
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestSortRocketSummaries(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-1 * time.Hour)
	later := now.Add(1 * time.Hour)

	// Create test data
	summaries := []models.RocketSummary{
		{ID: "rocket3", Type: "Falcon", Speed: 300, Mission: "Mars", Status: models.RocketStatusActive, UpdatedAt: now},
		{ID: "rocket1", Type: "Saturn", Speed: 100, Mission: "Moon", Status: models.RocketStatusExploded, UpdatedAt: earlier},
		{ID: "rocket2", Type: "Atlas", Speed: 200, Mission: "Earth", Status: models.RocketStatusActive, UpdatedAt: later},
	}

	// Test cases
	testCases := []struct {
		name           string
		sortField      string
		order          string
		expectedFirst  string // ID of the rocket that should be first after sorting
		expectedSecond string
		expectedThird  string
	}{
		{"Default Sort", "", "", "rocket1", "rocket2", "rocket3"},
		{"Sort by ID Ascending", "id", "asc", "rocket1", "rocket2", "rocket3"},
		{"Sort by ID Descending", "id", "desc", "rocket3", "rocket2", "rocket1"},
		{"Sort by Type Ascending", "type", "asc", "rocket2", "rocket3", "rocket1"},
		{"Sort by Type Descending", "type", "desc", "rocket1", "rocket3", "rocket2"},
		{"Sort by Speed Ascending", "speed", "asc", "rocket1", "rocket2", "rocket3"},
		{"Sort by Speed Descending", "speed", "desc", "rocket3", "rocket2", "rocket1"},
		{"Sort by Mission Ascending", "mission", "asc", "rocket2", "rocket3", "rocket1"},
		{"Sort by Status Ascending", "status", "asc", "rocket3", "rocket2", "rocket1"},
		{"Sort by UpdatedAt Ascending", "updatedat", "asc", "rocket1", "rocket3", "rocket2"},
		{"Sort by UpdatedAt Descending", "updatedat", "desc", "rocket2", "rocket3", "rocket1"},
		{"Invalid Field Falls Back to ID", "invalid", "asc", "rocket1", "rocket2", "rocket3"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy of the original summaries to avoid test interference
			summariesCopy := make([]models.RocketSummary, len(summaries))
			copy(summariesCopy, summaries)

			// Apply sorting
			options := ParseSortOptions(tc.sortField, tc.order)
			sortRocketSummaries(summariesCopy, options)

			// Verify the order
			assert.Equal(t, tc.expectedFirst, summariesCopy[0].ID)
			assert.Equal(t, tc.expectedSecond, summariesCopy[1].ID)
			assert.Equal(t, tc.expectedThird, summariesCopy[2].ID)
		})
	}
}

func TestParseSortOptions(t *testing.T) {
	testCases := []struct {
		name          string
		sortField     string
		order         string
		expectedField string
		expectedOrder string
	}{
		{"Default Values", "", "", "id", "asc"},
		{"Valid Field and Order", "speed", "desc", "speed", "desc"},
		{"Valid Field with Default Order", "mission", "", "mission", "asc"},
		{"Invalid Field with Valid Order", "invalid", "desc", "id", "desc"},
		{"Valid Field with Invalid Order", "type", "invalid", "type", "asc"},
		{"Case Insensitivity", "ID", "DESC", "id", "desc"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse options
			options := ParseSortOptions(tc.sortField, tc.order)

			// Verify the parsed options
			assert.Equal(t, tc.expectedField, options.Field)
			assert.Equal(t, tc.expectedOrder, options.Order)
		})
	}
}
