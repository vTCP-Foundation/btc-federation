package sphincsplus

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
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
	defer priv.Close()

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

// TestConcurrentKeyUsage demonstrates safe concurrent usage with different keys.
// This test shows that different key instances can be used safely from different goroutines.
func TestConcurrentKeyUsage(t *testing.T) {
	if err := CheckSLHDSAAvailability(); err != nil {
		t.Skipf("Skipping SLH-DSA tests: %v", err)
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan error, numGoroutines)

	// Each goroutine uses its own key instance - this is safe
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine creates its own key
			priv, err := GenerateKey()
			if err != nil {
				results <- fmt.Errorf("goroutine %d keygen: %v", id, err)
				return
			}
			defer priv.Close()

			msg := []byte(fmt.Sprintf("message from goroutine %d", id))

			// Sign with this key
			sig, err := priv.Sign(nil, msg, SLHDSA256sSignerOpts{})
			if err != nil {
				results <- fmt.Errorf("goroutine %d sign: %v", id, err)
				return
			}

			// Verify with public key
			pub := priv.Public().(*SLHDSA256sPublicKey)
			if err := pub.Verify(msg, sig); err != nil {
				results <- fmt.Errorf("goroutine %d verify: %v", id, err)
				return
			}

			results <- nil
		}(i)
	}

	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Error(err)
		}
	}
}

// TestCloseAndUseAfterClose tests proper resource cleanup and use-after-close protection.
func TestCloseAndUseAfterClose(t *testing.T) {
	if err := CheckSLHDSAAvailability(); err != nil {
		t.Skipf("Skipping SLH-DSA tests: %v", err)
	}

	priv, err := GenerateKey()
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	defer priv.Close()

	// Verify key works before closing
	msg := []byte("test message")
	sig, err := priv.Sign(nil, msg, SLHDSA256sSignerOpts{})
	if err != nil {
		t.Fatalf("sign before close: %v", err)
	}

	pub := priv.Public().(*SLHDSA256sPublicKey)
	if err := pub.Verify(msg, sig); err != nil {
		t.Fatalf("verify before close: %v", err)
	}

	// Close the key
	if err := priv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Multiple closes should be safe
	if err := priv.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}

	// Operations after close should fail gracefully
	_, err = priv.Sign(nil, msg, SLHDSA256sSignerOpts{})
	if err == nil {
		t.Fatal("expected sign to fail after close")
	}
	if err.Error() != "private key has been closed or cleared" {
		t.Fatalf("unexpected error after close: %v", err)
	}

	// Public() should still work but return empty key
	pubAfterClose := priv.Public().(*SLHDSA256sPublicKey)
	if pubAfterClose.key != nil {
		t.Fatal("expected public key to be nil after close")
	}
}
