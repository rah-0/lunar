package storage

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"github.com/rah-0/lunar/internal/models"
)

// RocketRepository defines the interface for rocket data access
type RocketRepository interface {
	// GetRocket retrieves a rocket by its ID
	GetRocket(id string) (*models.RocketState, bool)

	// ListRockets returns all rockets, optionally sorted
	ListRockets(sortField, order string) []models.RocketSummary

	// ProcessMessage processes a rocket message using the Envelope format
	ProcessMessage(envelope models.Envelope) bool
}

// MessageBuffer is a priority queue for out-of-order messages
type MessageBuffer []*models.Envelope

// Implementation of heap.Interface
func (mb MessageBuffer) Len() int { return len(mb) }
func (mb MessageBuffer) Less(i, j int) bool {
	return mb[i].GetMessageNumber() < mb[j].GetMessageNumber()
}
func (mb MessageBuffer) Swap(i, j int) { mb[i], mb[j] = mb[j], mb[i] }

// Push adds an envelope to the message buffer
func (mb *MessageBuffer) Push(x any) {
	*mb = append(*mb, x.(*models.Envelope))
}

// Pop removes and returns the highest priority envelope from the buffer
func (mb *MessageBuffer) Pop() any {
	old := *mb
	n := len(old)
	x := old[n-1]
	*mb = old[0 : n-1]
	return x
}

type InMemoryRepository struct {
	rockets       map[string]*models.RocketState
	messageBuffer map[string]*MessageBuffer // Message buffer per rocket channel
	mutex         sync.RWMutex
}


func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		rockets:       make(map[string]*models.RocketState),
		messageBuffer: make(map[string]*MessageBuffer),
		mutex:         sync.RWMutex{},
	}
}

func (r *InMemoryRepository) GetRocket(id string) (*models.RocketState, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	rocket, exists := r.rockets[id]
	if !exists {
		return nil, false
	}

	// Prevent concurrent modification
	rocketCopy := *rocket
	return &rocketCopy, true
}

func (r *InMemoryRepository) ListRockets(sortField, order string) []models.RocketSummary {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Create a slice to hold all rocket summaries
	summaries := make([]models.RocketSummary, 0, len(r.rockets))

	// Populate the summaries
	for _, rocket := range r.rockets {
		status := models.RocketStatusActive
		if rocket.Exploded {
			status = models.RocketStatusExploded
		}

		summary := models.RocketSummary{
			ID:        rocket.ID,
			Type:      rocket.Type,
			Speed:     rocket.Speed,
			Mission:   rocket.Mission,
			Status:    status,
			UpdatedAt: rocket.UpdatedAt,
		}

		summaries = append(summaries, summary)
	}

	// Parse sort options and sort the results
	options := ParseSortOptions(sortField, order)
	sortRocketSummaries(summaries, options)

	return summaries
}

// updateRocketState is a helper function that handles the common rocket state update logic
func (r *InMemoryRepository) updateRocketState(id string, messageTime time.Time, updateFunc func(*models.RocketState) bool, envelope models.Envelope) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if rocket exists, create it if it doesn't
	rocket, exists := r.rockets[id]
	if !exists {
		// Create a new rocket state for any message type
		rocket = &models.RocketState{
			ID:                         id,
			Speed:                      0,
			Exploded:                   false,
			UpdatedAt:                  messageTime,
			CreatedAt:                  messageTime,
			LastProcessedMessageNumber: 0, // Initialize to 0
		}
		r.rockets[id] = rocket
	}

	// Create message context with all required parameters
	ctx := MessageContext{
		ID:         id,
		Envelope:   envelope,
		UpdateFunc: updateFunc,
	}

	// Process messages in correct order using buffering
	return r.processMessageWithOrdering(rocket, ctx)
}

// MessageContext groups related message processing parameters
type MessageContext struct {
	ID         string
	Envelope   models.Envelope
	UpdateFunc func(*models.RocketState) bool
}

