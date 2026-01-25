// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package detect provides GPU detection utilities for rigrun.
package detect

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// GPU TYPE TESTS
// =============================================================================

func TestGpuType_String(t *testing.T) {
	tests := []struct {
		gpuType GpuType
		want    string
	}{
		{GpuTypeCPU, "CPU"},
		{GpuTypeNvidia, "NVIDIA"},
		{GpuTypeAmd, "AMD"},
		{GpuTypeAppleSilicon, "Apple Silicon"},
		{GpuTypeIntel, "Intel Arc"},
		{GpuType(99), "Unknown"},
	}

	for _, tc := range tests {
		got := tc.gpuType.String()
		if got != tc.want {
			t.Errorf("GpuType(%d).String() = %q, want %q", tc.gpuType, got, tc.want)
		}
	}
}

// =============================================================================
// GPU INFO TESTS
// =============================================================================

func TestGpuInfo_String(t *testing.T) {
	tests := []struct {
		info *GpuInfo
		want string
	}{
		{
			&GpuInfo{Name: "NVIDIA RTX 4090", VramGB: 24, Type: GpuTypeNvidia},
			"NVIDIA RTX 4090 (24GB VRAM)",
		},
		{
			&GpuInfo{Name: "NVIDIA RTX 4090", VramGB: 24, Driver: "535.154.05", Type: GpuTypeNvidia},
			"NVIDIA RTX 4090 (24GB VRAM) [Driver: 535.154.05]",
		},
		{
			&GpuInfo{Name: "Apple M2 Ultra", VramGB: 192, Type: GpuTypeAppleSilicon},
			"Apple M2 Ultra (192GB VRAM)",
		},
	}

	for _, tc := range tests {
		got := tc.info.String()
		if got != tc.want {
			t.Errorf("GpuInfo.String() = %q, want %q", got, tc.want)
		}
	}
}

// =============================================================================
// GPU DETECTION TESTS
// =============================================================================

func TestDetectGPU(t *testing.T) {
	// DetectGPU should always return a valid result (even if just CPU)
	info, err := DetectGPU()
	if err != nil {
		t.Fatalf("DetectGPU failed: %v", err)
	}

	if info == nil {
		t.Fatal("DetectGPU returned nil info")
	}

	// Should have a name
	if info.Name == "" {
		t.Error("GpuInfo.Name should not be empty")
	}

	// Type should be valid
	if info.Type < GpuTypeCPU || info.Type > GpuTypeIntel {
		t.Errorf("GpuInfo.Type = %d is out of valid range", info.Type)
	}

	t.Logf("Detected GPU: %s (Type: %s)", info.String(), info.Type.String())
}

func TestDetectGPUWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := DetectGPUWithContext(ctx)
	if err != nil {
		t.Fatalf("DetectGPUWithContext failed: %v", err)
	}

	if info == nil {
		t.Fatal("DetectGPUWithContext returned nil info")
	}
}

func TestDetectGPUWithContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should not hang forever
	done := make(chan bool)
	go func() {
		DetectGPUWithContext(ctx)
		done <- true
	}()

	select {
	case <-done:
		// Good - returned without hanging
	case <-time.After(2 * time.Second):
		t.Error("DetectGPUWithContext should respect context cancellation")
	}
}

// =============================================================================
// CACHE TESTS
// =============================================================================

func TestDetectGPUCached(t *testing.T) {
	// Clear cache first
	ClearGPUCache()

	// First call should detect
	info1, err := DetectGPUCached()
	if err != nil {
		t.Fatalf("First DetectGPUCached failed: %v", err)
	}

	// Second call should return cached result (much faster)
	start := time.Now()
	info2, err := DetectGPUCached()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Second DetectGPUCached failed: %v", err)
	}

	// Should be the same info
	if info1.Name != info2.Name || info1.Type != info2.Type {
		t.Error("Cached result should match original")
	}

	// Cached call should be very fast (< 1ms typical)
	if elapsed > 100*time.Millisecond {
		t.Logf("Cached call took %v (expected < 100ms)", elapsed)
	}
}

func TestClearGPUCache(t *testing.T) {
	// Populate cache
	DetectGPUCached()

	// Clear it
	ClearGPUCache()

	// Verify it's cleared by checking internal state
	gpuCacheMu.Lock()
	isClear := gpuCache == nil
	gpuCacheMu.Unlock()

	if !isClear {
		t.Error("ClearGPUCache should clear the cache")
	}
}

// =============================================================================
// CPU INFO TESTS
// =============================================================================

func TestGetCPUInfo(t *testing.T) {
	info := GetCPUInfo()

	if info == nil {
		t.Fatal("GetCPUInfo returned nil")
	}

	if info.Type != GpuTypeCPU {
		t.Errorf("GetCPUInfo should return GpuTypeCPU, got %v", info.Type)
	}

	if info.Name == "" {
		t.Error("CPU name should not be empty")
	}

	t.Logf("CPU Info: %s", info.String())
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestDetectGPUCached_Concurrent(t *testing.T) {
	ClearGPUCache()

	// Multiple goroutines calling cached detection
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_, err := DetectGPUCached()
				if err != nil {
					t.Errorf("Concurrent DetectGPUCached failed: %v", err)
				}
			}
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}
}

// =============================================================================
// GPU STATUS REPORT TESTS
// =============================================================================

func TestGetGPUStatusReport(t *testing.T) {
	report := GetGPUStatusReport()

	// Should always have GPU info
	if report.GPU == nil {
		t.Error("GPUStatusReport.GPU should not be nil")
	}

	// MemoryTotal should be non-negative
	if report.MemoryTotal < 0 {
		t.Error("GPUStatusReport.MemoryTotal should be non-negative")
	}

	// MemoryPercent should be in valid range
	if report.MemoryPercent < 0 || report.MemoryPercent > 100 {
		t.Errorf("GPUStatusReport.MemoryPercent = %f, want 0-100", report.MemoryPercent)
	}

	t.Logf("GPU Status: %s, Available: %v", report.GPU.Name, report.IsAvailable)
}
