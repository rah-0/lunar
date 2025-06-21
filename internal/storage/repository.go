package storage

import (
	"container/heap"
	"context"

	"github.com/rah-0/lunar/internal/models"
	"golang.org/x/sync/semaphore"
)

// RocketRepository defines the interface for rocket data access
type RocketRepository interface {
	// GetRocket retrieves a rocket by its ID
	GetRocket(ctx context.Context, id string) (*models.RocketState, bool)

	// ListRockets returns all rockets, optionally sorted
	ListRockets(ctx context.Context, sortField, order string) ([]models.RocketSummary, error)

	// ProcessMessage processes a rocket message using the Envelope format
	ProcessMessage(ctx context.Context, envelope models.Envelope) bool
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

// rocketEntry groups together a rocket's state and its message buffer
type rocketEntry struct {
	State  *models.RocketState
	Buffer *MessageBuffer
	Mu     *ContextMutex // Protects both State and Buffer
}

// InMemoryRepository is an in-memory implementation of RocketRepository
type InMemoryRepository struct {
	mu      *ContextMutex // Protects the rockets map only
	rockets map[string]*rocketEntry
}

// NewInMemoryRepository creates a new in-memory repository
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		mu:      NewContextMutex(),
		rockets: make(map[string]*rocketEntry),
	}
}

func (r *InMemoryRepository) GetRocket(ctx context.Context, id string) (*models.RocketState, bool) {
	// Check if context is done before acquiring locks
	if err := ctx.Err(); err != nil {
		return nil, false
	}

	// Get a read lock on the repository
	if err := r.mu.Lock(ctx); err != nil {
		return nil, false
	}

	entry, exists := r.rockets[id]
	if !exists {
		r.mu.Unlock()
		return nil, false
	}

	// Get a read lock on the entry
	if err := entry.Mu.Lock(ctx); err != nil {
		r.mu.Unlock()
		return nil, false
	}

	// Create a deep copy of the state without the mutex
	rocketCopy := &models.RocketState{
		ID:                         entry.State.ID,
		Type:                       entry.State.Type,
		Speed:                      entry.State.Speed,
		Mission:                    entry.State.Mission,
		Exploded:                   entry.State.Exploded,
		Reason:                     entry.State.Reason,
		UpdatedAt:                  entry.State.UpdatedAt,
		CreatedAt:                  entry.State.CreatedAt,
		LastProcessedMessageNumber: entry.State.LastProcessedMessageNumber,
	}

	// Unlock in reverse order of locking
	entry.Mu.Unlock()
	r.mu.Unlock()

	return rocketCopy, true
}

// ContextMutex is a context-aware mutex that can be cancelled
// It uses semaphore.Weighted under the hood to support context cancellation
type ContextMutex struct {
	sem *semaphore.Weighted
}

// NewContextMutex creates a new context-aware mutex
func NewContextMutex() *ContextMutex {
	return &ContextMutex{
		sem: semaphore.NewWeighted(1),
	}
}

// Lock acquires the lock, blocking until it is available or the context is cancelled
func (m *ContextMutex) Lock(ctx context.Context) error {
	return m.sem.Acquire(ctx, 1)
}

// TryLock attempts to acquire the lock without blocking
func (m *ContextMutex) TryLock() bool {
	return m.sem.TryAcquire(1)
}

// Unlock releases the lock
func (m *ContextMutex) Unlock() {
	m.sem.Release(1)
}