// processMessageWithOrdering processes messages in correct sequence using buffering
func (r *InMemoryRepository) processMessageWithOrdering(rocket *models.RocketState, ctx MessageContext) bool {
	// Case 1: Message is a duplicate (already processed)
	if ctx.Envelope.GetMessageNumber() <= rocket.LastProcessedMessageNumber {
		// Log duplicate message
		fmt.Printf("Discarded duplicate message: channel=%s, messageNumber=%d, current=%d\n",
			ctx.ID, ctx.Envelope.GetMessageNumber(), rocket.LastProcessedMessageNumber)
		return false
	}

	// Case 2: This is the next expected message (can be applied immediately)
	expectedMsgNumber := rocket.LastProcessedMessageNumber + 1
	if ctx.Envelope.GetMessageNumber() == expectedMsgNumber {
		// Skip further processing if the rocket has exploded (except for relaunch)
		if rocket.Exploded && ctx.Envelope.GetMessageType() != models.MessageTypeRocketLaunched {
			// If rocket is exploded and this isn't a relaunch, clean up any buffered messages
			// to save memory - no point keeping them if they won't be applied
			r.cleanupBufferForExplodedRocket(ctx.ID)
			return false
		}

		// Apply this message
		processed := ctx.UpdateFunc(rocket)
		if processed {
			// Update metadata
			rocket.LastProcessedMessageNumber = ctx.Envelope.GetMessageNumber()
			rocket.UpdatedAt = ctx.Envelope.GetMessageTime()

			// Check if the rocket exploded from this message
			if rocket.Exploded {
				// If the rocket just exploded, clean up buffered messages to save memory
				r.cleanupBufferForExplodedRocket(ctx.ID)
			} else {
				// Process any buffered messages that might now be applicable
				r.processBufferedMessages(ctx.ID, rocket)
			}
		}
		return processed
	}

	// Case 3: Message is from the future (buffer it for later processing)

	// Only buffer the message if the rocket is not exploded (to save memory)
	// Exception: If this is a relaunch message, we should buffer it
	if !rocket.Exploded || ctx.Envelope.GetMessageType() == models.MessageTypeRocketLaunched {
		return r.bufferMessage(ctx.ID, rocket, ctx.Envelope)
	}
	// Rocket is exploded and this isn't a relaunch, no need to buffer
	fmt.Printf("Skipped buffering message for exploded rocket: channel=%s, messageNumber=%d\n",
		ctx.ID, ctx.Envelope.GetMessageNumber())
	return false
}

func (r *InMemoryRepository) bufferMessage(id string, rocket *models.RocketState, envelope models.Envelope) bool {

	// Initialize buffer for this channel if it doesn't exist
	buffer, exists := r.messageBuffer[id]
	if !exists {
		buffer = &MessageBuffer{}
		heap.Init(buffer)
		r.messageBuffer[id] = buffer
	}

	// Add the message to the priority queue (sorted by message number)
	heap.Push(buffer, &envelope)

	// Log buffered message
	fmt.Printf("Buffered out-of-order message: channel=%s, messageNumber=%d, expected=%d\n",
		id, envelope.GetMessageNumber(), rocket.LastProcessedMessageNumber+1)

	return true // Message was accepted (buffered)
}

// processBufferedMessages attempts to process any buffered messages that can now be applied
func (r *InMemoryRepository) processBufferedMessages(id string, rocket *models.RocketState) {
	// Get the message buffer for this rocket
	buffer, exists := r.messageBuffer[id]
	if !exists || buffer.Len() == 0 {
		return // No buffered messages
	}

	// Process messages in order
	processed := true
	for processed && buffer.Len() > 0 {
		// Peek at the next message (don't remove it yet)
		nextMsg := (*buffer)[0]

		// If the next message is not the one we expect, stop processing
		if nextMsg.GetMessageNumber() != rocket.LastProcessedMessageNumber+1 {
			break
		}

		// Pop the message from the buffer
		msg := heap.Pop(buffer).(*models.Envelope)

		// Apply the message to the rocket state
		processed = r.applyMessage(rocket, msg)

		if processed {
			// Update metadata
			rocket.LastProcessedMessageNumber = msg.GetMessageNumber()
			rocket.UpdatedAt = msg.GetMessageTime()

			// Log applied message
			fmt.Printf("Applied buffered message: channel=%s, messageNumber=%d\n", id, msg.GetMessageNumber())

			// If rocket exploded from a buffered message, stop processing and clean up remaining buffer
			if rocket.Exploded {
				r.cleanupBufferForExplodedRocket(id)
				break
			}
		}
	}

	// Clean up empty buffer
	if buffer.Len() == 0 {
		delete(r.messageBuffer, id)
	}
}

