// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package security provides IL5 security controls.
//
// This file contains tests for NIST 800-53 SC-28 encryption compliance:
// - Key derivation (PBKDF2-SHA-256)
// - AES-256-GCM encryption/decryption
// - Nonce uniqueness
// - Round-trip encryption
package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// =============================================================================
// KEY DERIVATION TESTS
// =============================================================================

// TestEncryption_KeyDerivation tests that PBKDF2 key derivation is deterministic.
func TestEncryption_KeyDerivation(t *testing.T) {
	password := "testpassword123"
	salt := []byte("test_salt_value!")

	// Same password and salt should derive same key
	key1 := DeriveKey(password, salt)
	key2 := DeriveKey(password, salt)
	require.True(t, bytes.Equal(key1, key2), "Same password/salt should derive same key")

	// Different salt should derive different key
	salt2 := []byte("different_salt!!")
	key3 := DeriveKey(password, salt2)
	require.False(t, bytes.Equal(key1, key3), "Different salt should derive different key")

	// Different password should derive different key
	key4 := DeriveKey("differentpassword", salt)
	require.False(t, bytes.Equal(key1, key4), "Different password should derive different key")
}

// TestEncryption_KeyDerivationLength tests that derived keys are the correct length.
func TestEncryption_KeyDerivationLength(t *testing.T) {
	key := DeriveKey("password", []byte("salt"))
	require.Equal(t, KeySize, len(key), "Derived key should be %d bytes (256 bits)", KeySize)
}

// TestEncryption_KeyDerivationDeterministic tests key derivation is deterministic.
func TestEncryption_KeyDerivationDeterministic(t *testing.T) {
	password := "consistent_password"
	salt := []byte("consistent_salt!")

	// Derive key multiple times
	keys := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		keys[i] = DeriveKey(password, salt)
	}

	// All keys should be identical
	for i := 1; i < 100; i++ {
		require.True(t, bytes.Equal(keys[0], keys[i]),
			"Key derivation must be deterministic (iteration %d differs)", i)
	}
}

// TestEncryption_KeyDerivationEmptyPassword tests that empty passwords work.
func TestEncryption_KeyDerivationEmptyPassword(t *testing.T) {
	// Empty password should still derive a key (though not recommended)
	salt := []byte("test_salt_value!")
	key := DeriveKey("", salt)
	require.Equal(t, KeySize, len(key), "Empty password should still derive a valid key")

	// Empty password should derive different key than non-empty
	keyNonEmpty := DeriveKey("password", salt)
	require.False(t, bytes.Equal(key, keyNonEmpty))
}

// =============================================================================
// SALT GENERATION TESTS
// =============================================================================

// TestEncryption_GenerateSalt tests salt generation.
func TestEncryption_GenerateSalt(t *testing.T) {
	salt, err := GenerateSalt()
	require.NoError(t, err)
	require.Equal(t, SaltSize, len(salt), "Salt should be %d bytes", SaltSize)

	// Generate multiple salts and ensure they're unique
	salts := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := GenerateSalt()
		require.NoError(t, err)
		saltStr := string(s)
		require.False(t, salts[saltStr], "Duplicate salt generated")
		salts[saltStr] = true
	}
}

// TestEncryption_GenerateMasterKey tests master key generation.
func TestEncryption_GenerateMasterKey(t *testing.T) {
	key, err := GenerateMasterKey()
	require.NoError(t, err)
	require.Equal(t, KeySize, len(key), "Master key should be %d bytes", KeySize)

	// Generate multiple keys and ensure they're unique
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		k, err := GenerateMasterKey()
		require.NoError(t, err)
		keyStr := string(k)
		require.False(t, keys[keyStr], "Duplicate master key generated")
		keys[keyStr] = true
	}
}

// =============================================================================
// ENCRYPTION ROUND-TRIP TESTS
// =============================================================================

