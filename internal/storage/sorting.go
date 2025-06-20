package storage

import (
	"sort"
	"strings"

	"github.com/rah-0/lunar/internal/models"
)

// ValidSortFields defines the allowed fields for sorting rockets
var ValidSortFields = map[string]bool{
	"id":        true,
	"type":      true,
	"speed":     true,
	"mission":   true,
	"status":    true,
	"updatedat": true,
}

// ValidOrders defines the allowed sort orders
var ValidOrders = map[string]bool{
	"asc":  true,
	"desc": true,
}


type SortOptions struct {
	Field string
	Order string
}


func NewSortOptions() SortOptions {
	return SortOptions{
		Field: "id",
		Order: "asc",
	}
}


func sortRocketSummaries(summaries []models.RocketSummary, options SortOptions) {
	// Define the sort function based on the field and direction
	sortFunc := func(i, j int) bool {
		var result bool

		// Compare based on the specified field
		switch strings.ToLower(options.Field) {
		case "id":
			result = summaries[i].ID < summaries[j].ID
		case "type":
			result = summaries[i].Type < summaries[j].Type
		case "speed":
			result = summaries[i].Speed < summaries[j].Speed
		case "mission":
			result = summaries[i].Mission < summaries[j].Mission
		case "status":
			result = summaries[i].Status < summaries[j].Status
		case "updatedat":
			result = summaries[i].UpdatedAt.Before(summaries[j].UpdatedAt)
		default:
			// Default to sorting by ID
			result = summaries[i].ID < summaries[j].ID
		}

		// Reverse the result if descending order is requested
		if options.Order == "desc" {
			return !result
		}

		return result
	}

	// Sort the summaries
	sort.SliceStable(summaries, sortFunc)
}

// ParseSortOptions parses and validates sort and order parameters,
// falling back to defaults if invalid values are provided
func ParseSortOptions(sortField, order string) SortOptions {
	options := NewSortOptions()

	// Check if sort field is valid
	sortField = strings.ToLower(sortField)
	if sortField != "" && ValidSortFields[sortField] {
		options.Field = sortField
	}

	// Check if order is valid
	order = strings.ToLower(order)
	if order != "" && ValidOrders[order] {
		options.Order = order
	}

	return options
}
