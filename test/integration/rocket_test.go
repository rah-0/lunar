package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runRocketTest runs a rocket test with the given parameters and returns the rocket process
// It handles both starting the server and the rockets process
func runRocketTest(t *testing.T, concurrency, maxMessages int, delay string, timeout time.Duration) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	// Start the server first
	sp, err := startServer(ctx)
	require.NoError(t, err, "Failed to start server")
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Server stderr: %s", sp.stderr.String())
			t.Logf("Server stdout: %s", sp.stdout.String())
		}
	})

	// Then start the rockets process
	rp, err := startRockets(ctx, concurrency, maxMessages, delay)
	require.NoError(t, err, "Failed to start rockets process")
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("Rockets stderr: %s", rp.stderr.String())
			t.Logf("Rockets stdout: %s", rp.stdout.String())
		}
	})

	// Wait for rockets to complete
	err = waitForRockets(rp, timeout)
	if err != nil {
		t.Logf("Rockets stderr: %s", rp.stderr.String())
		t.Logf("Rockets stdout: %s", rp.stdout.String())
		require.NoError(t, err, "Rockets process failed or timed out")
	}

	// Sleep briefly to ensure all messages are processed
	time.Sleep(500 * time.Millisecond)
}

// validateRockets validates that the rockets have valid states
func validateRockets(t *testing.T, rockets []models.RocketSummary, minCount int) {
	assert.GreaterOrEqual(t, len(rockets), minCount, "Expected at least %d rockets, got %d", minCount, len(rockets))

	for _, rocket := range rockets {
		assert.NotEmpty(t, rocket.ID, "Rocket has no ID")
		assert.NotEmpty(t, rocket.Status, "Rocket %s has no status", rocket.ID)
		assert.NotEmpty(t, rocket.Type, "Rocket %s has no type", rocket.ID)
		assert.Greater(t, rocket.Speed, 0, "Rocket %s has zero speed", rocket.ID)
	}
}

// mustGetRocketStates gets the rocket states and fails the test if there's an error
// This helper simplifies error handling for state retrieval
func mustGetRocketStates(t *testing.T) []models.RocketSummary {
	rockets, err := getRocketStates()
	require.NoError(t, err, "Failed to get rocket states")
	return rockets
}

// TestRocketMessageProcessing tests rocket message processing with different concurrency levels
func TestRocketMessageProcessing(t *testing.T) {
	// Table of test cases with different concurrency levels
	testCases := []struct {
		name        string
		concurrency int
		maxMessages int
		delay       string
		timeout     time.Duration
	}{
		{"SingleRocket", 1, 10, "50ms", 5 * time.Second},
		{"MultipleRockets", 5, 5, "100ms", 10 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Run the rocket test with the specified parameters
			runRocketTest(t, tc.concurrency, tc.maxMessages, tc.delay, tc.timeout)

			// Get and validate rocket states
			rockets := mustGetRocketStates(t)
			validateRockets(t, rockets, tc.concurrency)
		})
	}
}

// TestRawMessageFormat tests the raw message format
func TestRawMessageFormat(t *testing.T) {
	// Run the rocket test with the raw message format parameters
	runRocketTest(t, 1, 3, "100ms", 10*time.Second)

	// Get and validate rocket states
	rockets := mustGetRocketStates(t)
	validateRockets(t, rockets, 1)
}

// TestHighConcurrencyRockets tests the system under high concurrency load
func TestHighConcurrencyRockets(t *testing.T) {
	// Use a longer timeout for this high-load test
	timeout := 20 * time.Second

	// Run test with high concurrency (20 rockets)
	runRocketTest(t, 20, 5, "20ms", timeout)

	// Get and validate rocket states
	rockets := mustGetRocketStates(t)
	validateRockets(t, rockets, 10) // Expect at least 10 rockets under high load
}

// TestSlowMessageProcessing tests with very slow message delivery
func TestSlowMessageProcessing(t *testing.T) {
	// Use a longer timeout for slow message processing
	timeout := 15 * time.Second

	// Run test with slow message delivery (1 second between messages)
	runRocketTest(t, 2, 3, "1s", timeout)

	// Get and validate rocket states
	rockets := mustGetRocketStates(t)
	validateRockets(t, rockets, 2)
}

// TestRocketStateConsistency tests that rocket states are consistent across multiple queries
func TestRocketStateConsistency(t *testing.T) {
	// Generate some initial rocket state
	runRocketTest(t, 3, 2, "50ms", 10*time.Second)

	// Get initial rocket states
	initialRockets := mustGetRocketStates(t)
	require.NotEmpty(t, initialRockets, "No rockets found in initial state")

	// Query multiple times and verify consistency
	for i := 0; i < 5; i++ {
		// Brief pause between queries
		time.Sleep(100 * time.Millisecond)

		// Get current rocket states
		currentRockets := mustGetRocketStates(t)

		// Verify count is same as initial
		assert.Equal(t, len(initialRockets), len(currentRockets),
			"Rocket count changed between queries (initial: %d, current: %d)",
			len(initialRockets), len(currentRockets))

		// Verify rockets have same IDs across queries
		initialIDs := make(map[string]bool)
		for _, rocket := range initialRockets {
			initialIDs[rocket.ID] = true
		}

		for _, rocket := range currentRockets {
			assert.True(t, initialIDs[rocket.ID], "Rocket ID %s appeared that wasn't in initial state", rocket.ID)
		}
	}
}

// TestConcurrentStateQueries tests concurrent queries to the rocket state endpoint
func TestConcurrentStateQueries(t *testing.T) {
	// Generate some initial rocket state
	runRocketTest(t, 5, 3, "50ms", 10*time.Second)

	// Now test concurrent queries
	var wg sync.WaitGroup
	errorChan := make(chan error, 10) // Buffer for potential errors

	// Launch 10 concurrent queries
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(queryNum int) {
			defer wg.Done()

			// Get rocket states
			rockets, err := getRocketStates()
			if err != nil {
				errorChan <- err
				return
			}

			// Verify we got results
			if len(rockets) == 0 {
				errorChan <- fmt.Errorf("query %d found no rockets", queryNum)
			}
		}(i)
	}

	// Wait for all queries to complete
	wg.Wait()

	// Check for any errors
	close(errorChan)
	for err := range errorChan {
		t.Errorf("Concurrent query error: %v", err)
	}
}