// TestEncryption_RoundTrip tests basic encryption and decryption.
func TestEncryption_RoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	// Create encryption manager and initialize
	em := createTestEncryptionManager(t, keyPath)

	plaintext := []byte("sensitive data that needs protection")

	// Encrypt
	ciphertext, err := em.Encrypt(plaintext)
	require.NoError(t, err)
	require.NotEqual(t, plaintext, ciphertext, "Ciphertext should differ from plaintext")

	// Ciphertext should be larger than plaintext (nonce + tag overhead)
	require.Greater(t, len(ciphertext), len(plaintext), "Ciphertext should be larger due to overhead")

	// Decrypt
	decrypted, err := em.Decrypt(ciphertext)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted, "Decrypted data should match original")
}

// TestEncryption_RoundTripString tests string encryption and decryption.
func TestEncryption_RoundTripString(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	original := "This is a secret API key: sk-test-1234567890"

	// Encrypt
	encrypted, err := em.EncryptString(original)
	require.NoError(t, err)
	require.True(t, IsEncrypted(encrypted), "Encrypted string should have ENC: prefix")
	require.NotEqual(t, original, encrypted)

	// Decrypt
	decrypted, err := em.DecryptString(encrypted)
	require.NoError(t, err)
	require.Equal(t, original, decrypted)
}

// TestEncryption_RoundTripEmptyData tests encryption of empty data.
func TestEncryption_RoundTripEmptyData(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	// Empty plaintext
	plaintext := []byte{}

	ciphertext, err := em.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := em.Decrypt(ciphertext)
	require.NoError(t, err)
	// For empty data, decryption may return nil instead of empty slice
	// Both are valid representations of "no data"
	require.Equal(t, 0, len(decrypted), "Decrypted data should have zero length")
}

// TestEncryption_RoundTripLargeData tests encryption of large data.
func TestEncryption_RoundTripLargeData(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	// 1MB of random data
	plaintext := make([]byte, 1024*1024)
	_, err := rand.Read(plaintext)
	require.NoError(t, err)

	ciphertext, err := em.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := em.Decrypt(ciphertext)
	require.NoError(t, err)
	require.True(t, bytes.Equal(plaintext, decrypted), "Large data round-trip failed")
}

// TestEncryption_RoundTripBinaryData tests encryption of binary data with null bytes.
func TestEncryption_RoundTripBinaryData(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	// Binary data with null bytes and special characters
	plaintext := []byte{0x00, 0x01, 0xFF, 0xFE, 0x00, 0x00, 0x42, 0x00}

	ciphertext, err := em.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := em.Decrypt(ciphertext)
	require.NoError(t, err)
	require.True(t, bytes.Equal(plaintext, decrypted), "Binary data round-trip failed")
}

// =============================================================================
// NONCE UNIQUENESS TESTS
// =============================================================================

// TestEncryption_NonceUniqueness tests that each encryption produces a unique nonce.
func TestEncryption_NonceUniqueness(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	plaintext := []byte("test data")
	nonces := make(map[string]bool)

	// Encrypt the same data multiple times
	for i := 0; i < 100; i++ {
		ciphertext, err := em.Encrypt(plaintext)
		require.NoError(t, err)

		// Extract nonce (first NonceSize bytes)
		nonce := string(ciphertext[:NonceSize])
		require.False(t, nonces[nonce], "Nonce reuse detected at iteration %d", i)
		nonces[nonce] = true
	}
}

// TestEncryption_ConcurrentNonceUniqueness tests nonce uniqueness under concurrent use.
func TestEncryption_ConcurrentNonceUniqueness(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	plaintext := []byte("test data")
	var mu sync.Mutex
	nonces := make(map[string]bool)
	var wg sync.WaitGroup

	// Encrypt concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ciphertext, err := em.Encrypt(plaintext)
			require.NoError(t, err)

			// Extract nonce
			nonce := string(ciphertext[:NonceSize])

			mu.Lock()
			defer mu.Unlock()
			require.False(t, nonces[nonce], "Concurrent nonce reuse detected")
			nonces[nonce] = true
		}()
	}
	wg.Wait()
}

// =============================================================================
// CIPHERTEXT INTEGRITY TESTS
// =============================================================================

