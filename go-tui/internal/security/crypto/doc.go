// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package crypto provides cryptographic operations for IL5 compliance.
//
// This package implements NIST 800-53 SC-* controls:
//   - SC-13: Cryptographic Protection
//   - SC-17: Public Key Infrastructure Certificates
//   - IA-7: Cryptographic Module Authentication
//
// # FIPS 140-2/3 Compliance
//
// This package documents and uses only FIPS-approved algorithms:
//   - AES-256-GCM for symmetric encryption
//   - SHA-256/384/512 for hashing
//   - HMAC-SHA-256 for message authentication
//   - ECDSA P-256/P-384 for signatures
//   - RSA-2048+ for signatures and key exchange
//   - PBKDF2-SHA-256 for key derivation
//
// # Encryption
//
// AES-256-GCM encryption is provided for data at rest:
//
//	manager, err := crypto.NewEncryptionManager(key)
//	if err != nil {
//	    return err
//	}
//	ciphertext, err := manager.Encrypt(plaintext)
//	plaintext, err := manager.Decrypt(ciphertext)
//
// # PKI Certificate Management
//
// PKI operations include certificate validation and pinning:
//
//	pki := crypto.NewPKIManager()
//	status, err := pki.ValidateCertificate("api.example.com")
//	pki.PinCertificate("api.example.com", fingerprint)
//
// # FIPS Mode
//
// FIPS mode can be enabled for stricter compliance:
//
//	crypto.SetFIPSMode(true)
//	status := crypto.GetCryptoStatus()
//	result := crypto.VerifyFIPSCompliance()
package crypto
