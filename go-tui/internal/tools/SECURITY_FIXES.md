# Security Fixes for Tool Permission Model

This document describes the comprehensive security fixes applied to address the vulnerabilities found in the tool permission system.

## Overview of Vulnerabilities

The security review identified four critical flaws in the permission model:

### Bug 1: Excessive Auto-Permissions
**Issue**: Read, Grep, and Glob tools were set to `PermissionAuto`, allowing AI to scan the entire filesystem without approval.

**Fix**: Implemented **context-aware permissions** that dynamically determine permission levels based on the target path:
- Normal files: `PermissionAuto` (no approval needed)
- Sensitive files (.env, credentials, SSH keys, etc.): `PermissionAsk` (requires approval)

### Bug 2: Path Traversal Before Canonicalization
**Issue**: Path validation checked for ".." BEFORE resolving the path, allowing bypasses like `/safe/path/../../../etc/shadow`.

**Fix**: Implemented proper path validation order:
1. Convert to absolute path
2. **Resolve symlinks** (using `filepath.EvalSymlinks`)
3. Canonicalize the path
4. **THEN** check for path traversal and blocked paths

### Bug 3: No Symlink Detection
**Issue**: Symlinks could bypass all path restrictions by pointing to blocked locations.

**Fix**: Added `filepath.EvalSymlinks()` to resolve symlinks to their real paths BEFORE validation, preventing symlink-based attacks.

### Bug 4: Bash Validation Uses Substring Matching
**Issue**: Command blocklist used simple substring matching, easily bypassed with:
- Extra spaces: `rm  -rf  /`
- Tabs: `rm\t-rf\t/`
- No spaces: `curl|bash`

**Fix**: Implemented improved command validation:
- Space/tab normalization (collapse multiple spaces)
- Token-based parsing (respecting quotes)
- Both substring and pattern matching
- Proper privileged command detection (sudo, su, doas, pkexec)

---

## Implementation Details

### 1. New Security Module (`security.go`)

Created a comprehensive security module with the following components:

#### Path Validation

```go
func ValidatePathSecure(path string) (string, error) {
    // 1. Convert to absolute path
    absPath, err := filepath.Abs(path)

    // 2. Resolve symlinks to get real path
    realPath, err := filepath.EvalSymlinks(absPath)

    // 3. Check for path traversal AFTER canonicalization
    if !isWithinAllowedPaths(realPath) {
        return "", ErrPathTraversal
    }

    // 4. Check against blocked paths
    if isBlockedPath(realPath) {
        return "", ErrBlockedPath
    }

    return realPath, nil
}
```

#### Sensitive Path Detection

```go
var SensitivePathPatterns = []string{
    "*/.env*",           // Environment files
    "*/.aws/*",          // Cloud credentials
    "*/.ssh/*",          // SSH keys
    "*/credentials*",    // Credential files
    "*/secrets*",        // Secret files
    "*/.git/config",     // Git configuration
    "*/password*",       // Password files
    "*.pem",             // Certificate files
    "*.key",             // Private key files
    // ... and more
}

func isSensitivePath(path string) bool {
    for _, pattern := range SensitivePathPatterns {
        if matchPath(path, pattern) {
            return true
        }
    }
    return false
}
```

#### Command Validation

```go
func ValidateCommandSecure(command string) error {
    // 1. Normalize: collapse spaces, lowercase
    normalized := normalizeCommand(command)

    // 2. Check against blocked commands
    for _, blocked := range DefaultBlockedCommands {
        if contains(normalized, blocked) {
            return ErrCommandBlocked
        }
    }

    // 3. Check dangerous patterns
    for _, pattern := range DefaultBlockedPatterns {
        if contains(normalized, pattern) {
            return ErrDangerousPattern
        }
    }

    // 4. Parse tokens and check first token
    tokens := parseCommandTokens(command)
    if isPrivilegedCommand(tokens[0]) {
        return ErrPrivilegedCommand
    }

    return nil
}
```

### 2. Context-Aware Permission System

Added `PermissionFunc` to the `Tool` struct:

```go
type Tool struct {
    Name           string
    Permission     PermissionLevel              // Static permission
    PermissionFunc func(params) PermissionLevel // Dynamic permission (overrides static)
    // ... other fields
}
```

Example for ReadTool:

```go
ReadTool = &Tool{
    Name:       "Read",
    Permission: PermissionAuto,  // Default
    PermissionFunc: func(params map[string]interface{}) PermissionLevel {
        filePath := params["file_path"].(string)
        return GetPermissionForPath(filePath)  // Context-aware
    },
    // ...
}
```

### 3. Updated Tool Implementations

#### Read Tool (`read.go`)
- Uses `ValidatePathSecure()` for all path validation
- Symlinks are resolved before validation
- Sensitive files trigger permission requirements

#### Glob Tool (`definitions.go`)
- Dynamic permission based on search directory
- Sensitive directories require approval

#### Grep Tool (`definitions.go`)
- Dynamic permission based on search directory
- Sensitive directories require approval

#### Bash Tool (`bash.go`)
- Uses `ValidateCommandSecure()` for command validation
- Token-based parsing with quote handling
- Space normalization to prevent bypasses

---

## Security Improvements Summary