// TestEncryption_TamperedCiphertext tests that tampered ciphertext is detected.
func TestEncryption_TamperedCiphertext(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	plaintext := []byte("sensitive data")
	ciphertext, err := em.Encrypt(plaintext)
	require.NoError(t, err)

	// Tamper with the ciphertext
	if len(ciphertext) > NonceSize+1 {
		ciphertext[NonceSize+1] ^= 0xFF // Flip bits in the ciphertext portion
	}

	// Decryption should fail
	_, err = em.Decrypt(ciphertext)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDecryptionFailed)
}

// TestEncryption_TamperedNonce tests that tampered nonce is detected.
func TestEncryption_TamperedNonce(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	plaintext := []byte("sensitive data")
	ciphertext, err := em.Encrypt(plaintext)
	require.NoError(t, err)

	// Tamper with the nonce
	ciphertext[0] ^= 0xFF

	// Decryption should fail
	_, err = em.Decrypt(ciphertext)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDecryptionFailed)
}

// TestEncryption_TruncatedCiphertext tests that truncated ciphertext is detected.
func TestEncryption_TruncatedCiphertext(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	plaintext := []byte("sensitive data")
	ciphertext, err := em.Encrypt(plaintext)
	require.NoError(t, err)

	// Truncate ciphertext
	truncated := ciphertext[:len(ciphertext)-5]

	// Decryption should fail
	_, err = em.Decrypt(truncated)
	require.Error(t, err)
}

// TestEncryption_CiphertextTooShort tests handling of too-short ciphertext.
func TestEncryption_CiphertextTooShort(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	// Ciphertext shorter than nonce
	shortCiphertext := []byte("short")

	_, err := em.Decrypt(shortCiphertext)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidCiphertext)
}

// =============================================================================
// ENCRYPTION STATUS TESTS
// =============================================================================

// TestEncryption_NotInitialized tests behavior when encryption is not initialized.
func TestEncryption_NotInitialized(t *testing.T) {
	em := &EncryptionManager{
		usedNonces: make(map[string]bool),
	}

	// Encrypt should fail
	_, err := em.Encrypt([]byte("test"))
	require.ErrorIs(t, err, ErrNotInitialized)

	// Decrypt should fail
	_, err = em.Decrypt([]byte("test"))
	require.ErrorIs(t, err, ErrNotInitialized)
}

// TestEncryption_IsInitialized tests initialization check.
func TestEncryption_IsInitialized(t *testing.T) {
	em := &EncryptionManager{
		usedNonces: make(map[string]bool),
	}
	require.False(t, em.IsInitialized())

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")
	em = createTestEncryptionManager(t, keyPath)
	require.True(t, em.IsInitialized())
}

// TestEncryption_IsEncrypted tests the IsEncrypted helper function.
func TestEncryption_IsEncrypted(t *testing.T) {
	require.True(t, IsEncrypted("ENC:abc123"))
	require.True(t, IsEncrypted("ENC:"))
	require.False(t, IsEncrypted("abc123"))
	require.False(t, IsEncrypted(""))
	require.False(t, IsEncrypted("enc:abc123")) // Case sensitive
}

// TestEncryption_DecryptStringNonEncrypted tests that non-encrypted strings pass through.
func TestEncryption_DecryptStringNonEncrypted(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	// Non-encrypted string should pass through unchanged
	original := "plain text value"
	decrypted, err := em.DecryptString(original)
	require.NoError(t, err)
	require.Equal(t, original, decrypted)

	// Empty string
	decrypted, err = em.DecryptString("")
	require.NoError(t, err)
	require.Equal(t, "", decrypted)
}

// =============================================================================
// CONFIG FIELD ENCRYPTION TESTS
// =============================================================================

// TestEncryption_ConfigField tests config field encryption/decryption.
func TestEncryption_ConfigField(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	value := "sk-test-apikey-1234567890"

	// Encrypt
	encrypted, err := em.EncryptConfigField(value)
	require.NoError(t, err)
	require.True(t, IsEncrypted(encrypted))

	// Decrypt
	decrypted, err := em.DecryptConfigField(encrypted)
	require.NoError(t, err)
	require.Equal(t, value, decrypted)
}

// TestEncryption_ConfigFieldEmpty tests empty config field handling.
func TestEncryption_ConfigFieldEmpty(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	// Empty value should return empty
	encrypted, err := em.EncryptConfigField("")
	require.NoError(t, err)
	require.Equal(t, "", encrypted)

	decrypted, err := em.DecryptConfigField("")
	require.NoError(t, err)
	require.Equal(t, "", decrypted)
}

