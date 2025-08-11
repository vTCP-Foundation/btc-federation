package sphincsplus

import (
	"crypto"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestSLHDSAAvailability(t *testing.T) {
	err := CheckSLHDSAAvailability()
	if err != nil {
		t.Skipf("Skipping SLH-DSA tests: %v", err)
	}
	fmt.Println("✓ SLH-DSA-SHA2-256s is available")
}

func TestSPHINCSPlusSignatureFlow(t *testing.T) {
	// Generate random message for signing
	message := make([]byte, 32)
	if _, err := rand.Read(message); err != nil {
		t.Fatalf("Failed to generate random message: %v", err)
	}
	
	fmt.Printf("Message to sign: %s\n", hex.EncodeToString(message))

	// Generate SLH-DSA-SHA2-256s key pair using EVP interface
	privKey, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate SLH-DSA-SHA2-256s key pair: %v", err)
	}

	// Get public key
	pubKey := privKey.Public().(*SLHDSA256sPublicKey)
	fmt.Printf("Public Key: %s\n", hex.EncodeToString(pubKey.Bytes()))

	// Sign the message using crypto.Signer interface (deterministic SLH-DSA-SHA2-256s)
	signature, err := privKey.Sign(nil, message, SLHDSA256sSignerOpts{})
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	// Output signature in hex format to stdout as requested
	signatureHex := hex.EncodeToString(signature)
	fmt.Printf("Signature (hex): %s\n", signatureHex)

	// Verify the signature using the public key
	err = pubKey.Verify(message, signature)
	if err != nil {
		t.Fatalf("Signature verification failed: %v", err)
	}

	fmt.Println("✓ Signature verification successful")

	// Test deterministic property - SLH-DSA-SHA2-256s should produce identical signatures
	signature2, err := privKey.Sign(nil, message, SLHDSA256sSignerOpts{})
	if err != nil {
		t.Fatalf("Failed to sign message second time: %v", err)
	}

	if hex.EncodeToString(signature) != hex.EncodeToString(signature2) {
		t.Errorf("Signatures differ - SLH-DSA-SHA2-256s should be deterministic")
	} else {
		fmt.Println("✓ Deterministic signing confirmed - identical signatures produced")
	}

	// Test with different message to ensure signature changes
	differentMessage := make([]byte, 32)
	if _, err := rand.Read(differentMessage); err != nil {
		t.Fatalf("Failed to generate different message: %v", err)
	}

	differentSignature, err := privKey.Sign(nil, differentMessage, SLHDSA256sSignerOpts{})
	if err != nil {
		t.Fatalf("Failed to sign different message: %v", err)
	}

	if hex.EncodeToString(signature) == hex.EncodeToString(differentSignature) {
		t.Fatalf("Different messages produced identical signatures - this should not happen")
	}

	// Verify the different signature fails with original message
	err = pubKey.Verify(message, differentSignature)
	if err == nil {
		t.Fatalf("Signature for different message incorrectly verified with original message")
	}

	fmt.Println("✓ All SLH-DSA-SHA2-256s signature flow tests passed")
}

func TestCryptoSignerInterface(t *testing.T) {
	// Test that our implementation properly satisfies crypto.Signer interface
	privKey, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Verify it implements crypto.Signer
	var signer crypto.Signer = privKey
	if signer == nil {
		t.Fatal("SLHDSA256sPrivateKey does not implement crypto.Signer interface")
	}

	// Test Public() method returns correct type
	pub := privKey.Public()
	if _, ok := pub.(*SLHDSA256sPublicKey); !ok {
		t.Fatal("Public() method does not return *SLHDSA256sPublicKey")
	}

	fmt.Println("✓ crypto.Signer interface compliance verified")
}