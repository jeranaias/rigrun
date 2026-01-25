# Rigrun Automated Test Plan for IL5 Compliance

**Classification: UNCLASSIFIED**

**Document Version:** 1.0
**Date:** 2026-01-20
**Prepared For:** Department of War (DoW) IL5 Certification
**Prepared By:** Automated Test Generation System

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [CLI Command Inventory](#cli-command-inventory)
3. [Test Environment Setup](#test-environment-setup)
4. [Security Tests (IL5 Compliance)](#security-tests-il5-compliance)
5. [Functional Tests](#functional-tests)
6. [Reliability Tests](#reliability-tests)
7. [Audit Tests](#audit-tests)
8. [Gap Analysis - Missing CLI Commands](#gap-analysis---missing-cli-commands)
9. [Sonnet Execution Instructions](#sonnet-execution-instructions)

---

## Executive Summary

This document provides a comprehensive automated test plan for rigrun, a Go TUI/CLI application for LLM interaction intended for IL5 (Impact Level 5) deployment by the Department of War.

### Current CLI Status

| Command | Status | Description |
|---------|--------|-------------|
| `rigrun` | **WIRED** | Launches TUI interface |
| `rigrun ask` | **WIRED** | Single question CLI mode |
| `rigrun chat` | **WIRED** | Interactive chat CLI mode |
| `rigrun status` | **WIRED** | System status display |
| `rigrun config` | **WIRED** | Configuration management |
| `rigrun setup` | **WIRED** | First-run wizard |
| `rigrun cache` | **WIRED** | Cache management |
| `rigrun doctor` | **WIRED** | System diagnostics |
| `rigrun version` | **WIRED** | Version information |
| `rigrun help` | **WIRED** | Help display |

### IL5 Security Features Present

- Audit logging with secret redaction (`internal/security/audit.go`)
- DoD classification marking system (`internal/security/classification.go`)
- Session timeout per DoD STIG AC-12 (`internal/security/session.go`)
- Paranoid mode for local-only operation
- API key masking in output
- Tool sandboxing with permission levels

---

## CLI Command Inventory

### Fully Implemented Commands

#### 1. `rigrun --help` / `rigrun help`
```bash
rigrun --help
```
**Expected Output:** Usage text with command list and examples
**Pass Criteria:** Exit code 0, contains "Usage:", lists all subcommands

#### 2. `rigrun version`
```bash
rigrun version
```
**Expected Output:** Version string with git commit and build date
**Pass Criteria:** Exit code 0, contains "version", contains "Git commit"

#### 3. `rigrun status`
```bash
rigrun status
```
**Expected Output:** System status including GPU, Ollama, routing, session, cache info
**Pass Criteria:** Exit code 0, contains "System", "Routing", "Session", "Cache"

#### 4. `rigrun config show`
```bash
rigrun config show
```
**Expected Output:** Current configuration in formatted sections
**Pass Criteria:** Exit code 0, contains "[general]", "[local]", "[cloud]", "[routing]"

#### 5. `rigrun config set <key> <value>`
```bash
rigrun config set default_mode local
```
**Expected Output:** Confirmation message "[OK] default_mode = local"
**Pass Criteria:** Exit code 0, contains "[OK]"

#### 6. `rigrun config reset`
```bash
rigrun config reset
```
**Expected Output:** Confirmation that config was reset to defaults
**Pass Criteria:** Exit code 0, contains "reset to defaults"

#### 7. `rigrun config path`
```bash
rigrun config path
```
**Expected Output:** Path to configuration file
**Pass Criteria:** Exit code 0, contains ".rigrun" and "config.toml"

#### 8. `rigrun cache stats`
```bash
rigrun cache stats
```
**Expected Output:** Cache statistics (entries, hit rate, savings)
**Pass Criteria:** Exit code 0, contains "Entries:", "Hit Rate:"

#### 9. `rigrun doctor`
```bash
rigrun doctor
```
**Expected Output:** Health check results with pass/warn/fail indicators
**Pass Criteria:** Exit code 0 or 1, contains checkmark or warning symbols

#### 10. `rigrun ask "<question>"`
```bash
rigrun ask "What is 2+2?" --quiet
```
**Expected Output:** LLM response to the question
**Pass Criteria:** Exit code 0, non-empty output (requires Ollama running)

---

## Test Environment Setup

### Prerequisites

```bash
# 1. Verify rigrun executable exists
test -f "C:\rigrun\go-tui\rigrun.exe" && echo "PASS: rigrun executable found" || echo "FAIL: rigrun executable not found"

# 2. Check Ollama is installed
ollama --version

# 3. Check Ollama is running
curl -s http://127.0.0.1:11434/api/tags > /dev/null && echo "PASS: Ollama running" || echo "FAIL: Ollama not running"
```

### Test Data Setup

```bash
# Create test directory
mkdir -p ~/.rigrun/test

# Backup existing config
cp ~/.rigrun/config.toml ~/.rigrun/config.toml.backup 2>/dev/null || true
```

---

## Security Tests (IL5 Compliance)

### SEC-001: Paranoid Mode Blocks Cloud Requests

**Purpose:** Verify paranoid mode prevents all cloud API calls per AC-2 (Account Management)

```bash
# Test 1: Set paranoid mode via config
rigrun config set paranoid_mode true

# Test 2: Verify status shows paranoid mode active
rigrun status | grep -i "paranoid"
# Expected: "Paranoid:     Yes"

# Test 3: Verify with --paranoid flag
rigrun status --paranoid | grep -i "paranoid"
# Expected: "Paranoid:     Yes"
```

**Pass Criteria:**
- Output contains "Paranoid: Yes" or "paranoid mode"
- No network requests to OpenRouter when paranoid=true

---

### SEC-002: API Key Secure Storage

**Purpose:** Verify API keys are not exposed in plain text (per AC-3 Access Enforcement)

```bash
# Test 1: Set API key
rigrun config set openrouter_key sk-or-v1-test1234567890abcdefghijklmnopqrstuvwxyz1234567890abcdefghijkl

# Test 2: Verify key is masked in output
rigrun config show | grep "openrouter_key"
# Expected: Contains "****" not full key

# Test 3: Verify key is not in environment output
rigrun status | grep -v "sk-or-v1-"
# Expected: Full key should never appear
```

**Pass Criteria:**
- API key shown as partial with asterisks (e.g., "sk-or-v1****")
- Full API key never appears in any CLI output

---

### SEC-003: Audit Log Secret Redaction

**Purpose:** Verify sensitive data is redacted from audit logs (per AU-3 Content of Audit Records)

```bash
# Test 1: Enable audit logging
rigrun config set audit_enabled true

# Test 2: Query that includes a secret pattern
rigrun ask "My password is secret123 and my key is sk-or-v1-abcd1234" --quiet 2>/dev/null || true

# Test 3: Check audit log for redaction
cat ~/.rigrun/audit.log | tail -5
# Expected: Contains "[PASSWORD_REDACTED]" or "[OPENROUTER_KEY_REDACTED]"
```

**Pass Criteria:**
- Audit log exists at `~/.rigrun/audit.log`
- Secrets patterns are replaced with `[*_REDACTED]` markers
- Query content is truncated to 200 characters

---

### SEC-004: Session Timeout Compliance

**Purpose:** Verify session timeout per DoD STIG AC-12 (Session Termination)

```bash
# Test 1: Check default session timeout
rigrun config show | grep "session_timeout"
# Expected: session_timeout value present

# Test 2: Set session timeout within IL5 limits (max 30 minutes = 1800 seconds)
rigrun config set session_timeout 900

# Test 3: Verify setting applied
rigrun config show | grep "session_timeout"
# Expected: "session_timeout: 900 seconds"
```

**Pass Criteria:**
- Session timeout is configurable
- Default is within DoD STIG limits (15-30 minutes)
- Session timeout is enforced in TUI mode

---

### SEC-005: Classification Banner Verification

**Purpose:** Verify classification banners per DoDI 5200.48

```bash
# Test 1: Check banner configuration
rigrun config show | grep -i "classification"
# Expected: Shows classification setting (default: UNCLASSIFIED)

# Test 2: Set classification level
rigrun config set security.classification UNCLASSIFIED

# Test 3: Verify banner can be enabled
rigrun config set security.banner_enabled true
```

**Pass Criteria:**
- Classification marking is configurable
- Valid levels: UNCLASSIFIED, CUI, CONFIDENTIAL, SECRET, TOP SECRET
- Banner renders with appropriate colors

---

### SEC-006: Tool Sandboxing Verification

**Purpose:** Verify agentic tools are sandboxed with proper permissions

```bash
# Test 1: Check doctor shows tool system
rigrun doctor
# Expected: Health checks pass

# Test 2: Verify tool permission levels are enforced
# (Note: Full test requires TUI mode with tool calls)
```

**Pass Criteria:**
- Tools have defined risk levels (Low, Medium, High, Critical)
- High-risk tools require user approval
- Dangerous commands are blocked (rm -rf, sudo, etc.)

---

### SEC-007: Network Isolation Test

**Purpose:** Verify no unauthorized network connections in local mode

```bash
# Test 1: Set to local-only mode
rigrun config set default_mode local
rigrun config set paranoid_mode true

# Test 2: Run status (should only connect to localhost:11434)
rigrun status

# Test 3: Verify no external connections
# (Requires network monitoring - manual verification)
```

**Pass Criteria:**
- In paranoid mode, only localhost:11434 (Ollama) is contacted
- No external DNS lookups or connections

---

### SEC-008: PII/Classified Marker Detection

**Purpose:** Verify system can detect and handle sensitive content

```bash
# Test 1: Query with classification markers
rigrun ask "This is SECRET//NOFORN information" --paranoid --quiet 2>/dev/null || true

# Test 2: Check if routing respects classification
# (Note: Classification-based routing is architectural, verify via code review)
```

**Pass Criteria:**
- Classification markers are recognized
- Higher classification queries could trigger appropriate handling

---

## Functional Tests

### FUNC-001: API Key Configuration

**Purpose:** Verify API key can be configured via CLI

```bash
# Test 1: Set OpenRouter key
rigrun config set openrouter_key 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

# Test 2: Verify key is set (masked)
rigrun config show | grep "openrouter_key"
# Expected: Contains "sk-or-v1****"

# Test 3: Verify doctor shows configured
rigrun doctor | grep -i "openrouter"
# Expected: "OpenRouter configured" with checkmark
```

**Pass Criteria:**
- Config set succeeds
- Key is masked in output
- Doctor shows configured status

---

### FUNC-002: Model Configuration

**Purpose:** Verify model selection and switching

```bash
# Test 1: Set default model
rigrun config set default_model qwen2.5-coder:7b

# Test 2: Verify model setting
rigrun config show | grep "default_model"
# Expected: "qwen2.5-coder:7b"

# Test 3: Check status shows model
rigrun status | grep "Model:"
# Expected: Shows model name and status (available/not downloaded)

# Test 4: Use --model flag override
rigrun status --model qwen2.5:7b
```

**Pass Criteria:**
- Model can be set via config
- Model can be overridden via --model flag
- Status shows model availability

---

### FUNC-003: Routing Mode Configuration

**Purpose:** Verify routing modes (local/cloud/hybrid)

```bash
# Test 1: Set to local mode
rigrun config set default_mode local
rigrun status | grep "Default:"
# Expected: "Local mode" or similar

# Test 2: Set to hybrid mode
rigrun config set default_mode hybrid
rigrun status | grep "Default:"
# Expected: "Hybrid mode" or similar

# Test 3: Set to cloud mode
rigrun config set default_mode cloud
rigrun status | grep "Default:"
# Expected: "Cloud mode" or similar

# Test 4: Reset to local for safety
rigrun config set default_mode local
```

**Pass Criteria:**
- All three modes are accepted: local, cloud, hybrid
- Status reflects current mode
- Invalid modes are rejected with error

---

### FUNC-004: Max Tier Configuration

**Purpose:** Verify max tier limits cloud spending

```bash
# Test 1: Set max tier to haiku (cheapest)
rigrun config set max_tier haiku
rigrun config show | grep "max_tier"
# Expected: "haiku"

# Test 2: Set max tier to sonnet
rigrun config set max_tier sonnet

# Test 3: Set max tier to opus (most expensive)
rigrun config set max_tier opus

# Test 4: Try invalid tier
rigrun config set max_tier invalid 2>&1 | grep -i "invalid"
# Expected: Error message about invalid tier
```

**Pass Criteria:**
- Valid tiers: cache, local, cloud, haiku, sonnet, opus, gpt-4o
- Invalid tiers are rejected with helpful error

---

### FUNC-005: Cache Management

**Purpose:** Verify cache operations

```bash
# Test 1: View cache stats
rigrun cache stats
# Expected: Shows entries, hit rate, savings

# Test 2: Export cache (creates backup)
rigrun cache export ~/.rigrun/test/
# Expected: Files created with timestamp

# Test 3: Clear cache
echo "y" | rigrun cache clear
# Expected: "Cache cleared successfully"

# Test 4: Verify cache cleared
rigrun cache stats | grep "Entries:"
# Expected: "Entries: 0"
```

**Pass Criteria:**
- Stats command shows current cache state
- Export creates timestamped files
- Clear removes all entries with confirmation

---

### FUNC-006: Doctor Diagnostics

**Purpose:** Verify health check system

```bash
# Test 1: Run doctor
rigrun doctor
# Expected: List of checks with pass/warn/fail

# Test 2: Verify key checks are present
rigrun doctor | grep -E "(Ollama|GPU|Model|Config|Cache|OpenRouter)"
# Expected: All 7 checks mentioned

# Test 3: Test --fix flag (if applicable)
rigrun doctor --fix 2>&1 || true
```

**Pass Criteria:**
- Checks: Ollama installed, Ollama running, Model available, GPU detected, Config valid, Cache writable, OpenRouter configured
- Summary line shows pass/warn/fail counts
- Exit code 0 if all pass, 1 if any fail

---

### FUNC-007: Ask Command (Single Query)

**Purpose:** Verify single question mode

```bash
# Test 1: Simple question (requires Ollama)
rigrun ask "What is 2+2?" --quiet 2>/dev/null
# Expected: LLM response (e.g., "4")

# Test 2: Question with file inclusion
echo "def hello(): pass" > /tmp/test.py
rigrun ask "What does this do?" --file /tmp/test.py --quiet 2>/dev/null

# Test 3: Question with model override
rigrun ask "Say hello" --model qwen2.5:7b --quiet 2>/dev/null

# Test 4: Paranoid mode
rigrun ask "Hello" --paranoid --quiet 2>/dev/null
```

**Pass Criteria:**
- Returns LLM response
- --file flag includes file content
- --model flag overrides default
- --paranoid prevents cloud routing

---

### FUNC-008: Setup Wizard

**Purpose:** Verify first-run wizard

```bash
# Test 1: Check setup subcommands
rigrun setup --help 2>&1 || rigrun help
# Expected: Shows setup subcommands

# Test 2: GPU setup (non-interactive display)
rigrun setup gpu
# Expected: Shows GPU information and model recommendations

# Test 3: Model setup (requires input, skip in automation)
# rigrun setup model
```

**Pass Criteria:**
- setup gpu shows hardware detection
- setup model lists available models
- Full wizard requires interactive input

---

### FUNC-009: Session Management (via TUI)

**Purpose:** Verify session save/load functionality exists

```bash
# Test 1: Check conversations directory exists after use
ls -la ~/.rigrun/conversations/ 2>/dev/null || echo "No conversations yet"

# Test 2: List saved sessions (if any exist)
ls ~/.rigrun/conversations/*.json 2>/dev/null | head -5 || echo "No saved sessions"
```

**Pass Criteria:**
- Conversations directory created at `~/.rigrun/conversations/`
- Sessions saved as JSON files with unique IDs

---

### FUNC-010: Environment Variable Overrides

**Purpose:** Verify environment variables override config

```bash
# Test 1: Override model
RIGRUN_MODEL=test-model rigrun config show | grep "default_model"
# Note: Env override may not show in config show, test with status

# Test 2: Override paranoid mode
RIGRUN_PARANOID=1 rigrun status | grep "Paranoid"
# Expected: "Paranoid: Yes"

# Test 3: Override Ollama URL
RIGRUN_OLLAMA_URL=http://localhost:11435 rigrun status 2>&1 || true
```

**Pass Criteria:**
- RIGRUN_MODEL overrides default_model
- RIGRUN_PARANOID=1 enables paranoid mode
- RIGRUN_OLLAMA_URL changes Ollama endpoint
- RIGRUN_OPENROUTER_KEY sets API key

---

## Reliability Tests

### REL-001: Invalid Input Handling

**Purpose:** Verify graceful handling of invalid inputs

```bash
# Test 1: Invalid command
rigrun invalidcommand 2>&1
# Expected: Launches TUI or shows help (not crash)

# Test 2: Invalid config key
rigrun config set invalid_key value 2>&1 | grep -i "unknown"
# Expected: Error message about unknown key

# Test 3: Invalid mode value
rigrun config set default_mode invalid_mode 2>&1 | grep -i "invalid"
# Expected: Error about invalid mode

# Test 4: Empty question
rigrun ask "" 2>&1 | grep -i "no question"
# Expected: Error about missing question
```

**Pass Criteria:**
- No crashes or panics
- Helpful error messages
- Non-zero exit code for errors

---

### REL-002: Missing Dependencies

**Purpose:** Verify behavior when Ollama is not running

```bash
# Test 1: Stop Ollama (if possible) then run status
# Note: Cannot stop Ollama in automation, check error handling

# Test 2: Try ask when model not available
rigrun ask "Hello" --model nonexistent-model 2>&1 | grep -i "not found\|not available\|error"
# Expected: Graceful error about model
```

**Pass Criteria:**
- Clear error messages when Ollama is down
- Suggests solutions (e.g., "Start Ollama with: ollama serve")

---

### REL-003: File Permission Errors

**Purpose:** Verify handling of permission issues

```bash
# Test 1: Read-only config directory (simulation)
# Note: Requires elevated permissions to test fully

# Test 2: Invalid file path for ask --file
rigrun ask "Review:" --file /nonexistent/path.txt 2>&1 | grep -i "not found\|error"
# Expected: Error about file not found
```

**Pass Criteria:**
- Permission errors are reported clearly
- Application doesn't crash on permission denial

---

### REL-004: Resource Limits

**Purpose:** Verify handling of resource constraints

```bash
# Test 1: Large file with ask --file (50KB limit)
dd if=/dev/zero of=/tmp/large_file.txt bs=1024 count=100 2>/dev/null
rigrun ask "Review:" --file /tmp/large_file.txt 2>&1 | grep -i "too large\|limit"
# Expected: Error about file too large
rm /tmp/large_file.txt

# Test 2: Long query (check for truncation in logs)
rigrun ask "$(python3 -c 'print("a"*10000)')" --quiet 2>/dev/null || true
```

**Pass Criteria:**
- File size limit (50KB) is enforced
- Long queries are handled (truncated in logs)

---

## Audit Tests

### AUD-001: Audit Log Creation

**Purpose:** Verify audit logs are created

```bash
# Test 1: Enable audit logging
rigrun config set audit_enabled true

# Test 2: Perform an action
rigrun status

# Test 3: Check audit log exists
test -f ~/.rigrun/audit.log && echo "PASS: Audit log exists" || echo "FAIL: No audit log"

# Test 4: Check log format
head -5 ~/.rigrun/audit.log 2>/dev/null || echo "Audit log empty or not present"
```

**Pass Criteria:**
- Audit log created at `~/.rigrun/audit.log`
- Log entries have timestamp, event type, session ID
- Format: `TIMESTAMP | EVENT_TYPE | SESSION_ID | TIER | QUERY | TOKENS | COST | STATUS`

---

### AUD-002: Event Types Logged

**Purpose:** Verify all required events are logged

```bash
# Test 1: Check for startup events
grep "STARTUP\|SESSION_START" ~/.rigrun/audit.log 2>/dev/null || echo "No startup events"

# Test 2: Check for query events
grep "QUERY" ~/.rigrun/audit.log 2>/dev/null || echo "No query events"

# Test 3: Check for shutdown events
grep "SHUTDOWN\|SESSION_END" ~/.rigrun/audit.log 2>/dev/null || echo "No shutdown events"
```

**Pass Criteria:**
- STARTUP logged on application start
- QUERY logged for each LLM query
- SESSION_START/SESSION_END logged for sessions
- BANNER_ACK logged when consent banner acknowledged

---

### AUD-003: Log Rotation

**Purpose:** Verify logs rotate when size limit reached

```bash
# Test 1: Check max file size constant
# Default: 10MB (10 * 1024 * 1024 bytes)
# Note: Hard to test without generating 10MB of logs

# Test 2: Check for rotated logs
ls -la ~/.rigrun/audit*.log 2>/dev/null
# Expected: audit.log and possibly audit_YYYYMMDD_HHMMSS.log
```

**Pass Criteria:**
- Log rotation occurs at 10MB
- Rotated files have timestamp suffix
- Old logs are preserved

---

## Gap Analysis - Missing CLI Commands

The following CLI commands are **MISSING** and would be needed for IL5 compliance and operational use:

### Critical Gaps

| Missing Command | Priority | IL5 Requirement | Description |
|----------------|----------|-----------------|-------------|
| `rigrun audit show` | **HIGH** | AU-9 (Protection of Audit Info) | View audit logs via CLI |
| `rigrun audit export` | **HIGH** | AU-4 (Audit Storage Capacity) | Export audit logs for SIEM |
| `rigrun session list` | **HIGH** | AC-12 (Session Termination) | List active/saved sessions via CLI |
| `rigrun session load <id>` | **HIGH** | - | Load a session without TUI |
| `rigrun session export <id>` | **MEDIUM** | AU-3 (Content of Audit Records) | Export session for review |
| `rigrun test` | **HIGH** | - | Run built-in self-tests |
| `rigrun health` | **MEDIUM** | - | Kubernetes/container health check |
| `rigrun model list` | **MEDIUM** | - | List available models |
| `rigrun model pull <name>` | **MEDIUM** | - | Download a model |
| `rigrun benchmark` | **LOW** | - | Performance benchmarking |

### Security Gaps

| Missing Feature | Priority | IL5 Requirement | Description |
|----------------|----------|-----------------|-------------|
| `rigrun classify <level>` | **HIGH** | IA-4 (Identifier Management) | Set runtime classification level |
| `rigrun consent` | **HIGH** | AC-8 (System Use Notification) | Acknowledge consent banner via CLI |
| `rigrun lockout` | **MEDIUM** | AC-7 (Unsuccessful Logon Attempts) | Manual session lockout |
| `--audit-file` flag | **MEDIUM** | AU-9 | Custom audit log location |
| `--no-network` flag | **HIGH** | SC-7 (Boundary Protection) | Complete network isolation |

### Operational Gaps

| Missing Feature | Priority | Use Case | Description |
|----------------|----------|----------|-------------|
| `rigrun pipe` | **MEDIUM** | Automation | Read from stdin, write to stdout |
| `rigrun batch <file>` | **MEDIUM** | Automation | Process multiple queries from file |
| `rigrun server` | **LOW** | Integration | HTTP API server mode |
| `--json` output flag | **HIGH** | Automation | JSON output for parsing |
| `--format <fmt>` flag | **MEDIUM** | Automation | Output format selection |
| Exit codes documentation | **HIGH** | Automation | Defined exit codes for scripting |

---

## Sonnet Execution Instructions

This section provides instructions for a Claude Sonnet agent to execute all tests automatically.

### Pre-Execution Checklist

```bash
# 1. Verify rigrun exists
RIGRUN="C:\rigrun\go-tui\rigrun.exe"
test -f "$RIGRUN" || { echo "ERROR: rigrun not found"; exit 1; }

# 2. Create test results directory
mkdir -p ~/.rigrun/test_results

# 3. Set test timestamp
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE=~/.rigrun/test_results/results_${TIMESTAMP}.txt

# 4. Backup existing config
cp ~/.rigrun/config.toml ~/.rigrun/config.toml.pre_test 2>/dev/null || true
```

### Test Execution Script

The following bash script executes all CLI-based tests:

```bash
#!/bin/bash
# Rigrun IL5 Automated Test Suite
# Run with: bash test_rigrun.sh

RIGRUN="C:\rigrun\go-tui\rigrun.exe"
PASS=0
FAIL=0
SKIP=0

run_test() {
    local name="$1"
    local cmd="$2"
    local expected="$3"

    echo -n "TEST: $name ... "
    output=$($cmd 2>&1)

    if echo "$output" | grep -qi "$expected"; then
        echo "PASS"
        ((PASS++))
    else
        echo "FAIL"
        echo "  Command: $cmd"
        echo "  Expected: $expected"
        echo "  Got: ${output:0:200}"
        ((FAIL++))
    fi
}

echo "========================================="
echo "Rigrun IL5 Automated Test Suite"
echo "Started: $(date)"
echo "========================================="
echo ""

# Basic CLI Tests
run_test "Help command" "$RIGRUN --help" "Usage:"
run_test "Version command" "$RIGRUN version" "version"
run_test "Status command" "$RIGRUN status" "System"
run_test "Config show" "$RIGRUN config show" "[general]"
run_test "Cache stats" "$RIGRUN cache stats" "Entries:"
run_test "Doctor command" "$RIGRUN doctor" "passed"

# Config Tests
run_test "Config set mode" "$RIGRUN config set default_mode local" "[OK]"
run_test "Config set paranoid" "$RIGRUN config set paranoid_mode true" "[OK]"
run_test "Config path" "$RIGRUN config path" ".rigrun"

# Security Tests
run_test "Paranoid in status" "$RIGRUN status --paranoid" "Paranoid"
run_test "Key masking" "$RIGRUN config show" "****"

# Error Handling Tests
run_test "Invalid config key" "$RIGRUN config set invalid_key value 2>&1" "unknown"
run_test "Invalid mode" "$RIGRUN config set default_mode xyz 2>&1" "invalid"

echo ""
echo "========================================="
echo "Test Summary"
echo "========================================="
echo "PASSED: $PASS"
echo "FAILED: $FAIL"
echo "SKIPPED: $SKIP"
echo ""
echo "Finished: $(date)"

# Restore config
cp ~/.rigrun/config.toml.pre_test ~/.rigrun/config.toml 2>/dev/null || true

# Exit with appropriate code
if [ $FAIL -gt 0 ]; then
    exit 1
else
    exit 0
fi
```

### Sonnet Agent Instructions

1. **Environment Check:**
   - Verify rigrun.exe exists at `C:\rigrun\go-tui\rigrun.exe`
   - Check Ollama is running: `curl -s http://127.0.0.1:11434/`
   - Create backup of config: `cp ~/.rigrun/config.toml ~/.rigrun/config.toml.backup`

2. **Execute Tests Sequentially:**
   - Run each test command from this document
   - Record output and compare against expected output
   - Note pass/fail status

3. **Cleanup:**
   - Restore original config: `cp ~/.rigrun/config.toml.backup ~/.rigrun/config.toml`
   - Clear any test data created

4. **Reporting:**
   - Generate summary of pass/fail counts
   - List any failed tests with details
   - Note any gaps or issues discovered

### Pass/Fail Criteria Reference

| Exit Code | Meaning |
|-----------|---------|
| 0 | Command succeeded |
| 1 | General error |
| 2 | Invalid arguments |

### Critical Test Subset for Quick Validation

For quick IL5 compliance check, run these essential tests:

```bash
rigrun --help                           # Basic CLI works
rigrun status --paranoid                # Paranoid mode works
rigrun config set paranoid_mode true    # Security config works
rigrun config show | grep "****"        # Key masking works
rigrun doctor                           # Health checks pass
rigrun cache stats                      # Cache subsystem works
```

---

## Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-20 | Automated Generation | Initial test plan |

---

**END OF DOCUMENT**
