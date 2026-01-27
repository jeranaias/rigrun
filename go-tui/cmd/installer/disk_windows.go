// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// getFreeDiskSpace returns the free disk space in bytes for the given path on Windows
func getFreeDiskSpace(path string) (uint64, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	ret, _, err := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if ret == 0 {
		return 0, err
	}

	return freeBytesAvailable, nil
}