func (r *InMemoryRepository) ListRockets(ctx context.Context, sortField, order string) ([]models.RocketSummary, error) {
	// Check if context is done before acquiring locks
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Get a read lock on the repository
	if err := r.mu.Lock(ctx); err != nil {
		return nil, ctx.Err()
	}
	defer r.mu.Unlock()

	// Pre-allocate slice with exact capacity needed
	summaries := make([]models.RocketSummary, 0, len(r.rockets))

	// Process each entry with its own lock
	for _, entry := range r.rockets {
		// Check if context is done before processing each entry
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// Try to acquire the lock with context
		if err := entry.Mu.Lock(ctx); err != nil {
			return nil, ctx.Err()
		}

		// Process the entry while holding the lock
		state := entry.State
		status := "active"
		if state.Exploded {
			status = "exploded"
		}

		summaries = append(summaries, models.RocketSummary{
			ID:        state.ID,
			Type:      state.Type,
			Speed:     state.Speed,
			Mission:   state.Mission,
			Status:    status,
			UpdatedAt: state.UpdatedAt,
		})

		// Unlock immediately after processing the entry
		entry.Mu.Unlock()
	}

	// Sort the results if needed
	if sortField != "" {
		sortRocketSummaries(summaries, sortField, order)
	}

	return summaries, nil
}

// sortRocketSummaries sorts the rocket summaries based on the provided field and order
func sortRocketSummaries(summaries []models.RocketSummary, field, order string) {
	options := ParseSortOptions(field, order)
	// Call the sorting function from sorting.go
	SortRocketSummaries(summaries, options)
}

// ProcessMessage processes a rocket message using the Envelope
func (r *InMemoryRepository) ProcessMessage(ctx context.Context, envelope models.Envelope) bool {
	// Check if context is done before processing the message
	if err := ctx.Err(); err != nil {
		return false
	}

	// Get the rocket ID from the envelope
	rocketID := envelope.Metadata.Channel

	// Get a write lock on the repository
	if err := r.mu.Lock(ctx); err != nil {
		return false
	}

	// Get or create the rocket entry
	entry, exists := r.rockets[rocketID]
	if !exists {
		// Create a new rocket state
		state := &models.RocketState{
			ID:       rocketID,
			Type:     envelope.Message.Type,
			Mission:  "",
			Speed:    0,
			Exploded: false,
		}

		// Create a new message buffer
		buffer := &MessageBuffer{}
		heap.Init(buffer)

		// Create a new entry with a new context mutex
		entry = &rocketEntry{
			State:  state,
			Buffer: buffer,
			Mu:     NewContextMutex(),
		}
		r.rockets[rocketID] = entry
	}

	// We can unlock the repository mutex now that we have the entry
	r.mu.Unlock()

	// Get the update function for this message type
	updateFunc := r.getUpdateFuncForMessage(envelope)
	if updateFunc == nil {
		return false
	}

	// Process the message with proper ordering
	msgCtx := MessageContext{
		ID:         rocketID,
		Envelope:   envelope,
		UpdateFunc: updateFunc,
		Ctx:        ctx, // Pass through the original context
	}

	// Process the message with ordering
	return r.processMessageWithOrdering(entry, msgCtx)
}

// MessageContext groups related message processing parameters
type MessageContext struct {
	ID         string
	Envelope   models.Envelope
	UpdateFunc func(*models.RocketState) bool
	Ctx        context.Context // Original context from the request
}

// processMessageWithOrdering processes messages in correct sequence using buffering
func (r *InMemoryRepository) processMessageWithOrdering(entry *rocketEntry, ctx MessageContext) bool {
	// Lock the entry for the duration of processing
	if err := entry.Mu.Lock(ctx.Ctx); err != nil {
		return false
	}
	defer entry.Mu.Unlock()

	rocket := entry.State
	msgNum := ctx.Envelope.GetMessageNumber()

	// If rocket has exploded, only allow relaunch messages
	if rocket.Exploded && ctx.Envelope.GetMessageType() != models.MessageTypeRocketLaunched {
		return false
	}

	// Check if this is a duplicate or old message
	if msgNum <= rocket.LastProcessedMessageNumber {
		return false
	}

	// Check if this is the next expected message
	expectedMsgNum := rocket.LastProcessedMessageNumber + 1

	// If this is the next expected message, process it immediately
	if msgNum == expectedMsgNum {
		// Apply the update
		if !ctx.UpdateFunc(rocket) {
			return false
		}
		rocket.LastProcessedMessageNumber = msgNum
		rocket.UpdatedAt = ctx.Envelope.GetMessageTime()

		// If rocket exploded, clean up its buffer
		if rocket.Exploded {
			entry.Buffer = &MessageBuffer{} // Clear the buffer
			heap.Init(entry.Buffer)         // Initialize the new buffer
			return true
		}

		// Process any buffered messages that can now be applied
		r.processBufferedMessages(entry)
		return true
	}

	// If we get here, the message is out of order and needs to be buffered
	return r.bufferMessage(entry, ctx.Envelope)
}

