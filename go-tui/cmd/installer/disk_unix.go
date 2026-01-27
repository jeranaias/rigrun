// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package main

import (
	"syscall"
)

// getFreeDiskSpace returns the free disk space in bytes for the given path on Unix systems
func getFreeDiskSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t

	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}

	// Available blocks * block size = available bytes
	// Use Bavail (available to non-root users) rather than Bfree (total free)
	return stat.Bavail * uint64(stat.Bsize), nil
}
