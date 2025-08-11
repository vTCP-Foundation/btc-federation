//go:build cgo && openssl && amd64
// +build cgo,openssl,amd64

// Package sphincsplus provides SLH-DSA-SHA2-256s implementation via OpenSSL.
//
// Thread Safety: This package is NOT thread-safe. The global OpenSSL provider
// state and key instances require external synchronization for concurrent use.
// It is safe to use different key instances concurrently, but the same key
// instance must not be used from multiple goroutines without synchronization.
package sphincsplus

/*
#cgo LDFLAGS: -lssl -lcrypto
#include <openssl/evp.h>
#include <openssl/param_build.h>
#include <openssl/core_names.h>
#include <openssl/err.h>
#include <openssl/provider.h>
#include <openssl/crypto.h>
#include <openssl/params.h>
#include <stdlib.h>
#include <string.h>

// Additional C stdlib functions we need
void* malloc(size_t size);
void* memcpy(void *dest, const void *src, size_t n);

static OSSL_PROVIDER *default_provider = NULL;

// ensure_default_provider loads the "default" provider exactly once and returns
// 1 on success, 0 on failure. Re-invocation is safe and cheap.
int ensure_default_provider() {
    if (default_provider == NULL) {
        default_provider = OSSL_PROVIDER_load(NULL, "default");
    }
    return default_provider != NULL;
}

void unload_default_provider() {
    if (default_provider != NULL) {
        OSSL_PROVIDER_unload(default_provider);
        default_provider = NULL;
    }
}

// Helper to obtain the most recent OpenSSL error string or NULL if none.
char* get_openssl_error() {
    unsigned long err = ERR_get_error();
    if (err == 0) return NULL;
    return ERR_error_string(err, NULL);
}

// Securely wipes a memory region before freeing.
void secure_free(void *ptr, size_t len) {
    if (ptr && len > 0) {
        OPENSSL_cleanse(ptr, len);
        free(ptr);
    }
}

// free_evp_pkey securely frees an EVP_PKEY (including private material).
void free_evp_pkey(EVP_PKEY *pkey) {
    if (pkey) {
        EVP_PKEY_free(pkey);
    }
}

// Return OpenSSL version string
const char* get_openssl_version() {
    return OPENSSL_VERSION_TEXT;
}

// Test if SLH-DSA-SHA2-256s algorithm is available.
int test_slh_dsa_available() {
    if (!ensure_default_provider()) {
        return 0;
    }
    EVP_PKEY_CTX *ctx = EVP_PKEY_CTX_new_from_name(NULL, "SLH-DSA-SHA2-256s", NULL);
    if (ctx) {
        EVP_PKEY_CTX_free(ctx);
        return 1;
    }
    return 0;
}

// Key-generation: returns an EVP_PKEY* and raw public key via out-params.
EVP_PKEY* evp_slh_dsa_keygen_pkey(unsigned char **pub_key, size_t *pub_len) {
    EVP_PKEY_CTX *ctx = NULL;
    EVP_PKEY     *pkey = NULL;

    if (!ensure_default_provider()) {
        return NULL;
    }

    ctx = EVP_PKEY_CTX_new_from_name(NULL, "SLH-DSA-SHA2-256s", NULL);
    if (!ctx) goto cleanup;

    if (EVP_PKEY_keygen_init(ctx) <= 0) goto cleanup;
    if (EVP_PKEY_keygen(ctx, &pkey) <= 0)        goto cleanup;

    if (EVP_PKEY_get_raw_public_key(pkey, NULL, pub_len) <= 0) goto cleanup;

    *pub_key = malloc(*pub_len);
    if (!*pub_key) goto cleanup;

    if (EVP_PKEY_get_raw_public_key(pkey, *pub_key, pub_len) <= 0) {
        secure_free(*pub_key, *pub_len);
        *pub_key = NULL;
        goto cleanup;
    }

    EVP_PKEY_CTX_free(ctx);
    return pkey;

cleanup:
    if (pkey) EVP_PKEY_free(pkey);
    if (ctx)  EVP_PKEY_CTX_free(ctx);
    return NULL;
}

// Sign a message deterministically with SLH-DSA-SHA2-256s.
int evp_slh_dsa_sign_with_pkey(EVP_PKEY *pkey, const unsigned char *msg, size_t msg_len,
                               unsigned char **sig, size_t *sig_len) {
    EVP_MD_CTX *md_ctx = NULL;
    int ret = 0;

    if (!pkey) return 0;

    md_ctx = EVP_MD_CTX_new();
    if (!md_ctx) return 0;

    if (EVP_DigestSignInit(md_ctx, NULL, NULL, NULL, pkey) <= 0) goto cleanup;

    int deterministic_flag = 1; // stack-allocated to avoid UAF risk
    OSSL_PARAM params[2];
    params[0] = OSSL_PARAM_construct_int("deterministic", &deterministic_flag);
    params[1] = OSSL_PARAM_construct_end();

    EVP_PKEY_CTX *pctx = EVP_MD_CTX_get_pkey_ctx(md_ctx);
    if (pctx) {
        if (EVP_PKEY_CTX_set_params(pctx, params) <= 0) goto cleanup;
    }

    if (EVP_DigestSign(md_ctx, NULL, sig_len, msg, msg_len) <= 0) goto cleanup;

    *sig = malloc(*sig_len);
    if (!*sig) goto cleanup;

    if (EVP_DigestSign(md_ctx, *sig, sig_len, msg, msg_len) <= 0) {
        secure_free(*sig, *sig_len);
        *sig = NULL;
        goto cleanup;
    }

    ret = 1;

cleanup:
    if (!ret && sig && *sig) {
        secure_free(*sig, *sig_len);
        *sig = NULL;
    }
    if (md_ctx) EVP_MD_CTX_free(md_ctx);
    return ret;
}

// Verify a signature.
int evp_slh_dsa_verify(const unsigned char *pub_key, size_t pub_len,
                       const unsigned char *msg, size_t msg_len,
                       const unsigned char *sig, size_t sig_len) {
    EVP_PKEY   *pkey   = NULL;
    EVP_MD_CTX *md_ctx = NULL;
    int ret = 0;

    if (!ensure_default_provider()) {
        return 0;
    }

    pkey = EVP_PKEY_new_raw_public_key_ex(NULL, "SLH-DSA-SHA2-256s", NULL, pub_key, pub_len);
    if (!pkey) goto cleanup;

    md_ctx = EVP_MD_CTX_new();
    if (!md_ctx) goto cleanup;

    if (EVP_DigestVerifyInit(md_ctx, NULL, NULL, NULL, pkey) <= 0) goto cleanup;

    if (EVP_DigestVerify(md_ctx, sig, sig_len, msg, msg_len) == 1) ret = 1;

cleanup:
    if (md_ctx) EVP_MD_CTX_free(md_ctx);
    if (pkey)  EVP_PKEY_free(pkey); // public-key only
    return ret;
}
*/
import "C"

