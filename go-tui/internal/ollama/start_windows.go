// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows
// +build windows

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

// Windows-specific creation flags
const (
	// CREATE_NO_WINDOW prevents a console window from being created
	CREATE_NO_WINDOW = 0x08000000
	// DETACHED_PROCESS creates a new process that is detached from the console
	DETACHED_PROCESS = 0x00000008
)

// findOllamaExecutable searches for ollama.exe in common installation paths on Windows.
// Returns the full path to ollama.exe if found, or "ollama" to fall back to PATH lookup.
func findOllamaExecutable() (string, error) {
	// First, check if ollama is in PATH
	if path, err := exec.LookPath("ollama.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("ollama"); err == nil {
		return path, nil
	}

	// Common Ollama installation paths on Windows
	possiblePaths := []string{}

	// User install location: %LOCALAPPDATA%\Programs\Ollama\ollama.exe
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		possiblePaths = append(possiblePaths, filepath.Join(localAppData, "Programs", "Ollama", "ollama.exe"))
	}

	// System install locations
	possiblePaths = append(possiblePaths,
		`C:\Program Files\Ollama\ollama.exe`,
		`C:\Program Files (x86)\Ollama\ollama.exe`,
	)

	// User profile locations (alternative installs)
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		possiblePaths = append(possiblePaths,
			filepath.Join(userProfile, "Ollama", "ollama.exe"),
			filepath.Join(userProfile, ".ollama", "ollama.exe"),
		)
	}

	// Check each possible path
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Not found - return error with helpful message
	return "", fmt.Errorf("ollama.exe not found in PATH or common installation directories. "+
		"Please ensure Ollama is installed. Checked: PATH, %%LOCALAPPDATA%%\\Programs\\Ollama, "+
		"C:\\Program Files\\Ollama")
}

// startOllamaProcess starts the Ollama server on Windows.
// Uses Windows-specific process creation flags for proper background execution.
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

	// Set Windows-specific process attributes for background execution:
	// - CREATE_NEW_PROCESS_GROUP: Creates a new process group (allows independent termination)
	// - CREATE_NO_WINDOW: Prevents a console window from appearing
	// - DETACHED_PROCESS: Detaches from the parent console
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW | DETACHED_PROCESS,
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
	// This is important for background processes on Windows
	if cmd.Process != nil {
		if err := cmd.Process.Release(); err != nil {
			// Non-fatal: process started but release failed
			// Log but continue - the process should still be running
		}
	}

	// Wait for Ollama to become ready (poll for up to 15 seconds)
	// Ollama can take a while to start on Windows, especially first launch
	deadline := time.Now().Add(15 * time.Second)
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
		checkCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
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
		Message: fmt.Sprintf("Ollama started but not responding after 15 seconds (path: %s)", ollamaPath),
		Cause:   lastErr,
	}
}
