// json_output.go - JSON output support for IL5 SIEM integration.
//
// Provides a standardized JSON output format for all CLI commands
// to support AU-6 (automated audit review) and SI-4 (system monitoring).
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// JSONResponse is the standardized response format for all CLI commands.
// This structure supports IL5 SIEM integration requirements:
// - AU-6: Automated audit review (machine-parseable output)
// - SI-4: Information system monitoring integration
type JSONResponse struct {
	// Success indicates whether the command completed successfully
	Success bool `json:"success"`

	// Data contains the command-specific response data
	Data interface{} `json:"data"`

	// Error contains the error message if Success is false, null otherwise
	Error *string `json:"error"`

	// Timestamp is the ISO8601 timestamp when the response was generated
	Timestamp string `json:"timestamp"`

	// Command is the command that was executed (for audit trail)
	Command string `json:"command,omitempty"`
}

// NewJSONResponse creates a new successful JSON response.
func NewJSONResponse(command string, data interface{}) *JSONResponse {
	return &JSONResponse{
		Success:   true,
		Data:      data,
		Error:     nil,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   command,
	}
}

// NewJSONErrorResponse creates a new error JSON response.
func NewJSONErrorResponse(command string, err error) *JSONResponse {
	errStr := err.Error()
	return &JSONResponse{
		Success:   false,
		Data:      nil,
		Error:     &errStr,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   command,
	}
}

// NewJSONErrorResponseStr creates a new error JSON response from a string.
func NewJSONErrorResponseStr(command string, errMsg string) *JSONResponse {
	return &JSONResponse{
		Success:   false,
		Data:      nil,
		Error:     &errMsg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Command:   command,
	}
}

// Print outputs the JSON response to stdout.
// Human-readable messages should go to stderr when JSON mode is enabled.
func (r *JSONResponse) Print() error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// PrintCompact outputs the JSON response without indentation.
// Useful for piping to other tools or log aggregation.
func (r *JSONResponse) PrintCompact() error {
	return json.NewEncoder(os.Stdout).Encode(r)
}