import (
	"crypto"
	"errors"
	"io"
	"runtime"
	"sync"
	"unsafe"
)

// SLHDSA256sPrivateKey satisfies crypto.Signer for SLH-DSA-SHA2-256s.
// This implementation is NOT thread-safe. Callers must provide external synchronization
// for concurrent access to the same key instance.
//
// Implements crypto.Signer and io.Closer for deterministic cleanup.
type SLHDSA256sPrivateKey struct {
	mu        sync.RWMutex
	pkey      *C.EVP_PKEY // owned C resource
	publicKey []byte      // raw bytes
	closed    bool        // tracks if key has been closed
}

type SLHDSA256sPublicKey struct {
	key []byte
}

var (
	providerInitOnce sync.Once
	providerCleanup  func()
	initErr          error
)

const (
	PublicKeySize    = 64    // bytes for SLH-DSA-SHA2-256s public key (spec)
	MaxSignatureSize = 49856 // exact specification size for SLH-DSA-SHA2-256s signature
)

func init() {
	providerInitOnce.Do(func() {
		if C.ensure_default_provider() == 0 {
			initErr = errors.New("openssl default provider could not be loaded")
			return
		}
		// Provider loaded successfully, set up cleanup
		providerCleanup = func() {
			C.unload_default_provider()
		}
	})
}