// cleanupBufferForExplodedRocket removes all buffered messages for an exploded rocket
// to free up memory - once a rocket has exploded, we don't need to process most messages
// (except potential future relaunch messages)
func (r *InMemoryRepository) cleanupBufferForExplodedRocket(id string) {
	buffer, exists := r.messageBuffer[id]
	if !exists || buffer.Len() == 0 {
		return // No buffered messages to clean up
	}

	// Keep only future relaunch messages (if any)
	preservedMessages := &MessageBuffer{}
	heap.Init(preservedMessages)

	for buffer.Len() > 0 {
		msg := heap.Pop(buffer).(*models.Envelope)

		// Preserve only relaunch messages
		if msg.GetMessageType() == models.MessageTypeRocketLaunched {
			heap.Push(preservedMessages, msg)
		} else {
			fmt.Printf("Discarded buffered message for exploded rocket: channel=%s, messageNumber=%d\n",
				id, msg.GetMessageNumber())
		}
	}

	// If we saved any messages, update the buffer, otherwise remove it
	if preservedMessages.Len() > 0 {
		r.messageBuffer[id] = preservedMessages
		fmt.Printf("Kept %d relaunch messages in buffer for exploded rocket: channel=%s\n",
			preservedMessages.Len(), id)
	} else {
		delete(r.messageBuffer, id)
		fmt.Printf("Removed entire message buffer for exploded rocket: channel=%s\n", id)
	}
}

func (r *InMemoryRepository) applyMessage(rocket *models.RocketState, msg *models.Envelope) bool {
	// Ensure we have the message content before applying
	if msg == nil {
		fmt.Printf("Error: Attempted to apply nil message\n")
		return false
	}

	switch msg.GetMessageType() {
	case models.MessageTypeRocketLaunched:
		// Verify we have the necessary message content
		if msg.Message.Type == "" || msg.Message.Mission == "" {
			fmt.Printf("Warning: Incomplete launch message content: type=%s, mission=%s\n",
				msg.Message.Type, msg.Message.Mission)
		}

		rocket.Type = msg.Message.Type
		rocket.Mission = msg.Message.Mission
		rocket.Speed = msg.Message.LaunchSpeed
		rocket.CreatedAt = msg.GetMessageTime()
		rocket.Exploded = false
		rocket.Reason = ""
		return true

	case models.MessageTypeRocketSpeedIncreased:
		rocket.Speed += msg.Message.By
		return true

	case models.MessageTypeRocketSpeedDecreased:
		rocket.Speed -= msg.Message.By
		if rocket.Speed < 0 {
			rocket.Speed = 0
		}
		return true

	case models.MessageTypeRocketExploded:
		rocket.Exploded = true
		rocket.Reason = msg.Message.Reason
		return true

	case models.MessageTypeRocketMissionChanged:
		rocket.Mission = msg.Message.NewMission
		return true

	default:
		return false // Unknown message type
	}
}
// ProcessMessage processes a rocket message using the Envelope
func (r *InMemoryRepository) ProcessMessage(envelope models.Envelope) bool {
	// Process the message passing the original envelope through for potential buffering
	return r.updateRocketState(
		envelope.GetChannel(),
		envelope.GetMessageTime(),
		func(rocket *models.RocketState) bool {
			// Process message based on its type
			switch envelope.GetMessageType() {
			case models.MessageTypeRocketLaunched:
				rocket.Type = envelope.Message.Type
				rocket.Mission = envelope.Message.Mission
				rocket.Speed = envelope.Message.LaunchSpeed
				rocket.CreatedAt = envelope.GetMessageTime()
				rocket.Exploded = false
				rocket.Reason = ""
				return true

			case models.MessageTypeRocketSpeedIncreased:
				rocket.Speed += envelope.Message.By
				return true

			case models.MessageTypeRocketSpeedDecreased:
				rocket.Speed -= envelope.Message.By
				if rocket.Speed < 0 {
					rocket.Speed = 0
				}
				return true

			case models.MessageTypeRocketExploded:
				rocket.Exploded = true
				rocket.Reason = envelope.Message.Reason
				return true

			case models.MessageTypeRocketMissionChanged:
				rocket.Mission = envelope.Message.NewMission
				return true

			default:
				return false // Unknown message type
			}
		},
		envelope,
	)
}