// String returns the JSON response as a string.
func (r *JSONResponse) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"success":false,"error":"failed to marshal response: %s","timestamp":"%s"}`,
			err.Error(), time.Now().UTC().Format(time.RFC3339))
	}
	return string(data)
}

// OutputJSON is a helper function that outputs either JSON or runs a normal handler.
// If jsonMode is true, it outputs JSON and handles errors. Otherwise it runs the handler.
func OutputJSON(jsonMode bool, command string, handler func() (interface{}, error)) error {
	if !jsonMode {
		_, err := handler()
		return err
	}

	data, err := handler()
	if err != nil {
		resp := NewJSONErrorResponse(command, err)
		resp.Print()
		return err
	}

	resp := NewJSONResponse(command, data)
	return resp.Print()
}

// StderrPrint prints a message to stderr (for human-readable output in JSON mode).
func StderrPrint(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// StderrPrintln prints a line to stderr (for human-readable output in JSON mode).
func StderrPrintln(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}

// =============================================================================
// COMMAND-SPECIFIC DATA STRUCTURES
// =============================================================================

// StatusData represents the data returned by the status command.
type StatusData struct {
	System  StatusSystemInfo  `json:"system"`
	Routing StatusRoutingInfo `json:"routing"`
	Session StatusSessionInfo `json:"session"`
	Cache   StatusCacheInfo   `json:"cache"`
}

// StatusSystemInfo contains system information for status command.
type StatusSystemInfo struct {
	GPU         string `json:"gpu"`
	GPUType     string `json:"gpu_type,omitempty"`
	VRAMGB      int    `json:"vram_gb,omitempty"`
	Ollama      string `json:"ollama"`
	OllamaVer   string `json:"ollama_version,omitempty"`
	Model       string `json:"model"`
	ModelStatus string `json:"model_status"`
}

// StatusRoutingInfo contains routing configuration for status command.
type StatusRoutingInfo struct {
	DefaultMode  string `json:"default_mode"`
	MaxTier      string `json:"max_tier"`
	ParanoidMode bool   `json:"paranoid_mode"`
	CloudKeySet  bool   `json:"cloud_key_configured"`
}

// StatusSessionInfo contains session statistics for status command.
type StatusSessionInfo struct {
	Queries   int     `json:"queries"`
	Local     int     `json:"local"`
	Cloud     int     `json:"cloud"`
	CacheHits int     `json:"cache_hits"`
	CostCents float64 `json:"cost_cents"`
	SavedCents float64 `json:"saved_cents"`
}

// StatusCacheInfo contains cache statistics for status command.
type StatusCacheInfo struct {
	Entries     int     `json:"entries"`
	HitRate     float64 `json:"hit_rate"`
	ExactHits   int     `json:"exact_hits"`
	SemanticHits int    `json:"semantic_hits"`
}

// DoctorData represents the data returned by the doctor command.
type DoctorData struct {
	Checks  []DoctorCheck `json:"checks"`
	Summary DoctorSummary `json:"summary"`
}

// DoctorCheck represents a single health check result.
type DoctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "warn", "fail"
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// DoctorSummary contains the summary of health checks.
type DoctorSummary struct {
	Passed  int  `json:"passed"`
	Warned  int  `json:"warned"`
	Failed  int  `json:"failed"`
	Healthy bool `json:"healthy"`
}

// ConfigData represents the data returned by the config show command.
type ConfigData struct {
	General  ConfigGeneralInfo  `json:"general"`
	Local    ConfigLocalInfo    `json:"local"`
	Cloud    ConfigCloudInfo    `json:"cloud"`
	Routing  ConfigRoutingInfo  `json:"routing"`
	Security ConfigSecurityInfo `json:"security"`
	Cache    ConfigCacheInfo    `json:"cache"`
	Path     string             `json:"config_path"`
}

// ConfigGeneralInfo contains general configuration.
type ConfigGeneralInfo struct {
	DefaultModel string `json:"default_model"`
	DefaultMode  string `json:"default_mode"`
}

// ConfigLocalInfo contains local inference configuration.
type ConfigLocalInfo struct {
	OllamaURL   string `json:"ollama_url"`
	OllamaModel string `json:"ollama_model"`
}

// ConfigCloudInfo contains cloud configuration (API key masked).
type ConfigCloudInfo struct {
	OpenRouterKeySet bool   `json:"openrouter_key_configured"`
	DefaultModel     string `json:"default_model"`
}

// ConfigRoutingInfo contains routing configuration.
type ConfigRoutingInfo struct {
	DefaultMode  string `json:"default_mode"`
	MaxTier      string `json:"max_tier"`
	ParanoidMode bool   `json:"paranoid_mode"`
}

// ConfigSecurityInfo contains security configuration.
type ConfigSecurityInfo struct {
	SessionTimeoutSecs int  `json:"session_timeout_secs"`
	AuditEnabled       bool `json:"audit_enabled"`
}

// ConfigCacheInfo contains cache configuration.
type ConfigCacheInfo struct {
	Enabled  bool `json:"enabled"`
	TTLHours int  `json:"ttl_hours"`
}

// CacheStatsData represents the data returned by the cache stats command.
type CacheStatsData struct {
	ExactCache    CacheTypeStats `json:"exact_cache"`
	SemanticCache CacheTypeStats `json:"semantic_cache"`
	Savings       CacheSavings   `json:"savings"`
	Location      string         `json:"location"`
}

// CacheTypeStats contains statistics for a specific cache type.
type CacheTypeStats struct {
	Entries   int     `json:"entries"`
	SizeBytes int64   `json:"size_bytes,omitempty"`
	HitRate   float64 `json:"hit_rate,omitempty"`
	Hits      int     `json:"hits,omitempty"`
	Misses    int     `json:"misses,omitempty"`
}

// CacheSavings contains cache savings information.
type CacheSavings struct {
	CacheHits    int     `json:"cache_hits"`
	SavedDollars float64 `json:"saved_dollars"`
}

// VersionData represents the data returned by the version command.
type VersionData struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version,omitempty"`
}

// AskData represents the data returned by the ask command.
type AskData struct {
	Response     string    `json:"response"`
	Tier         string    `json:"tier"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	CostCents    float64   `json:"cost_cents"`
	DurationMs   int64     `json:"duration_ms"`
	Complexity   string    `json:"complexity,omitempty"`
}
