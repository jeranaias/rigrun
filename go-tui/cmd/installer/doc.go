// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

/*
Package installer provides the rigrun interactive installer - a beautiful,
guided setup experience for new users.

# Overview

The installer is a terminal-based TUI application built with Bubble Tea that
guides users through the complete rigrun setup process. It provides both an
interactive TUI mode and a text-based mode for copy/paste friendly installation.

# Features

  - System requirements checking (OS, Go, Ollama, network, disk space)
  - Ollama service detection and setup guidance
  - AI model selection and download (qwen2.5-coder, codestral, llama3.1)
  - Configuration file generation (~/.rigrun/config.toml)
  - Binary download from GitHub releases
  - Beautiful animations and progress indicators

# Building

Build the installer binary:

	go build -o rigrun-installer.exe ./cmd/installer

Or build with version information:

	go build -ldflags "-X main.version=1.0.0" -o rigrun-installer.exe ./cmd/installer

# Command Line Options

The installer supports the following command line options:

	--text, -t     Run in text mode (copy/paste friendly, no TUI)
	--help, -h     Show help information
	--version, -v  Show version number

# Usage Examples

Run the interactive TUI installer (default):

	rigrun-installer

Run in text mode for non-interactive environments:

	rigrun-installer --text

# Files Created

The installer creates the following directory structure:

	~/.rigrun/
	    config.toml      # Main configuration file
	    sessions/        # Saved conversation sessions
	    logs/            # Application logs
	    costs/           # Cost tracking data
	    benchmarks/      # Performance benchmarks

	~/.local/bin/
	    rigrun           # Main rigrun binary (or rigrun.exe on Windows)

# Architecture

The installer consists of three main components:

  - main.go: Entry point, CLI argument parsing, text mode implementation
  - installer.go: TUI installer model with phases (welcome, checks, setup, complete)
  - welcome.go: First-run welcome screen with interactive tutorial tips

The TUI uses a phase-based state machine:

  - PhaseWelcome: Introduction and feature overview
  - PhaseSystemCheck: Verifies system requirements
  - PhaseOllamaSetup: Guides Ollama installation if needed
  - PhaseModelDownload: AI model selection and download
  - PhaseConfigSetup: Creates configuration files
  - PhaseComplete: Success screen with launch option

# Dependencies

  - github.com/charmbracelet/bubbletea - TUI framework
  - github.com/charmbracelet/bubbles - TUI components (spinner, progress)
  - github.com/charmbracelet/lipgloss - Terminal styling
*/
package main
