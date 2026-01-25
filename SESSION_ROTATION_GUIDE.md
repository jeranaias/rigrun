# Session ID Rotation Implementation Guide

## Overview

Session ID rotation has been implemented to prevent session fixation attacks. Session IDs are now automatically rotated in the following scenarios:

1. **Periodic Rotation**: Every 2 hours for long-lived sessions
2. **Privilege Escalation**: When user privilege level changes
3. **On Authentication**: After successful authentication (recommended)

## Changes Made

### 1. Session Manager (`C:/rigrun/src/security/session_manager.rs`)

#### New Privilege Level Enum
```rust
pub enum PrivilegeLevel {
    Guest = 0,
    User = 1,
    Admin = 2,
    System = 3,
}
```

#### Updated Session Struct
- Added `privilege_level: PrivilegeLevel` field
- Added `last_rotation: Instant` field to track rotation time

#### New Methods

**Check if periodic rotation is needed:**
```rust
pub fn should_rotate_periodic(&self) -> bool
```

**Update privilege level:**
```rust
pub fn set_privilege_level(&mut self, new_level: PrivilegeLevel) -> bool
```

**Update privilege with automatic rotation:**
```rust
pub fn update_privilege_level(&self, session_id: &str, new_level: PrivilegeLevel) -> Option<String>
```

**Check and rotate periodically:**
```rust
pub fn check_and_rotate_periodic(&self, session_id: &str) -> Option<String>
```

**Enhanced rotate_session_id:**
- Now updates `last_rotation` timestamp

### 2. Server Middleware (`C:/rigrun/src/server/mod.rs`)

#### Updated `validate_session` Middleware
- Automatically checks for periodic rotation on every request
- Returns new session ID via `X-New-Session-Id` header if rotation occurred
- Logs rotation events for audit trail

## Usage Examples

### Example 1: Automatic Periodic Rotation (Already Implemented)

The `validate_session` middleware automatically rotates sessions every 2 hours:

```rust
// In validate_session middleware (C:/rigrun/src/server/mod.rs:886-887)
let new_session_id = state.session_manager.check_and_rotate_periodic(session_id);

// Client receives X-New-Session-Id header if rotation occurred
if let Some(new_id) = new_session_id {
    response.headers_mut().insert("X-New-Session-Id", new_id);
}
```

### Example 2: Rotate on Authentication Success

Add this to your authentication handler after successful login:

```rust
async fn authenticate_user(
    State(state): State<Arc<AppState>>,
    Json(credentials): Json<LoginRequest>,
) -> Result<Json<LoginResponse>, UserError> {
    // Validate credentials...

    // Create new session
    let session = state.session_manager.create_session(&user_id);

    // Immediately rotate after creation for security
    let new_session_id = state.session_manager.rotate_session_id(
        &session.id,
        "post_authentication"
    );

    let final_session_id = new_session_id.unwrap_or(session.id);

    Ok(Json(LoginResponse {
        session_id: final_session_id,
        // ... other fields
    }))
}
```

### Example 3: Rotate on Privilege Escalation

When a user's privilege level changes:

```rust
async fn grant_admin_privileges(
    State(state): State<Arc<AppState>>,
    headers: HeaderMap,
) -> Result<Json<PrivilegeResponse>, UserError> {
    let session_id = headers.get("X-Session-Id")
        .and_then(|h| h.to_str().ok())
        .ok_or_else(|| UserError::authentication_required(None))?;

    // Update privilege level - automatically rotates if privilege changed
    let new_session_id = state.session_manager.update_privilege_level(
        session_id,
        PrivilegeLevel::Admin
    );

    Ok(Json(PrivilegeResponse {
        success: true,
        new_session_id,
        message: "Admin privileges granted. Session ID rotated for security.",
    }))
}
```

### Example 4: Manual Rotation for Sensitive Operations

For critical operations that should trigger rotation:

```rust
async fn perform_sensitive_operation(
    State(state): State<Arc<AppState>>,
    headers: HeaderMap,
) -> Result<Json<OperationResponse>, UserError> {
    let session_id = headers.get("X-Session-Id")
        .and_then(|h| h.to_str().ok())
        .ok_or_else(|| UserError::authentication_required(None))?;

    // Perform the operation...

    // Rotate session after sensitive operation
    if should_rotate_for_operation(&operation_type) {
        let new_session_id = state.session_manager.rotate_session_id(
            session_id,
            "sensitive_operation_completed"
        );

        // Return new session ID to client
        return Ok(Json(OperationResponse {
            success: true,
            new_session_id,
        }));
    }

    Ok(Json(OperationResponse {
        success: true,
        new_session_id: None,
    }))
}
```

