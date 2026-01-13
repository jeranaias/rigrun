# Security and Privacy

rigrun is designed with privacy maximalism in mind. This document explains what data rigrun stores, where it goes, and how to maintain complete privacy.

---

## Table of Contents

- [Privacy Philosophy](#privacy-philosophy)
- [Local Data Storage](#local-data-storage)
- [Cloud Data Transmission](#cloud-data-transmission)
- [Paranoid Mode](#paranoid-mode)
- [Audit Logging](#audit-logging)
- [API Key Management](#api-key-management)
- [Authentication](#authentication)
- [Data Export and Deletion](#data-export-and-deletion)
- [Security Best Practices](#security-best-practices)

---

## Privacy Philosophy

**Your data is yours. rigrun operates on these principles:**

1. **Local-First**: 90% of requests handled locally without any cloud transmission
2. **Transparent**: Audit logs show exactly what was sent where
3. **Controllable**: Paranoid mode blocks ALL cloud access
4. **Minimal**: Only essential data is stored or transmitted
5. **Deletable**: You can export and delete all data at any time

---

## Local Data Storage

rigrun stores data in `~/.rigrun/` (Unix/macOS) or `C:\Users\<USERNAME>\.rigrun\` (Windows).

### Configuration (`config.json`)

**Location**: `~/.rigrun/config.json`

**Contains**:
- OpenRouter API key (if configured)
- Model preferences
- Port settings
- First-run status

**Format**: Plain JSON

**Security Considerations**:
- API keys stored in plaintext (file permissions protect it)
- Readable only by your user account
- Should NOT be committed to version control

**Protection**:
```bash
# Unix/macOS - verify permissions
ls -l ~/.rigrun/config.json
# Should show: -rw------- (600)

# If not, fix:
chmod 600 ~/.rigrun/config.json
```

### Cache (`cache/`)

**Location**: `~/.rigrun/cache/`

**Contains**:
- Cached query responses
- Embedding vectors for semantic matching
- Cache metadata (timestamps, hit counts)

**What's Cached**:
- Query text (first 50 characters)
- Model responses
- Embeddings (numerical vectors)

**Retention**: 24 hours TTL (time-to-live)

**Security Considerations**:
- Contains snippets of your queries and AI responses
- Stored as JSON (readable by your user)
- Automatically expires after 24 hours

**Clear Cache**:
```bash
rm -rf ~/.rigrun/cache
```

### Audit Log (`audit.log`)

**Location**: `~/.rigrun/audit.log`

**Contains**:
- Timestamp of each request
- Routing decision (cache/local/cloud)
- Query preview (first 30 characters)
- Token counts
- Cost estimates

**Example Entry**:
```
2024-01-15 10:23:45 |     CACHE_HIT | "What is recursi..." | 0 tokens | $0.00
2024-01-15 10:24:12 |         LOCAL | "Explain async/a..." | 847 tokens | $0.00
2024-01-15 10:25:33 |         CLOUD | "Design a microservices..." | 1234 tokens | $0.03
```

**Security Considerations**:
- Logs query previews (30 chars max)
- Rotates at 100MB or 7 days
- API keys are redacted (see below)

**Disable Logging**:
Add to `config.json`:
```json
{
  "audit_log_enabled": false
}
```

### Stats (`stats.json`)

**Location**: `~/.rigrun/stats.json`

**Contains**:
- Query counts (total, local, cloud)
- Token usage
- Cost savings calculations
- Daily/session statistics

**Security Considerations**:
- No query text stored
- Only aggregated statistics
- Safe to share for benchmarking

---

## Cloud Data Transmission

### When Data Goes to Cloud

**Scenario 1: OpenRouter Fallback**

When OpenRouter is configured and a query is routed to cloud:

**What's Sent**:
- Full conversation history (`messages` array)
- Model preferences
- Generation parameters (temperature, max_tokens)

**What's NOT Sent**:
- Your IP address (sent by OpenRouter, not rigrun)
- User identifiers (rigrun doesn't track users)
- Cache data or stats

**Who Sees It**:
- OpenRouter (receives request)
- The model provider (e.g., Anthropic, OpenAI)

**How to Prevent**:
- Don't configure OpenRouter key (no cloud fallback)
- Use `--paranoid` flag (blocks all cloud)
- Use `model: "local"` explicitly in requests

### Data Redaction

**API Key Protection**:

rigrun automatically redacts API keys from logs:

```
Before: sk-or-v1-abc123xyz789...
After:  sk-or-***REDACTED***
```

Patterns detected and redacted:
- `sk-*` (OpenAI keys)
- `sk-or-*` (OpenRouter keys)
- `sk-ant-*` (Anthropic keys)
- Bearer tokens
- Password fields

**Configuration**:
```json
{
  "audit_log_redaction": true  // Default: enabled
}
```

---

## Paranoid Mode

**100% local operation - your data NEVER leaves your machine.**

### Enable Paranoid Mode

**Method 1: CLI Flag**
```bash
rigrun --paranoid
```

**Method 2: Configuration**
```json
{
  "paranoid_mode": true
}
```

### What Paranoid Mode Does

1. **Blocks ALL cloud requests**
   - Returns error if cloud model requested
   - Logs blocked attempts
   - Shows warning banner on startup

2. **Forces local-only routing**
   - Cache → Local GPU only
   - No OpenRouter calls
   - No external network access

3. **Audit trail**
   - Logs blocked cloud attempts
   - Shows "CLOUD_BLOCKED" in audit log

### Example Output

**Startup:**
```
⚠️  PARANOID MODE ENABLED ⚠️
All cloud requests will be blocked.
Only local inference available.

✓ Server: http://localhost:8787
```

**Blocked Request:**
```
[✗] Cloud request blocked by paranoid mode

This request requires cloud access, which is blocked in paranoid mode.

To allow cloud requests:
  1. Stop rigrun (Ctrl+C)
  2. Disable paranoid mode: rigrun config set paranoid_mode false
  3. Restart: rigrun

Or use local model: {"model":"local","messages":[...]}
```

**Audit Log Entry:**
```
2024-01-15 10:25:33 | CLOUD_BLOCKED | "Design a microservices..." | 0 tokens | $0.00
```

---

## Audit Logging

### What Gets Logged

**Every request logs**:
- Timestamp
- Routing tier (CACHE_HIT, LOCAL, CLOUD, CLOUD_BLOCKED)
- Query preview (first 30 characters)
- Token counts
- Cost estimate

**What's NOT logged**:
- Full query text (only 30-char preview)
- Full responses
- API keys (redacted automatically)
- User identifiers
- IP addresses

### Log Rotation

**Automatic rotation triggers**:
- Size limit: 100MB
- Age limit: 7 days retention

**Manual rotation**:
```bash
# Backup and clear
mv ~/.rigrun/audit.log ~/.rigrun/audit.log.backup
touch ~/.rigrun/audit.log
```

### Disable Audit Logging

If you don't want any logging:

**Edit `~/.rigrun/config.json`**:
```json
{
  "audit_log_enabled": false
}
```

**Restart rigrun**:
```bash
rigrun
```

**Verify**:
```bash
ls -l ~/.rigrun/audit.log
# Should not grow
```

---

## API Key Management

### Storage

**Current Implementation**:
- API keys stored in `~/.rigrun/config.json`
- Plain text (protected by file permissions)

**Future Implementation** (planned):
- System keyring integration
  - macOS: Keychain
  - Windows: Credential Manager
  - Linux: Secret Service (libsecret)

### Setting API Keys

**Method 1: CLI**
```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
```

**Method 2: Environment Variable**
```bash
export OPENROUTER_API_KEY=sk-or-v1-xxxxx
rigrun
```

**Method 3: Manual Edit**
```bash
nano ~/.rigrun/config.json
```

Add:
```json
{
  "openrouter_key": "sk-or-v1-xxxxx"
}
```

### Key Security Best Practices

1. **Never commit keys to git**
```bash
# Add to .gitignore
echo ".rigrun/config.json" >> .gitignore
```

2. **Use environment variables in CI/CD**
```bash
export OPENROUTER_API_KEY=${{ secrets.OPENROUTER_KEY }}
```

3. **Rotate keys periodically**
- Generate new key at https://openrouter.ai/keys
- Update config
- Delete old key

4. **Revoke compromised keys immediately**
- Visit https://openrouter.ai/keys
- Delete compromised key
- Generate new one

---

## Authentication

### Current Status

**rigrun server has NO authentication by default.**

This is appropriate for:
- Local development
- Personal use on localhost
- Trusted network environments

### Future Authentication (Planned)

**Phase 1.2 of remediation plan adds**:
- Bearer token authentication
- `--api-key` flag for server startup
- Protected endpoints:
  - `POST /v1/chat/completions`
  - `GET /stats`
  - `GET /cache/stats`
- Public endpoints (no auth):
  - `GET /health`
  - `GET /v1/models`

### Securing rigrun Today

**If exposing rigrun beyond localhost**:

**1. Use reverse proxy with auth**:
```nginx
# nginx example
location / {
    auth_basic "rigrun";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8787;
}
```

**2. Use SSH tunneling**:
```bash
# From remote machine
ssh -L 8787:localhost:8787 user@your-server
```

**3. Firewall rules**:
```bash
# Only allow localhost
sudo ufw deny 8787
# Or allow specific IP
sudo ufw allow from 192.168.1.100 to any port 8787
```

**4. Network isolation**:
- Run rigrun on private network
- Don't expose port 8787 to internet
- Use VPN for remote access

---

## Data Export and Deletion

### Export All Data

```bash
rigrun export
```

Creates timestamped archive:
```
~/.rigrun/exports/rigrun-export-2024-01-15-143022.tar.gz
```

**Contains**:
- Configuration (config.json)
- Cache data
- Audit logs
- Statistics

**Custom output location**:
```bash
rigrun export --output ~/my-backups/
```

### Delete All Data

**Complete removal**:
```bash
rm -rf ~/.rigrun
```

**Partial cleanup**:
```bash
# Keep config, delete cache and logs
rm -rf ~/.rigrun/cache
rm ~/.rigrun/audit.log
rm ~/.rigrun/stats.json
```

**After deletion**:
```bash
rigrun
# Creates fresh config, runs first-time setup
```

---

## Security Best Practices

### For Individual Users

1. **Use Paranoid Mode for sensitive work**
```bash
rigrun --paranoid
```

2. **Regularly review audit logs**
```bash
tail -f ~/.rigrun/audit.log
```

3. **Clear cache periodically**
```bash
# Weekly or monthly
rm -rf ~/.rigrun/cache
```

4. **Keep rigrun updated**
```bash
cargo install rigrun --force
# Or download latest release
```

### For Teams/Organizations

1. **Document data policies**
- What data can be sent to cloud?
- When is paranoid mode required?
- How often to rotate keys?

2. **Use environment variables for keys**
```bash
# Don't store keys in config files
export OPENROUTER_API_KEY=$VAULT_KEY
```

3. **Audit log retention**
```bash
# Centralize logs
cp ~/.rigrun/audit.log /central/logs/rigrun-$(date +%Y%m%d).log
```

4. **Network segmentation**
- Run rigrun on isolated network
- Use VPN for access
- Firewall external access

5. **Regular security reviews**
- Review what data is cached
- Check for API key exposure
- Monitor cloud usage

### For Compliance (GDPR, HIPAA, etc.)

1. **Data minimization**
```json
{
  "audit_log_enabled": false,  // Disable if logging sensitive data
  "paranoid_mode": true         // Force local-only
}
```

2. **Right to erasure**
```bash
# User can delete all data
rigrun export  # Backup first
rm -rf ~/.rigrun
```

3. **Data portability**
```bash
# Export in standard format
rigrun export --format json
```

4. **Audit trails**
```bash
# Keep logs for compliance period
cp ~/.rigrun/audit.log /compliance/archive/
```

---

## Threat Model

### What rigrun Protects Against

✅ **API key exposure in logs** - Automatic redaction
✅ **Unintended cloud transmission** - Paranoid mode
✅ **Cache data persistence** - 24-hour TTL, manual clearing
✅ **Unauthorized local access** - File permissions

### What rigrun Does NOT Protect Against

❌ **Local system compromise** - If attacker has your user access, they can read config/cache
❌ **Network eavesdropping** - No TLS by default (localhost only)
❌ **Cloud provider data retention** - Once sent to OpenRouter/Anthropic, follows their policies
❌ **Physical access to machine** - No encryption at rest (planned for future)

### Security Improvements (Planned)

**Phase 1 (Security Hardening)**:
- ✅ API key redaction (completed)
- ⏳ Bearer token authentication (planned)
- ⏳ Keyring integration (planned)
- ⏳ Cache encryption (planned)

**Phase 2 (Network Security)**:
- ⏳ TLS support (planned)
- ⏳ CORS configuration (planned)
- ⏳ Rate limiting improvements (planned)

---

## Privacy Comparison

### rigrun vs Cloud-Only (OpenAI/Anthropic)

| Aspect | rigrun (Local) | Cloud-Only |
|--------|---------------|------------|
| **Data Transmission** | 10% of requests | 100% of requests |
| **Storage** | Your machine only | Provider's servers |
| **Audit Trail** | Full local logs | Limited access |
| **Retention** | You control | Provider policy |
| **GDPR Compliance** | Easier | Complex |
| **Zero-Trust** | Achievable | Impossible |

### rigrun Paranoid Mode vs Cloud

| Feature | Paranoid Mode | Cloud |
|---------|--------------|-------|
| **Local Only** | ✅ 100% | ❌ 0% |
| **No Internet** | ✅ Works offline | ❌ Requires internet |
| **Data Leaves Machine** | ❌ Never | ✅ Always |
| **Third-Party Access** | ❌ Never | ✅ Provider sees all |

---

## Frequently Asked Questions

**Q: Can OpenRouter see my queries?**

A: Only queries routed to cloud (typically 10%). Cache and local queries never leave your machine.

**Q: Does rigrun phone home?**

A: No. rigrun has zero telemetry or analytics.

**Q: What happens if I don't configure OpenRouter?**

A: rigrun works 100% locally. No cloud access at all.

**Q: Can I use rigrun completely offline?**

A: Yes, with paranoid mode:
```bash
rigrun --paranoid
```

**Q: Is my data encrypted?**

A: Currently: File system permissions protect data. Future: Cache encryption planned.

**Q: How do I ensure complete privacy?**

A: Use paranoid mode + disable audit logging:
```json
{
  "paranoid_mode": true,
  "audit_log_enabled": false
}
```

**Q: What if I accidentally send sensitive data to cloud?**

A:
1. Check audit log to confirm what was sent
2. Contact OpenRouter to request deletion
3. Enable paranoid mode to prevent future occurrences

---

## Reporting Security Issues

**DO NOT open public issues for security vulnerabilities.**

**Contact**: security@rigrun.com (if available) or private email to maintainers

**Include**:
- Description of vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (optional)

**Response time**: 48 hours for critical issues

---

## Related Documentation

- [Getting Started](getting-started.md) - Setup guide
- [Configuration](configuration.md) - All configuration options
- [Troubleshooting](troubleshooting.md) - Common issues
- [API Reference](api-reference.md) - API documentation

---

## Privacy Statement

rigrun respects your privacy:

1. **No Telemetry**: rigrun doesn't collect usage statistics
2. **No Phone Home**: No automatic update checks or version reporting
3. **No Third-Party Tracking**: No analytics, ads, or trackers
4. **Open Source**: All code is auditable
5. **Local-First**: Designed to work 100% locally

**Your data stays on your machine unless you explicitly configure cloud fallback.**
