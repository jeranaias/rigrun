# Spillage Detection Security Fixes

## Summary

Fixed critical security vulnerabilities in spillage detection that allowed trivial bypasses of classification marker detection.

## Vulnerabilities Fixed

### 1. Unicode Substitution Bypass
**Before:** Detection used simple regex patterns that could be bypassed with Unicode lookalikes.
- `ТOP SECRET` (Cyrillic T) - BYPASSED
- `Α` vs `A` (Greek Alpha vs Latin A) - BYPASSED

**After:** Implemented `normalizeForDetection()` function that:
- Normalizes Unicode to NFKD form
- Removes combining marks (accents, diacritics)
- Maps Cyrillic lookalikes to Latin equivalents (А→A, Т→T, О→O, etc.)
- Maps Greek lookalikes to Latin equivalents (Α→A, Τ→T, Ο→O, etc.)

### 2. Character Substitution Bypass
**Before:** Simple character-for-character matching allowed numeric/symbol substitutions.
- `T0P SECRET` (zero instead of O) - BYPASSED
- `S3CR3T` (3 instead of E) - BYPASSED
- `T@P $ECRET` (symbols) - BYPASSED

**After:** Lookalike detection maps common substitutions:
- Numbers to letters: 0→O, 1→I, 3→E, 4→A, 5→S, 7→T, 8→B
- Symbols to letters: @→A, $→S, !→I, |→I

### 3. Separator Bypass
**Before:** Separators broke pattern matching.
- `TOP_SECRET` (underscore) - BYPASSED
- `TOP-SECRET` (hyphen) - BYPASSED
- `TOP.SECRET` (period) - BYPASSED

**After:** Normalization removes all separators (_, -, ., spaces) before matching.

### 4. No Content-Based Detection
**Before:** Only keyword matching, missed encoded secrets.
- API keys: `test_key_aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2u` - MISSED
- Random tokens: `xK9#mP2$qL5@nR8&vT3!wY4%zU6` - MISSED

**After:** Added entropy analysis (`calculateEntropy()`):
- Implements Shannon entropy calculation
- Flags strings with entropy > 4.5 bits as potential secrets
- Complements pattern matching with statistical analysis

### 5. Lossy Sanitization
**Before:** Original data destroyed without backup.
- `SanitizeFile()` overwrote files directly
- No way to recover original content
- No audit trail of changes

**After:** Implemented `createBackup()` function:
- Creates timestamped backups in `.spillage-backup/` directory
- Preserves original content with restrictive permissions (0600)
- Logs backup creation to audit trail
- Backup path: `<dir>/.spillage-backup/<filename>.<timestamp>.backup`

## Implementation Details

### Unicode Normalization Function
```go
func normalizeForDetection(s string) string {
    // 1. Unicode normalization (NFKD)
    t := transform.Chain(norm.NFKD)
    normalized, _, _ := transform.String(t, s)

    // 2. Remove combining marks
    // 3. Convert to uppercase
    // 4. Replace lookalikes (Cyrillic, Greek, numbers, symbols)
    // 5. Remove separators
    return normalized
}
```

### Entropy Calculation
```go
func calculateEntropy(s string) float64 {
    // Shannon entropy: H(X) = -Σ p(x) * log₂(p(x))
    freq := make(map[rune]int)
    for _, r := range s {
        freq[r]++
    }

    var entropy float64
    length := float64(len(s))
    for _, count := range freq {
        p := float64(count) / length
        entropy -= p * math.Log2(p)
    }
    return entropy
}
```

### Backup Creation
```go
func (s *SpillageManager) createBackup(originalPath string, content []byte) (string, error) {
    // 1. Create .spillage-backup directory with 0700 permissions
    // 2. Generate timestamped filename
    // 3. Write backup with 0600 permissions
    // 4. Log to audit trail
    return backupPath, nil
}
```

### Overlap Detection
To prevent duplicate detection of substrings (e.g., "SECRET" within "TOP SECRET"), the detection system now:
- Tracks matched ranges in normalized content
- Skips overlapping matches from less-specific patterns
- Patterns ordered from most to least specific

## Test Coverage

All security fixes verified with comprehensive test suite:

### Test: Unicode Normalization
- ✓ Simple uppercase: "TOP SECRET"
- ✓ Number substitution: "T0P SECRET"
- ✓ Cyrillic substitution: "ТOP SECRET"
- ✓ Underscore separator: "TOP_SECRET"
- ✓ Multiple substitutions: "T0P_S3CR3T"
- ✓ Mixed Cyrillic/Latin: "ТОPSECRЕТ"

### Test: Bypass Detection
- ✓ Normal classification markers detected
- ✓ Zero-substitution bypasses caught
- ✓ Cyrillic-substitution bypasses caught
- ✓ Separator bypasses caught
- ✓ Multiple bypass techniques caught
- ✓ No false positives on unclassified text

### Test: Entropy Analysis
- ✓ API keys detected (e.g., `sk_live_51...`)
- ✓ Random secrets detected
- ✓ Normal text not flagged
- ✓ Configurable entropy threshold (4.5 bits)

### Test: Backup Functionality
- ✓ Backup created before sanitization
- ✓ Original content preserved in backup
- ✓ Sanitized content written to original file
- ✓ Backup directory created with proper permissions
- ✓ Audit log entry created

## Files Modified

1. **C:\rigrun\go-tui\internal\security\spillage.go**
   - Added Unicode normalization support
   - Added entropy calculation
   - Added backup functionality
   - Updated detection patterns for normalized content
   - Added fuzzy matching for sanitization
   - Enhanced with comprehensive documentation

2. **C:\rigrun\go-tui\internal\security\spillage_test.go** (NEW)
   - Comprehensive test coverage for all security fixes
   - Tests for normalization, entropy, bypass detection
   - Tests for backup functionality
   - All tests passing

## Security Impact

**Before:** Spillage detection could be trivially bypassed with:
- Character substitution (0→O, 1→I, etc.)
- Unicode lookalikes (Cyrillic/Greek characters)
- Separator insertion (underscores, hyphens)
- Encoded secrets completely missed

**After:** Robust detection that:
- Normalizes all input before pattern matching
- Detects obfuscation attempts
- Uses entropy analysis for secrets
- Preserves original data in backups
- Maintains comprehensive audit trail

## Dependencies Added

- `golang.org/x/text/transform` - Unicode transformation
- `golang.org/x/text/unicode/norm` - Unicode normalization (NFKD)
- `math` - Entropy calculations (Log2)

## Backward Compatibility

All changes are backward compatible:
- Existing API unchanged
- New fields added to SpillageEvent (DetectionType, Entropy)
- Backup functionality is transparent
- Patterns updated but behavior enhanced, not changed

## Recommendations

1. **Review Backups:** Periodically review `.spillage-backup/` directories
2. **Entropy Threshold:** Consider adjusting threshold (4.5) based on false positive rate
3. **Pattern Tuning:** Add organization-specific classification patterns
4. **Monitoring:** Monitor audit logs for detection events
5. **Testing:** Test with real-world obfuscation attempts

## References

- NIST 800-53 IR-9: Information Spillage Response
- Unicode Normalization: https://unicode.org/reports/tr15/
- Shannon Entropy: https://en.wikipedia.org/wiki/Entropy_(information_theory)
- Homoglyph Attack: https://en.wikipedia.org/wiki/IDN_homograph_attack
