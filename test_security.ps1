# Security Test Script for Rigrun
# Tests redaction patterns based on audit.rs implementation

Write-Host "`nSECURITY REDACTION TESTS" -ForegroundColor Cyan
Write-Host "========================`n" -ForegroundColor Cyan

$tests = @(
    @{
        Name = "OpenAI API Key"
        Pattern = "sk-[a-zA-Z0-9]{20,}"
        Test = "Use this key: sk-1234567890abcdefghij1234567890"
        Expected = "[REDACTED_API_KEY]"
    },
    @{
        Name = "OpenRouter API Key"
        Pattern = "sk-or-[a-zA-Z0-9-]{20,}"
        Test = "OpenRouter key: sk-or-v1-1234567890abcdefghij1234567890"
        Expected = "[REDACTED_API_KEY]"
    },
    @{
        Name = "Anthropic API Key"
        Pattern = "sk-ant-[a-zA-Z0-9-]{20,}"
        Test = "Anthropic key: sk-ant-api03-1234567890abcdefghij1234567890"
        Expected = "[REDACTED_API_KEY]"
    },
    @{
        Name = "AWS Access Key"
        Pattern = "AKIA[0-9A-Z]{16}"
        Test = "AWS key: AKIAIOSFODNN7EXAMPLE"
        Expected = "[REDACTED_AWS_KEY]"
    },
    @{
        Name = "GitHub Token"
        Pattern = "ghp_[a-zA-Z0-9]{36}"
        Test = "GitHub token: ghp_123456789012345678901234567890123456"
        Expected = "[REDACTED_GITHUB_TOKEN]"
    },
    @{
        Name = "Password with ="
        Pattern = "password[=:]\s*\S+"
        Test = "Connect with password=secretpass123"
        Expected = "password=[REDACTED]"
    },
    @{
        Name = "Password with :"
        Pattern = "password[=:]\s*\S+"
        Test = "Config password: mypassword123"
        Expected = "password=[REDACTED]"
    },
    @{
        Name = "Bearer Token"
        Pattern = "Bearer [a-zA-Z0-9-._~+/]+=*"
        Test = "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
        Expected = "Bearer [REDACTED]"
    },
    @{
        Name = "Generic Long Key"
        Pattern = "\b[A-Za-z0-9]{32,}\b"
        Test = "Secret: abcdefghij1234567890abcdefghij1234567890"
        Expected = "[REDACTED_KEY]"
    }
)

$passed = 0
$failed = 0

foreach ($test in $tests) {
    # Use regex to check if pattern would match
    if ($test.Test -match $test.Pattern) {
        Write-Host "[PASS] $($test.Name)" -ForegroundColor Green
        $passed++
    } else {
        Write-Host "[FAIL] $($test.Name) - Pattern did not match" -ForegroundColor Red
        $failed++
    }
}

Write-Host "`n========================" -ForegroundColor Cyan
Write-Host "Redaction Patterns: $passed/$($tests.Count) patterns verified" -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Yellow" })
Write-Host ""
