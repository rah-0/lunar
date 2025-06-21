package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// TestRocketsBinaryExists simply verifies that the rockets binary exists
func TestRocketsBinaryExists(t *testing.T) {
	if _, err := os.Stat("./rockets"); os.IsNotExist(err) {
		t.Errorf("rockets binary does not exist in the bin directory")
	}
}

// TestRocketsAcceptsLaunch verifies that rockets binary accepts an launch command
func TestRocketsAcceptsLaunch(t *testing.T) {
	cmd := exec.Command("./rockets", "--help")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		// The command might exit with error code, but we're just looking for output
		t.Logf("Note: rockets --help exited with status: %v", err)
	}

	// Check for launch command
	if strings.Contains(outputStr, "launch") {
		t.Logf("Rockets binary accepts launch command")
	} else {
		t.Fatalf("Command line help: %s", outputStr)
	}
}

// TestRocketsHelpOutput verifies that the rockets binary provides help output
func TestRocketsHelpOutput(t *testing.T) {
	// Try different help flags that might be supported
	helpFlags := []string{"--help", "-help", "-h", "help"}
	var lastErr error
	var lastOutput string

	for _, helpFlag := range helpFlags {
		cmd := exec.Command("./rockets", helpFlag)
		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		// If we got a reasonable amount of output, consider it successful
		if len(outputStr) > 20 {
			// Look for common CLI option patterns
			for _, term := range []string{"endpoint", "url", "host", "target", "usage"} {
				if strings.Contains(strings.ToLower(outputStr), term) {
					// Found something that looks like help text
					return
				}
			}
		}

		// Save the last error and output in case we need to report it
		lastErr = err
		lastOutput = outputStr
	}

	// If we get here, we tried all help flags and didn't find useful output
	t.Errorf("Could not get helpful output from rockets binary using standard help flags. Last error: %v, Last output: %s", lastErr, lastOutput)
}

// TestRocketsVersion verifies the version of the rockets binary
func TestRocketsVersion(t *testing.T) {
	// Run the version command
	cmd := exec.Command("./rockets", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get rockets version: %v", err)
	}

	// Output the version information
	outputStr := string(output)
	t.Logf("Rockets version: %s", outputStr)

	// Check for expected version format
	// We're looking for something that resembles a version number like 1.0.0 or v1.2.3
	versionPattern := regexp.MustCompile(`v?\d+\.\d+\.\d+`)
	if !versionPattern.MatchString(outputStr) {
		t.Errorf("Version output doesn't contain a recognizable version number: %s", outputStr)
	}

	// If there's a specific required version, we can check for that
	requiredVersion := "0.1.0"
	if !strings.Contains(outputStr, requiredVersion) {
		t.Fatalf("Version doesn't match expected version %s.", requiredVersion)
	}
}
