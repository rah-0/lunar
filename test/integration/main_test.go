package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rah-0/lunar/internal/models"
)

// TestMain handles setup and teardown for all tests using testutil
func TestMain(m *testing.M) {
	// Create a context that can be canceled when all tests are done
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to receive test results
	testResult := make(chan int, 1)

	// Start the server as a subprocess
	serverProc, err := startServer(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}

	// Function to clean up processes
	cleanup := func() {
		// Clean up any other processes first
		cleanupAllProcesses()
		
		// Then handle the server process
		if serverProc != nil && serverProc.cmd != nil && serverProc.cmd.Process != nil {
			// Send SIGTERM to the server process group for graceful shutdown
			syscall.Kill(-serverProc.cmd.Process.Pid, syscall.SIGTERM)
			
			// Create a channel to wait for process exit
			done := make(chan struct{})
			
			// Set up a timer for force kill
			timer := time.AfterFunc(500*time.Millisecond, func() {
				syscall.Kill(-serverProc.cmd.Process.Pid, syscall.SIGKILL)
			})
			defer timer.Stop()
			
			// Wait for process to exit in a goroutine
			go func() {
				_, _ = serverProc.cmd.Process.Wait()
				close(done)
			}()
			
			// Wait for either process exit or timeout (handled by timer)
			select {
			case <-done:
				// Process exited normally
			case <-time.After(1 * time.Second):
				// Timer will handle the force kill
			}
		}
	}

	// Run tests in a goroutine
	go func() {
		defer func() {
			// Recover from any panics in tests
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Test panicked: %v\n", r)
				testResult <- 1
			}
		}()
		testResult <- m.Run()
	}()

	// Wait for test completion or timeout with increased timeout
	const testTimeout = 60 * time.Second
	select {
	case result := <-testResult:
		// Tests completed
		cleanup()
		os.Exit(result)
	case <-time.After(testTimeout):
		// Tests timed out
		fmt.Fprintf(os.Stderr, "Tests timed out after %v\n", testTimeout)
		cleanup()
		os.Exit(1)
	}
}

// Variables to track running processes
var (
	processLock    sync.Mutex
	runningServers []*serverProcess
	runningRockets []*rocketsProcess
)

// cleanupAllProcesses kills all tracked processes and any orphaned processes
func cleanupAllProcesses() {
	processLock.Lock()
	defer processLock.Unlock()

	// First, try to gracefully shut down all servers
	for _, server := range runningServers {
		if server.cmd != nil && server.cmd.Process != nil {
			// Send SIGTERM for graceful shutdown
			syscall.Kill(-server.cmd.Process.Pid, syscall.SIGTERM)
			
			// Set up a timer for force kill
			timer := time.AfterFunc(500*time.Millisecond, func() {
				syscall.Kill(-server.cmd.Process.Pid, syscall.SIGKILL)
			})
			
			// Wait for the process to exit
			_, _ = server.cmd.Process.Wait()
			timer.Stop()
		}
	}
	runningServers = nil

	// Kill any remaining rocket processes
	for _, rocket := range runningRockets {
		if rocket.cmd != nil && rocket.cmd.Process != nil {
			// Kill the entire process group
			syscall.Kill(-rocket.cmd.Process.Pid, syscall.SIGKILL)
			_, _ = rocket.cmd.Process.Wait()
		}
	}
	runningRockets = nil

	// Final cleanup: kill any remaining processes that might be using the port
	// This is a last resort and should only be reached if the above steps fail
	killProcessesOnPort(8088)
}

// killProcessesOnPort kills all processes using the specified port
func killProcessesOnPort(port int) {
	// Get the current process ID to avoid killing ourselves
	currentPid := os.Getpid()

	// Try to find processes using the port and kill them
	if _, err := exec.LookPath("lsof"); err == nil {
		// Use lsof to find processes using the port
		cmd := exec.Command("lsof", "-t", fmt.Sprintf("-i:%d", port))
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			pids := strings.Fields(string(output))
			for _, pidStr := range pids {
				pid, err := strconv.Atoi(pidStr)
				if err == nil && pid != currentPid {
					// Try to kill the process
					syscall.Kill(pid, syscall.SIGTERM)
					// Give it a moment to shut down
					time.Sleep(100 * time.Millisecond)
					// Force kill if still running
					syscall.Kill(pid, syscall.SIGKILL)
				}
			}
		}
	}

	// Additional check using fuser if available
	if _, err := exec.LookPath("fuser"); err == nil {
		cmd := exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", port))
		_ = cmd.Run()
	}
}

// syncBuffer is a thread-safe buffer implementation
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write implements io.Writer
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String returns the buffer contents as a string
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// serverProcess represents the Lunar service process
type serverProcess struct {
	cmd    *exec.Cmd
	stdout *syncBuffer
	stderr *syncBuffer
}

// rocketsProcess represents the rockets binary process
type rocketsProcess struct {
	cmd    *exec.Cmd
	stdout *syncBuffer
	stderr *syncBuffer
}

// startServer starts the Lunar server as a subprocess
func startServer(ctx context.Context) (*serverProcess, error) {
	process := &serverProcess{}

	// Build the server binary first to avoid race conditions with go run
	buildCmd := exec.Command("go", "build", "-o", "/tmp/lunar-server", "../../cmd/server/main.go")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to build server: %w", err)
	}

	// Set up the command to run the server binary
	// Use CommandContext to tie the process to the context
	cmd := exec.CommandContext(ctx, "/tmp/lunar-server", "--port", "8088")
	cmd.Env = os.Environ()
	
	// Set process group ID and create a new session
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0, // Create a new process group
	}

	// Create pipes for stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the server
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	// Create thread-safe buffers for stdout/stderr
	stdoutBuf := &syncBuffer{}
	stderrBuf := &syncBuffer{}

	// Read stdout and stderr in the background
	go func() {
		if _, err := io.Copy(stdoutBuf, stdoutPipe); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdout: %v\n", err)
		}
	}()

	go func() {
		if _, err := io.Copy(stderrBuf, stderrPipe); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stderr: %v\n", err)
		}
	}()

	// Store the process information
	process.cmd = cmd
	process.stdout = stdoutBuf
	process.stderr = stderrBuf

	// Track this process for cleanup
	processLock.Lock()
	runningServers = append(runningServers, process)
	processLock.Unlock()

	// Remove the binary when the process exits
	go func() {
		if err := cmd.Wait(); err != nil {
			// Log process exit error to stderr buffer for debugging
			fmt.Fprintf(stderrBuf, "Process exited with error: %v\n", err)
		}
		// Clean up the binary after process exits
		os.Remove("/tmp/lunar-server")
	}()

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

	// Create thread-safe buffers for stdout/stderr
	process.cmd = cmd
	process.stdout = &syncBuffer{}
	process.stderr = &syncBuffer{}
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