// Shutdown cleans up global OpenSSL provider resources.
// This is optional and typically called during application shutdown.
// After calling Shutdown, new key operations may fail.
func Shutdown() {
	if providerCleanup != nil {
		providerCleanup()
		providerCleanup = nil
	}
}

// CheckSLHDSAAvailability returns an error if the algorithm is unavailable.
func CheckSLHDSAAvailability() error {
	if initErr != nil {
		return initErr
	}
	if C.test_slh_dsa_available() == 0 {
		version := C.GoString(C.get_openssl_version())
		return errors.New("SLH-DSA-SHA2-256s not available (OpenSSL " + version + ")")
	}
	return nil
}

// GenerateKey creates a new deterministic SLH-DSA keypair.
// The returned key is NOT thread-safe for concurrent operations.
func GenerateKey() (*SLHDSA256sPrivateKey, error) {
	if err := CheckSLHDSAAvailability(); err != nil {
		return nil, err
	}

	var pub *C.uchar
	var pubLen C.size_t

	p := C.evp_slh_dsa_keygen_pkey(&pub, &pubLen)
	if p == nil {
		errStr := C.get_openssl_error()
		if errStr != nil {
			return nil, errors.New(C.GoString(errStr))
		}
		return nil, errors.New("key generation failed")
	}
	// Validate public key length
	if pubLen != C.size_t(PublicKeySize) || pubLen == 0 {
		if p != nil {
			C.free_evp_pkey(p)
		}
		if pub != nil {
			C.secure_free(unsafe.Pointer(pub), pubLen)
		}
		return nil, errors.New("unexpected public key length")
	}

	pubBytes := C.GoBytes(unsafe.Pointer(pub), C.int(pubLen))
	C.secure_free(unsafe.Pointer(pub), pubLen)

	pk := &SLHDSA256sPrivateKey{
		pkey:      p,
		publicKey: pubBytes,
	}
	runtime.SetFinalizer(pk, (*SLHDSA256sPrivateKey).finalize)
	return pk, nil
}

func (pk *SLHDSA256sPrivateKey) finalize() {
	// Finalizer calls Close but doesn't return errors
	_ = pk.Close()
}

// Close securely destroys the private key material and frees resources.
// After calling Close, the key cannot be used for signing operations.
// It is safe to call Close multiple times.
func (pk *SLHDSA256sPrivateKey) Close() error {
	pk.mu.Lock()
	defer pk.mu.Unlock()

	if pk.closed {
		return nil // already closed
	}

	if pk.pkey != nil {
		C.free_evp_pkey(pk.pkey)
		pk.pkey = nil
	}

	// Clear public key copy as well for consistency
	if pk.publicKey != nil {
		for i := range pk.publicKey {
			pk.publicKey[i] = 0
		}
		pk.publicKey = nil
	}

	pk.closed = true
	runtime.SetFinalizer(pk, nil) // Remove finalizer since we're cleaned up
	return nil
}

// Public returns the corresponding public key.
func (pk *SLHDSA256sPrivateKey) Public() crypto.PublicKey {
	pk.mu.RLock()
	defer pk.mu.RUnlock()

	if pk.closed {
		// Return a public key with empty bytes - this is safe since public keys are not secret
		return &SLHDSA256sPublicKey{key: nil}
	}

	// Make a defensive copy
	pubCopy := make([]byte, len(pk.publicKey))
	copy(pubCopy, pk.publicKey)
	return &SLHDSA256sPublicKey{key: pubCopy}
}