## Client-Side Handling

Clients must check for the `X-New-Session-Id` header in responses and update their stored session ID:

```javascript
async function apiRequest(endpoint, data) {
    const response = await fetch(endpoint, {
        method: 'POST',
        headers: {
            'Authorization': `Bearer ${apiKey}`,
            'X-Session-Id': sessionId,
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
    });

    // Check for session rotation
    const newSessionId = response.headers.get('X-New-Session-Id');
    if (newSessionId) {
        console.log('Session rotated:', sessionId, '->', newSessionId);
        sessionId = newSessionId;
        // Persist new session ID
        localStorage.setItem('session_id', newSessionId);
    }

    return response.json();
}
```

## Response Headers

After session rotation, the following headers are added:

- `X-New-Session-Id`: The new session ID (only if rotation occurred)
- `X-Session-Expires-In`: Time remaining until session expires (seconds)
- `X-Session-State`: Current session state (ACTIVE, WARNING, etc.)
- `X-Session-Warning`: Warning message if in warning period

## Audit Logging

All session rotation events are logged with:

```
SESSION_ROTATED | old_session=<old_id> new_session=<new_id> reason=<reason>
```

Common rotation reasons:
- `periodic_rotation`: Automatic 2-hour rotation
- `privilege_escalation_to_Admin`: Privilege level changed
- `post_authentication`: After successful authentication
- `sensitive_operation_completed`: After sensitive operation

## Configuration

Rotation interval can be adjusted by modifying the constant in `Session::should_rotate_periodic()`:

```rust
const ROTATION_INTERVAL_SECS: u64 = 7200; // 2 hours (current default)
```

## Security Benefits

1. **Prevents Session Fixation**: Attackers can't fix a victim's session ID
2. **Limits Session Exposure**: Even if intercepted, session IDs are time-limited
3. **Privilege Separation**: Different privilege levels use different session IDs
4. **Audit Trail**: All rotations are logged for security monitoring

## Testing

Example test for session rotation:

```rust
#[tokio::test]
async fn test_privilege_escalation_rotation() {
    let manager = SessionManager::dod_stig_default();
    let session = manager.create_session("test_user");
    let old_id = session.id.clone();

    // Escalate privilege
    let new_id = manager.update_privilege_level(&old_id, PrivilegeLevel::Admin);

    assert!(new_id.is_some());
    assert_ne!(new_id.as_ref().unwrap(), &old_id);

    // Old session should not exist
    assert!(manager.get_session(&old_id).is_none());

    // New session should exist with Admin privilege
    let new_session = manager.get_session(new_id.unwrap().as_str()).unwrap();
    assert_eq!(new_session.privilege_level, PrivilegeLevel::Admin);
}
```

## Migration Notes

**Breaking Changes:**
- `Session` struct now has additional fields (`privilege_level`, `last_rotation`)
- Existing sessions will need to be recreated or migrated

**Backward Compatibility:**
- `Session::new()` still works (defaults to `PrivilegeLevel::User`)
- Existing session validation logic remains unchanged
- Session rotation is opt-in (only triggers when explicitly called or periodically)

## Recommendations

1. **Always rotate on authentication**: Implement Example 2 in your auth handlers
2. **Monitor rotation logs**: Set up alerts for unusual rotation patterns
3. **Educate clients**: Ensure client applications handle `X-New-Session-Id` header
4. **Test thoroughly**: Verify rotation doesn't break existing workflows
5. **Consider shorter intervals**: For high-security environments, reduce rotation interval

## Implementation Checklist

- [x] Add `PrivilegeLevel` enum to session manager
- [x] Update `Session` struct with new fields
- [x] Implement `should_rotate_periodic()` method
- [x] Implement `set_privilege_level()` method
- [x] Implement `update_privilege_level()` method
- [x] Implement `check_and_rotate_periodic()` method
- [x] Update `rotate_session_id()` to set `last_rotation`
- [x] Update `validate_session` middleware for periodic rotation
- [x] Add `X-New-Session-Id` header to responses
- [x] Export `PrivilegeLevel` from security module
- [ ] Add rotation on authentication handlers (TODO: implement in your auth code)
- [ ] Add rotation on privilege change handlers (TODO: implement in your privilege code)
- [ ] Update client-side code to handle session rotation
- [ ] Add tests for all rotation scenarios
- [ ] Update API documentation

## Support

For questions or issues, please refer to:
- Session Manager source: `C:/rigrun/src/security/session_manager.rs`
- Server middleware: `C:/rigrun/src/server/mod.rs`
- Security module exports: `C:/rigrun/src/security/mod.rs`
