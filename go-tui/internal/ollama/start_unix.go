// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows
// +build !windows

// Package ollama provides the HTTP client for communicating with Ollama API.
package ollama

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// findOllamaExecutable searches for ollama in common installation paths on Unix.
// Returns the full path to ollama if found, or "ollama" to fall back to PATH lookup.
func findOllamaExecutable() (string, error) {
	// First, check if ollama is in PATH
	if path, err := exec.LookPath("ollama"); err == nil {
		return path, nil
	}

	// Common Ollama installation paths on Unix/macOS
	possiblePaths := []string{
		"/usr/local/bin/ollama",
		"/usr/bin/ollama",
		"/opt/ollama/ollama",
	}

	// User home directory locations
	if home := os.Getenv("HOME"); home != "" {
		possiblePaths = append(possiblePaths,
			filepath.Join(home, ".local", "bin", "ollama"),
			filepath.Join(home, "bin", "ollama"),
		)
	}

	// macOS application bundle location
	possiblePaths = append(possiblePaths,
		"/Applications/Ollama.app/Contents/Resources/ollama",
	)

	// Check each possible path
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Not found - return error with helpful message
	return "", fmt.Errorf("ollama not found in PATH or common installation directories. "+
		"Please ensure Ollama is installed. Checked: PATH, /usr/local/bin, /usr/bin, ~/.local/bin")
}

// startOllamaProcess starts the Ollama server on Unix/macOS.
// Uses Unix-specific process attributes for proper background execution.
func (c *Client) startOllamaProcess(ctx context.Context) error {
	// Find the Ollama executable
	ollamaPath, err := findOllamaExecutable()
	if err != nil {
		return &ClientError{
			Type:    ErrTypeConnection,
			Message: "failed to find Ollama executable",
			Cause:   err,
		}
	}

	// Create the command with "serve" argument
	cmd := exec.Command(ollamaPath, "serve")

	// CRITICAL: Pass environment variables to the child process
	// This ensures OLLAMA_VULKAN=1 and other GPU-related vars reach Ollama
	cmd.Env = os.Environ()

	// Set Unix-specific process attributes for background execution:
	// - Setpgid: Creates a new process group (allows independent termination)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Don't capture output - let it run independently
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the process
	if err := cmd.Start(); err != nil {
		return &ClientError{
			Type:    ErrTypeConnection,
			Message: fmt.Sprintf("failed to start Ollama (path: %s)", ollamaPath),
			Cause:   err,
		}
	}

	// Release the process so it continues running after we exit
	if cmd.Process != nil {
		if err := cmd.Process.Release(); err != nil {
			// Non-fatal: process started but release failed
		}
	}

	// Wait for Ollama to become ready (poll for up to 10 seconds)
	deadline := time.Now().Add(10 * time.Second)
	startTime := time.Now()
	var lastErr error

	// Print initial status
	fmt.Fprintf(os.Stderr, "Starting Ollama service...\n")

	for time.Now().Before(deadline) {
		// Check if parent context was cancelled
		select {
		case <-ctx.Done():
			return &ClientError{
				Type:    ErrTypeConnection,
				Message: "Ollama startup cancelled",
				Cause:   ctx.Err(),
			}
		default:
		}

		// Try to connect to Ollama
		checkCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		lastErr = c.CheckRunning(checkCtx)
		cancel()

		if lastErr == nil {
			elapsed := time.Since(startTime)
			fmt.Fprintf(os.Stderr, "Ollama service started successfully (%.1fs)\n", elapsed.Seconds())
			return nil // Started successfully
		}

		// Show progress with elapsed time
		elapsed := time.Since(startTime)
		fmt.Fprintf(os.Stderr, "\rStarting Ollama service... %.1fs elapsed", elapsed.Seconds())

		// Wait before retrying
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Fprintf(os.Stderr, "\n")

	return &ClientError{
		Type:    ErrTypeConnection,
		Message: fmt.Sprintf("Ollama started but not responding after 10 seconds (path: %s)", ollamaPath),
		Cause:   lastErr,
	}
}