// bufferMessage adds a message to the buffer in a thread-safe way
func (r *InMemoryRepository) bufferMessage(entry *rocketEntry, envelope models.Envelope) bool {
	// Create a copy of the envelope to avoid data races
	envCopy := envelope
	heap.Push(entry.Buffer, &envCopy)
	return true
}

// processBufferedMessages processes any buffered messages that can now be applied
// in the correct order. It processes messages in sequence starting from the next
// expected message number.
func (r *InMemoryRepository) processBufferedMessages(entry *rocketEntry) {
	rocket := entry.State
	buffer := entry.Buffer

	for buffer.Len() > 0 {
		// Peek at the next message without removing it
		nextMsg := (*buffer)[0]
		expectedMsgNum := rocket.LastProcessedMessageNumber + 1

		// If the next message is not the one we expect, stop processing
		if nextMsg.GetMessageNumber() != expectedMsgNum {
			break
		}

		// Get the update function for this message type
		updateFunc := r.getUpdateFuncForMessage(*nextMsg)
		if updateFunc == nil {
			// Remove the message we can't process
			heap.Pop(buffer)
			continue
		}

		// Apply the update
		if !updateFunc(rocket) {
			// If the update fails, remove the message and continue
			heap.Pop(buffer)
			continue
		}

		// Update the last processed message number
		rocket.LastProcessedMessageNumber = expectedMsgNum
		rocket.UpdatedAt = nextMsg.GetMessageTime()

		// Remove the processed message from the buffer
		heap.Pop(buffer)

		// If the rocket exploded, clear the buffer and stop processing
		if rocket.Exploded {
			entry.Buffer = &MessageBuffer{}
			heap.Init(entry.Buffer) // Initialize the new buffer
			break
		}
	}
}

// getUpdateFuncForMessage returns the appropriate update function for a message type
func (r *InMemoryRepository) getUpdateFuncForMessage(msg models.Envelope) func(*models.RocketState) bool {
	switch msg.GetMessageType() {
	case models.MessageTypeRocketLaunched:
		return func(rocket *models.RocketState) bool {
			if msg.Message.Type == "" || msg.Message.Mission == "" {
				return false
			}
			rocket.Type = msg.Message.Type
			rocket.Mission = msg.Message.Mission
			rocket.Speed = msg.Message.LaunchSpeed
			rocket.CreatedAt = msg.GetMessageTime()
			rocket.Exploded = false
			rocket.Reason = ""
			return true
		}

	case models.MessageTypeRocketSpeedIncreased:
		return func(rocket *models.RocketState) bool {
			if msg.Message.By <= 0 {
				return false
			}
			rocket.Speed += msg.Message.By
			return true
		}

	case models.MessageTypeRocketSpeedDecreased:
		return func(rocket *models.RocketState) bool {
			if msg.Message.By <= 0 {
				return false
			}
			rocket.Speed -= msg.Message.By
			if rocket.Speed < 0 {
				rocket.Speed = 0
			}
			return true
		}

	case models.MessageTypeRocketExploded:
		return func(rocket *models.RocketState) bool {
			if msg.Message.Reason == "" {
				return false
			}
			rocket.Exploded = true
			rocket.Reason = msg.Message.Reason
			return true
		}

	case models.MessageTypeRocketMissionChanged:
		return func(rocket *models.RocketState) bool {
			if msg.Message.NewMission == "" {
				return false
			}
			rocket.Mission = msg.Message.NewMission
			return true
		}

	default:
		return nil // Unknown message type
	}
}