// Sign deterministically signs the provided message (digest is the entire message).
func (pk *SLHDSA256sPrivateKey) Sign(_ io.Reader, digest []byte, _ crypto.SignerOpts) ([]byte, error) {
	pk.mu.RLock()
	defer pk.mu.RUnlock()

	if pk.closed || pk.pkey == nil {
		return nil, errors.New("private key has been closed or cleared")
	}

	// Securely allocate memory for digest
	if len(digest) == 0 {
		return nil, errors.New("empty digest")
	}
	msgPtr := (*C.uchar)(C.malloc(C.size_t(len(digest))))
	if msgPtr == nil {
		return nil, errors.New("memory allocation failed")
	}
	C.memcpy(unsafe.Pointer(msgPtr), unsafe.Pointer(&digest[0]), C.size_t(len(digest)))
	defer C.secure_free(unsafe.Pointer(msgPtr), C.size_t(len(digest)))

	var sig *C.uchar
	var sigLen C.size_t

	ok := C.evp_slh_dsa_sign_with_pkey(pk.pkey, msgPtr, C.size_t(len(digest)), &sig, &sigLen)
	if ok == 0 {
		errStr := C.get_openssl_error()
		if errStr != nil {
			return nil, errors.New(C.GoString(errStr))
		}
		return nil, errors.New("signing failed")
	}

	signature := C.GoBytes(unsafe.Pointer(sig), C.int(sigLen))
	C.secure_free(unsafe.Pointer(sig), sigLen)

	// Ensure pk remains live until after the C call completes.
	runtime.KeepAlive(pk)

	return signature, nil
}

// Bytes returns the raw public key.
func (pub *SLHDSA256sPublicKey) Bytes() []byte { return pub.key }

// Verify checks a signature against a message.
func (pub *SLHDSA256sPublicKey) Verify(message, signature []byte) error {
	if len(message) == 0 {
		return errors.New("empty message")
	}
	if len(signature) == 0 {
		return errors.New("empty signature")
	}
	if len(pub.key) != PublicKeySize {
		return errors.New("invalid public key length")
	}
	if len(signature) > MaxSignatureSize {
		return errors.New("signature too large")
	}
	// Allocate and securely handle message
	msgPtr := (*C.uchar)(C.malloc(C.size_t(len(message))))
	if msgPtr == nil {
		return errors.New("memory allocation failed")
	}
	C.memcpy(unsafe.Pointer(msgPtr), unsafe.Pointer(&message[0]), C.size_t(len(message)))
	defer C.secure_free(unsafe.Pointer(msgPtr), C.size_t(len(message)))

	// Allocate and securely handle signature
	sigPtr := (*C.uchar)(C.malloc(C.size_t(len(signature))))
	if sigPtr == nil {
		return errors.New("memory allocation failed")
	}
	C.memcpy(unsafe.Pointer(sigPtr), unsafe.Pointer(&signature[0]), C.size_t(len(signature)))
	defer C.secure_free(unsafe.Pointer(sigPtr), C.size_t(len(signature)))

	// Public keys don't need secure handling
	pubPtr := (*C.uchar)(C.CBytes(pub.key))
	defer C.free(unsafe.Pointer(pubPtr))

	ok := C.evp_slh_dsa_verify(pubPtr, C.size_t(len(pub.key)), msgPtr, C.size_t(len(message)), sigPtr, C.size_t(len(signature)))
	if ok == 0 {
		return errors.New("signature verification failed")
	}
	return nil
}

// SLHDSA256sSignerOpts signals that we sign the message directly (no hash).
type SLHDSA256sSignerOpts struct{}

func (SLHDSA256sSignerOpts) HashFunc() crypto.Hash { return crypto.Hash(0) }