| Vulnerability | Before | After |
|--------------|--------|-------|
| **Path Traversal** | Checked before canonicalization | Checked AFTER canonicalization and symlink resolution |
| **Symlink Bypass** | Not detected | Resolved with `filepath.EvalSymlinks()` before validation |
| **Filesystem Scanning** | AI could scan entire filesystem | Only allowed paths + sensitive paths require approval |
| **Command Bypass** | Simple substring match | Normalized matching + token parsing |

---

## Testing

Comprehensive test suite added in `security_test.go`:

### Path Validation Tests
- ✅ Valid paths in allowed directories
- ✅ Path traversal attempts blocked
- ✅ Symlink escapes detected
- ✅ Blocked system paths rejected

### Sensitive Path Detection
- ✅ .env files flagged as sensitive
- ✅ AWS/SSH credentials detected
- ✅ Certificate files (.pem, .key) detected
- ✅ Normal files pass through

### Command Validation Tests
- ✅ Safe commands allowed (ls, git status)
- ✅ Destructive commands blocked (rm -rf /)
- ✅ Remote execution blocked (curl | bash)
- ✅ Privilege escalation blocked (sudo)
- ✅ Space/tab bypasses prevented

### Integration Tests
- ✅ Bug 1: Path traversal after canonicalization
- ✅ Bug 2: Symlink detection
- ✅ Bug 3: Command parsing with extra spaces
- ✅ Bug 4: Context-aware permissions

---

## Usage Examples

### Reading a Normal File (Auto-Approved)
```go
// Normal file in project directory
result, err := ReadTool.Execute(ctx, map[string]interface{}{
    "file_path": "/home/user/project/main.go",
})
// Permission: PermissionAuto - no approval needed
```

### Reading a Sensitive File (Requires Approval)
```go
// Sensitive credential file
result, err := ReadTool.Execute(ctx, map[string]interface{}{
    "file_path": "/home/user/.aws/credentials",
})
// Permission: PermissionAsk - user must approve
```

### Command Validation
```go
// Safe command
err := ValidateCommandSecure("git status")  // ✅ Allowed

// Dangerous command
err := ValidateCommandSecure("rm -rf /")    // ❌ Blocked

// Bypass attempt
err := ValidateCommandSecure("rm  -rf  /")  // ❌ Still blocked (space normalization)

// Privileged command
err := ValidateCommandSecure("sudo rm file") // ❌ Requires approval
```

---

## Migration Guide

### For Tool Implementers

1. **Use `ValidatePathSecure()` instead of manual validation:**
   ```go
   // Before
   if strings.Contains(path, "..") { return error }
   path = filepath.Clean(path)

   // After
   validPath, err := ValidatePathSecure(path)
   if err != nil { return err }
   ```

2. **Add context-aware permissions to tools:**
   ```go
   MyTool = &Tool{
       Permission: PermissionAuto,  // Default
       PermissionFunc: func(params map[string]interface{}) PermissionLevel {
           // Check params and return appropriate level
           if isSensitiveOperation(params) {
               return PermissionAsk
           }
           return PermissionAuto
       },
   }
   ```

3. **Use `ValidateCommandSecure()` for command validation:**
   ```go
   // Before
   if strings.Contains(command, "rm -rf") { return error }

   // After
   if err := ValidateCommandSecure(command); err != nil {
       return err
   }
   ```

---

## Security Best Practices

1. **Always canonicalize paths BEFORE validation**
   - Use `filepath.Abs()` to get absolute path
   - Use `filepath.EvalSymlinks()` to resolve symlinks
   - THEN check against blocked paths

2. **Use context-aware permissions**
   - Don't blindly auto-approve all operations
   - Check if the operation involves sensitive data
   - Require approval for sensitive operations

3. **Normalize user input before matching**
   - Collapse multiple spaces
   - Convert to lowercase for case-insensitive matching
   - Handle tabs and other whitespace

4. **Use defense in depth**
   - Multiple layers of validation
   - Both pattern matching AND token parsing
   - Check both static lists AND dynamic patterns

---

## Future Enhancements

1. **Configurable sensitivity patterns** - Allow users to add custom sensitive patterns
2. **Audit logging** - Log all permission checks and approvals
3. **Rate limiting** - Prevent brute-force bypass attempts
4. **Machine learning** - Detect anomalous file access patterns
5. **Sandboxing** - Run tools in restricted containers

---

## References

- OWASP Path Traversal: https://owasp.org/www-community/attacks/Path_Traversal
- CWE-22: Improper Limitation of a Pathname to a Restricted Directory
- CWE-59: Improper Link Resolution Before File Access ('Link Following')
- CWE-78: Improper Neutralization of Special Elements used in an OS Command

---

## Changelog

### v2.0.0 (2025-01-22)
- **BREAKING**: All path operations now use `ValidatePathSecure()`
- **ADDED**: Context-aware permission system
- **ADDED**: Symlink detection and resolution
- **ADDED**: Sensitive path pattern matching
- **IMPROVED**: Command validation with space normalization
- **IMPROVED**: Token-based command parsing
- **FIXED**: Path traversal before canonicalization
- **FIXED**: Symlink bypass vulnerability
- **FIXED**: Command validation bypass with extra spaces
