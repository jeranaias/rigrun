# Integration Patch for PS-6 and PL-4 Commands

This document describes the changes needed to integrate the agreements and rules commands into the CLI.

## Files Created

1. `internal/security/agreements.go` - NIST 800-53 PS-6 (Access Agreements) implementation
2. `internal/security/rulesofbehavior.go` - NIST 800-53 PL-4 (Rules of Behavior) implementation
3. `internal/cli/agreements_cmd.go` - CLI commands for agreements and rules

## Changes Needed

### 1. internal/cli/cli.go

#### A. Add to the Command enum (around line 45):

```go
CmdAgreements // NIST 800-53 PS-6: Access Agreements
CmdRules      // NIST 800-53 PL-4: Rules of Behavior
```

#### B. Add to usage text (after auth command, around line 93):

```
  rigrun agreements [subcommand] Access agreements management (PS-6)
  rigrun rules [subcommand]   Rules of behavior management (PL-4)
```

#### C. Add detailed command documentation (around line 260):

```
Agreements Commands (NIST 800-53 PS-6: Access Agreements):
  rigrun agreements list          List all agreements
  rigrun agreements show <id>     Show agreement text
  rigrun agreements sign <id>     Sign agreement
  rigrun agreements status        Show user's agreement status
  rigrun agreements check         Verify all required agreements signed
    --user USER                   Specify user ID
    --json                        Output in JSON format

  Agreement Types: nda, aup, security, pii
  Compliance: Required for NIST 800-53 PS-6

Rules Commands (NIST 800-53 PL-4: Rules of Behavior):
  rigrun rules show               Show rules of behavior
    --category CATEGORY           Filter by category (general/security/data/network/incident)
  rigrun rules acknowledge        Acknowledge rules
  rigrun rules status             Show acknowledgment status
    --user USER                   Specify user ID
    --json                        Output in JSON format

  Categories: general, security, data, network, incident
  Compliance: Required for NIST 800-53 PL-4
```

#### D. Add to Parse() function switch statement (around line 440):

```go
case "agreements", "agreement":
	if len(remaining) > 0 {
		parsedArgs.Subcommand = remaining[0]
	}
	return CmdAgreements, parsedArgs

case "rules", "rob", "behavior":
	if len(remaining) > 0 {
		parsedArgs.Subcommand = remaining[0]
	}
	return CmdRules, parsedArgs
```

### 2. main.go

Add to the switch statement in main() function (after CmdAuth, around line 143):

```go
case cli.CmdAgreements:
	if err := cli.HandleAgreements(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
case cli.CmdRules:
	if err := cli.HandleRules(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
```

## Testing Commands

After integration, test with:

```bash
# List agreements
rigrun agreements list

# Show specific agreement
rigrun agreements show nda

# Sign agreement
rigrun agreements sign nda

# Check agreement status
rigrun agreements status
rigrun agreements check

# Show rules of behavior
rigrun rules show

# Acknowledge rules
rigrun rules acknowledge

# Check rules status
rigrun rules status

# JSON output for automation
rigrun agreements list --json
rigrun rules status --json
```

## Compliance Features

### PS-6 (Access Agreements)
- ✅ Agreement management (NDA, AUP, Security, PII)
- ✅ Digital signature tracking
- ✅ Expiration and renewal
- ✅ Revocation support
- ✅ Audit logging for all agreement events

### PL-4 (Rules of Behavior)
- ✅ Rules categorization (general, security, data, network, incident)
- ✅ Acknowledgment tracking
- ✅ Version control (requires re-acknowledgment on updates)
- ✅ Audit logging for acknowledgment events

## Storage

Data is stored in:
- `~/.rigrun/agreements.json` - Agreement signatures
- `~/.rigrun/rules.json` - Rules acknowledgments

Files are created with 0600 permissions (owner read/write only).

## Audit Integration

All agreement and rules operations are logged to the audit log:
- `AGREEMENT_SIGNED` - When user signs an agreement
- `AGREEMENT_REVOKED` - When agreement is revoked
- `RULES_ACKNOWLEDGED` - When user acknowledges rules
- `RULES_UPDATED` - When rules are updated

## Default Content

### Default Agreements:
1. **NDA** - Non-disclosure agreement
2. **AUP** - Acceptable use policy
3. **Security** - Security awareness acknowledgment
4. **PII** - PII handling agreement

All agreements expire after 365 days by default.

### Default Rules:
- General rules (2)
- Security rules (4)
- Data handling rules (3)
- Network rules (2)
- Incident response rules (2)

Total: 13 rules across 5 categories.
