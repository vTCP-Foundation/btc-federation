//go:build cgo && openssl && amd64
// +build cgo,openssl,amd64

package sphincsplus

import (
	"crypto/rand"
	"runtime"
	"runtime/debug"
	"testing"
)

// TestSLHDSAMemoryLeak runs many key-gen/sign/verify cycles and checks that
// heap allocations do not grow unbounded (simple leak smoke-test). It is **not**
// a formal leak detector but catches obvious retention issues.
func TestSLHDSAMemoryLeak(t *testing.T) {
	if err := CheckSLHDSAAvailability(); err != nil {
		t.Skipf("Skipping leak test: %v", err)
	}

	const iterations = 200

	// Warm-up once so one-time allocations (e.g., provider load) are excluded.
	warmMsg := make([]byte, 32)
	rand.Read(warmMsg)
	pk, err := GenerateKey()
	if err != nil {
		t.Fatalf("warm-up keygen: %v", err)
	}
	defer pk.Close()
	sig, err := pk.Sign(nil, warmMsg, SLHDSA256sSignerOpts{})
	if err != nil {
		t.Fatalf("warm-up sign: %v", err)
	}
	if err := pk.Public().(*SLHDSA256sPublicKey).Verify(warmMsg, sig); err != nil {
		t.Fatalf("warm-up verify: %v", err)
	}

	// GC & capture baseline memory stats.
	runtime.GC()
	debug.FreeOSMemory()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for i := 0; i < iterations; i++ {
		msg := make([]byte, 64)
		if _, err := rand.Read(msg); err != nil {
			t.Fatalf("rand: %v", err)
		}
		priv, err := GenerateKey()
		if err != nil {
			t.Fatalf("keygen: %v", err)
		}
		sig, err := priv.Sign(nil, msg, SLHDSA256sSignerOpts{})
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		pub := priv.Public().(*SLHDSA256sPublicKey)
		if err := pub.Verify(msg, sig); err != nil {
			t.Fatalf("verify: %v", err)
		}
		priv.Close()
	}

	runtime.GC()
	debug.FreeOSMemory()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// Allow up to 20% increase or 8 MB absolute – accounts for allocator noise.
	const slack = 8 << 20 // 8 MB
	if after.Alloc > before.Alloc+before.Alloc/5+slack {
		t.Fatalf("possible memory leak: before=%d after=%d", before.Alloc, after.Alloc)
	}
}