// TestEncryption_ConfigFieldAlreadyEncrypted tests that already encrypted values aren't re-encrypted.
func TestEncryption_ConfigFieldAlreadyEncrypted(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	value := "test value"
	encrypted, err := em.EncryptConfigField(value)
	require.NoError(t, err)

	// Encrypt again - should return same value
	encrypted2, err := em.EncryptConfigField(encrypted)
	require.NoError(t, err)
	require.Equal(t, encrypted, encrypted2, "Already encrypted value should not be re-encrypted")
}

// =============================================================================
// CONCURRENT ENCRYPTION TESTS
// =============================================================================

// TestEncryption_ConcurrentOperations tests thread safety of encryption operations.
func TestEncryption_ConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	var wg sync.WaitGroup

	// Concurrent encryptions
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			plaintext := []byte("data from goroutine")
			ciphertext, err := em.Encrypt(plaintext)
			require.NoError(t, err)

			decrypted, err := em.Decrypt(ciphertext)
			require.NoError(t, err)
			require.Equal(t, plaintext, decrypted)
		}(i)
	}

	// Concurrent decryptions
	testCiphertext, _ := em.Encrypt([]byte("shared data"))
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := em.Decrypt(testCiphertext)
			require.NoError(t, err)
		}()
	}

	wg.Wait()
}

// =============================================================================
// FILE ENCRYPTION TESTS
// =============================================================================

// TestEncryption_FileRoundTrip tests file encryption and decryption.
func TestEncryption_FileRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "master.key")

	em := createTestEncryptionManager(t, keyPath)

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	content := []byte("This is sensitive file content that needs encryption.")
	err := os.WriteFile(srcPath, content, 0600)
	require.NoError(t, err)

	// Encrypt to destination
	encPath := filepath.Join(tempDir, "encrypted.enc")
	err = em.EncryptFile(srcPath, encPath)
	require.NoError(t, err)

	// Verify encrypted file exists and is different
	encContent, err := os.ReadFile(encPath)
	require.NoError(t, err)
	require.NotEqual(t, content, encContent)
	require.True(t, IsEncrypted(string(encContent)))

	// Decrypt to new destination
	decPath := filepath.Join(tempDir, "decrypted.txt")
	err = em.DecryptFile(encPath, decPath)
	require.NoError(t, err)

	// Verify decrypted content matches original
	decContent, err := os.ReadFile(decPath)
	require.NoError(t, err)
	require.Equal(t, content, decContent)
}

// =============================================================================
// ZEROBYTES SECURITY TESTS
// =============================================================================

// TestZeroBytes tests secure memory zeroing.
func TestZeroBytes(t *testing.T) {
	// Create a buffer with sensitive data
	sensitive := []byte("sensitive password data")
	original := make([]byte, len(sensitive))
	copy(original, sensitive)

	// Zero it
	ZeroBytes(sensitive)

	// Verify all bytes are zero
	for i, b := range sensitive {
		require.Equal(t, byte(0), b, "Byte at position %d not zeroed", i)
	}

	// Verify original was different
	allZero := true
	for _, b := range original {
		if b != 0 {
			allZero = false
			break
		}
	}
	require.False(t, allZero, "Original data should not be all zeros")
}

// TestZeroBytes_Empty tests zeroing empty slice.
func TestZeroBytes_Empty(t *testing.T) {
	// Should not panic on empty slice
	ZeroBytes([]byte{})
	ZeroBytes(nil)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// createTestEncryptionManager creates an initialized encryption manager for testing.
func createTestEncryptionManager(t *testing.T, keyPath string) *EncryptionManager {
	t.Helper()

	// Ensure directory exists
	err := os.MkdirAll(filepath.Dir(keyPath), 0700)
	require.NoError(t, err)

	// Generate and store a master key
	key, err := GenerateMasterKey()
	require.NoError(t, err)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)

	em := &EncryptionManager{
		keyPath:      keyPath,
		cipher:       gcm,
		nonceCounter: 0,
		usedNonces:   make(map[string]bool),
	}

	return em
}
