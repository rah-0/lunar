package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
)

// TestMain handles setup and teardown for all tests using testutil
func TestMain(m *testing.M) {
	// Create a context that can be canceled when all tests are done
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server as a subprocess
	serverProcess, err := startServer(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Defer cleanup to ensure the server process is stopped
	defer cleanupAllProcesses()

	// Run all tests
	testResult := m.Run()

	// Stop the server regardless of test outcome
	if err := serverProcess.cmd.Process.Kill(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop server: %v\n", err)
	}

	// Exit with the test result
	os.Exit(testResult)
}

// Variables to track running processes
var (
	processLock    sync.Mutex
	runningServers []*serverProcess
	runningRockets []*rocketsProcess
)

// cleanupAllProcesses kills all tracked processes
func cleanupAllProcesses() {
	processLock.Lock()
	defer processLock.Unlock()

	for _, server := range runningServers {
		if server.cmd.Process != nil {
			_ = server.cmd.Process.Kill()
			_, _ = server.cmd.Process.Wait()
		}
	}
	runningServers = nil

	for _, rocket := range runningRockets {
		if rocket.cmd.Process != nil {
			_ = rocket.cmd.Process.Kill()
			_, _ = rocket.cmd.Process.Wait()
		}
	}
	runningRockets = nil
}

// serverProcess represents the Lunar service process
type serverProcess struct {
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// rocketsProcess represents the rockets binary process
type rocketsProcess struct {
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// startServer starts the Lunar server as a subprocess
func startServer(ctx context.Context) (*serverProcess, error) {
	process := &serverProcess{}

	// Set up the command to run the server binary
	cmd := exec.CommandContext(ctx, "go", "run", "../../cmd/server/main.go", "--port", "8088")
	cmd.Env = os.Environ()

	// Capture stdout and stderr
	process.cmd = cmd
	process.stdout = new(bytes.Buffer)
	process.stderr = new(bytes.Buffer)
	cmd.Stdout = process.stdout
	cmd.Stderr = process.stderr

	// Start the server
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	// Track this process for cleanup
	processLock.Lock()
	runningServers = append(runningServers, process)
	processLock.Unlock()

	// Wait for the server to be ready
	startTime := time.Now()
	for {
		resp, err := http.Get("http://localhost:8088/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}
		if err != nil && !strings.Contains(err.Error(), "connection refused") {
			return process, fmt.Errorf("unexpected error checking server health: %w", err)
		}

		if time.Since(startTime) > 5*time.Second {
			return process, fmt.Errorf("server failed to start within timeout")
		}

		time.Sleep(100 * time.Millisecond)
	}

	return process, nil
}

// startRockets starts the rockets binary with specified parameters
func startRockets(ctx context.Context, concurrency int, maxMessages int, messageDelay string) (*rocketsProcess, error) {
	process := &rocketsProcess{}

	// Set up the command to run the rockets binary
	cmd := exec.CommandContext(
		ctx,
		"../../bin/rockets",
		"launch",                         // Use the launch subcommand
		"http://localhost:8088/messages", // URL endpoint for messages
		"--concurrency-level", fmt.Sprintf("%d", concurrency),
		"--max-messages", fmt.Sprintf("%d", maxMessages),
		"--message-delay", messageDelay,
	)
	cmd.Env = os.Environ()

	// Capture stdout and stderr
	process.cmd = cmd
	process.stdout = new(bytes.Buffer)
	process.stderr = new(bytes.Buffer)
	cmd.Stdout = process.stdout
	cmd.Stderr = process.stderr

	// Start the rockets process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start rockets: %w", err)
	}

	// Track this process for cleanup
	processLock.Lock()
	runningRockets = append(runningRockets, process)
	processLock.Unlock()

	return process, nil
}

// getRocketStates fetches the current rocket states from the server
func getRocketStates() ([]models.RocketSummary, error) {
	// Make a GET request to the rockets endpoint
	resp, err := http.Get("http://localhost:8088/rockets")
	if err != nil {
		return nil, fmt.Errorf("failed to get rockets: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	// Parse the response
	var rockets []models.RocketSummary
	if err := json.NewDecoder(resp.Body).Decode(&rockets); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return rockets, nil
}

// waitForRockets waits for the rockets process to complete
func waitForRockets(rp *rocketsProcess, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- rp.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for rockets process")
	}
}
