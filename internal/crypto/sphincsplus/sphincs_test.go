package sphincsplus

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestSLHDSAEndToEnd(t *testing.T) {
	if err := CheckSLHDSAAvailability(); err != nil {
		t.Skipf("Skipping SLH-DSA tests: %v", err)
	}

	// Random 32-byte message
	msg := make([]byte, 32)
	if _, err := rand.Read(msg); err != nil {
		t.Fatalf("rand: %v", err)
	}

	priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	pub := priv.Public().(*SLHDSA256sPublicKey)
	fmt.Printf("Public key: %s\n", hex.EncodeToString(pub.Bytes()))

	sig, err := priv.Sign(nil, msg, SLHDSA256sSignerOpts{})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	fmt.Printf("Signature: %s\n", hex.EncodeToString(sig))

	if err := pub.Verify(msg, sig); err != nil {
		t.Fatalf("verify failed: %v", err)
	}

	// Determinism check
	sig2, err := priv.Sign(nil, msg, SLHDSA256sSignerOpts{})
	if err != nil {
		t.Fatalf("sign2: %v", err)
	}
	if hex.EncodeToString(sig) != hex.EncodeToString(sig2) {
		t.Fatalf("determinism violated")
	}
}
