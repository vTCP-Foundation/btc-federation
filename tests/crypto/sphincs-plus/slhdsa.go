package sphincsplus

/*
#cgo LDFLAGS: -lssl -lcrypto
#include <openssl/evp.h>
#include <openssl/param_build.h>
#include <openssl/core_names.h>
#include <openssl/err.h>
#include <openssl/provider.h>
#include <openssl/params.h>
#include <stdlib.h>
#include <string.h>

// Debug function to get OpenSSL error string
char* get_openssl_error() {
    unsigned long err = ERR_get_error();
    if (err == 0) return NULL;
    return ERR_error_string(err, NULL);
}

// Initialize providers and test if SLH-DSA algorithm is available
int test_slh_dsa_available() {
    // Load the default provider explicitly
    if (!OSSL_PROVIDER_load(NULL, "default")) {
        return 0;
    }
    
    EVP_PKEY_CTX *ctx = EVP_PKEY_CTX_new_from_name(NULL, "SLH-DSA-SHA2-256s", NULL);
    if (ctx) {
        EVP_PKEY_CTX_free(ctx);
        return 1;
    }
    return 0;
}

// Get OpenSSL version
const char* get_openssl_version() {
    return OPENSSL_VERSION_TEXT;
}

// Free EVP_PKEY
void free_evp_pkey(EVP_PKEY *pkey) {
    if (pkey) {
        EVP_PKEY_free(pkey);
    }
}

// Debug function to test if a private key can be loaded
int debug_test_private_key(const unsigned char *priv_key, size_t priv_len) {
    EVP_PKEY *pkey = EVP_PKEY_new_raw_private_key_ex(NULL, "SLH-DSA-SHA2-256s", NULL, priv_key, priv_len);
    if (!pkey) {
        return 0;
    }
    EVP_PKEY_free(pkey);
    return 1;
}

// EVP-based SLH-DSA key generation that returns EVP_PKEY
EVP_PKEY* evp_slh_dsa_keygen_pkey(unsigned char **pub_key, size_t *pub_len) {
    EVP_PKEY_CTX *ctx = NULL;
    EVP_PKEY *pkey = NULL;
    
    // Create EVP context directly for SLH-DSA-SHA2-256s (specific parameter set)
    ctx = EVP_PKEY_CTX_new_from_name(NULL, "SLH-DSA-SHA2-256s", NULL);
    if (!ctx) {
        return NULL;
    }
    
    // Initialize key generation
    if (EVP_PKEY_keygen_init(ctx) <= 0) {
        goto cleanup;
    }
    
    // Generate key pair (no additional parameters needed for specific algorithm)
    if (EVP_PKEY_keygen(ctx, &pkey) <= 0) {
        goto cleanup;
    }
    
    // Extract raw public key for convenience
    if (EVP_PKEY_get_raw_public_key(pkey, NULL, pub_len) <= 0) {
        goto cleanup;
    }
    
    *pub_key = malloc(*pub_len);
    if (!*pub_key) {
        goto cleanup;
    }
    
    if (EVP_PKEY_get_raw_public_key(pkey, *pub_key, pub_len) <= 0) {
        free(*pub_key);
        *pub_key = NULL;
        goto cleanup;
    }
    
    // Success - return the pkey (don't free it)
    EVP_PKEY_CTX_free(ctx);
    return pkey;

cleanup:
    if (pkey) EVP_PKEY_free(pkey);
    if (ctx) EVP_PKEY_CTX_free(ctx);
    return NULL;
}

// EVP-based SLH-DSA signing using digest context (required for SLH-DSA)
int evp_slh_dsa_sign_with_pkey(EVP_PKEY *pkey, const unsigned char *msg, size_t msg_len,
                               unsigned char **sig, size_t *sig_len) {
    EVP_MD_CTX *md_ctx = NULL;
    int ret = 0;
    
    if (!pkey) {
        return 0;
    }
    
    // Create digest context for SLH-DSA (required even though no explicit hash)
    md_ctx = EVP_MD_CTX_new();
    if (!md_ctx) {
        return 0;
    }
    
    // Initialize for signing with NULL digest (SLH-DSA handles its own hashing)
    if (EVP_DigestSignInit(md_ctx, NULL, NULL, NULL, pkey) <= 0) {
        goto cleanup;
    }
    
    // Set deterministic parameter for SLH-DSA-SHA2-256s
    EVP_PKEY_CTX *pctx = EVP_MD_CTX_get_pkey_ctx(md_ctx);
    if (pctx) {
        // Set deterministic flag using proper OSSL_PARAM construction
        OSSL_PARAM params[2];
        int *deterministic_flag = malloc(sizeof(int));
        *deterministic_flag = 1;
        params[0] = OSSL_PARAM_construct_int("deterministic", deterministic_flag);
        params[1] = OSSL_PARAM_construct_end();
        
        if (EVP_PKEY_CTX_set_params(pctx, params) <= 0) {
            // Deterministic parameter setting failed - this is required
            free(deterministic_flag);
            goto cleanup;
        }
        
        free(deterministic_flag);
    }
    
    // Get signature length
    if (EVP_DigestSign(md_ctx, NULL, sig_len, msg, msg_len) <= 0) {
        goto cleanup;
    }
    
    // Allocate signature buffer
    *sig = malloc(*sig_len);
    if (!*sig) {
        goto cleanup;
    }
    
    // Create signature (deterministic for SLH-DSA-SHA2-256s)
    if (EVP_DigestSign(md_ctx, *sig, sig_len, msg, msg_len) <= 0) {
        free(*sig);
        *sig = NULL;
        goto cleanup;
    }
    
    // Success
    ret = 1;

cleanup:
    if (md_ctx) EVP_MD_CTX_free(md_ctx);
    return ret;
}

// EVP-based SLH-DSA verification using digest context (required for SLH-DSA)
int evp_slh_dsa_verify(const unsigned char *pub_key, size_t pub_len,
                       const unsigned char *msg, size_t msg_len,
                       const unsigned char *sig, size_t sig_len) {
    EVP_PKEY *pkey = NULL;
    EVP_MD_CTX *md_ctx = NULL;
    int ret = 0;
    
    // Create EVP_PKEY from raw public key using specific algorithm name
    pkey = EVP_PKEY_new_raw_public_key_ex(NULL, "SLH-DSA-SHA2-256s", NULL, pub_key, pub_len);
    if (!pkey) {
        return 0;
    }
    
    // Create digest context for SLH-DSA verification
    md_ctx = EVP_MD_CTX_new();
    if (!md_ctx) {
        goto cleanup;
    }
    
    // Initialize for verification with NULL digest (SLH-DSA handles its own hashing)
    if (EVP_DigestVerifyInit(md_ctx, NULL, NULL, NULL, pkey) <= 0) {
        goto cleanup;
    }
    
    // Verify signature
    if (EVP_DigestVerify(md_ctx, sig, sig_len, msg, msg_len) == 1) {
        ret = 1;
    }

cleanup:
    if (md_ctx) EVP_MD_CTX_free(md_ctx);
    if (pkey) EVP_PKEY_free(pkey);
    return ret;
}
*/
import "C"

import (
	"crypto"
	"errors"
	"io"
	"runtime"
	"unsafe"
)

/*
// Forward declaration for EVP_PKEY
typedef struct evp_pkey_st EVP_PKEY;
*/

// SLHDSA256sPrivateKey implements crypto.Signer for SLH-DSA-SHA2-256s using EVP interface
type SLHDSA256sPrivateKey struct {
	pkey      unsafe.Pointer // EVP_PKEY*
	publicKey []byte
}

// SLHDSA256sPublicKey represents the public key
type SLHDSA256sPublicKey struct {
	key []byte
}

// CheckSLHDSAAvailability checks if SLH-DSA is available in OpenSSL
func CheckSLHDSAAvailability() error {
	available := C.test_slh_dsa_available()
	if available == 0 {
		version := C.GoString(C.get_openssl_version())
		return errors.New("SLH-DSA-SHA2-256s not available in OpenSSL version: " + version)
	}
	return nil
}

// GenerateKey generates a new SLH-DSA-SHA2-256s key pair using EVP interface
func GenerateKey() (*SLHDSA256sPrivateKey, error) {
	// First check if SLH-DSA is available
	if err := CheckSLHDSAAvailability(); err != nil {
		return nil, err
	}

	var pubKey *C.uchar
	var pubLen C.size_t

	// Generate key pair using EVP interface, keep EVP_PKEY intact
	pkey := C.evp_slh_dsa_keygen_pkey(&pubKey, &pubLen)
	if pkey == nil {
		errStr := C.get_openssl_error()
		if errStr != nil {
			return nil, errors.New("failed to generate SLH-DSA-SHA2-256s key pair: " + C.GoString(errStr))
		}
		return nil, errors.New("failed to generate SLH-DSA-SHA2-256s key pair via EVP interface")
	}

	// Copy public key to Go memory
	pubKeyBytes := C.GoBytes(unsafe.Pointer(pubKey), C.int(pubLen))

	// Free C memory for public key (but keep pkey)
	C.free(unsafe.Pointer(pubKey))

	privKeyObj := &SLHDSA256sPrivateKey{
		pkey:      unsafe.Pointer(pkey),
		publicKey: pubKeyBytes,
	}
	
	// Set finalizer to clean up EVP_PKEY
	runtime.SetFinalizer(privKeyObj, (*SLHDSA256sPrivateKey).finalize)
	
	return privKeyObj, nil
}

// finalize cleans up the EVP_PKEY
func (priv *SLHDSA256sPrivateKey) finalize() {
	if priv.pkey != nil {
		C.free_evp_pkey((*C.EVP_PKEY)(priv.pkey))
		priv.pkey = nil
	}
}

// Public returns the public key corresponding to the private key (crypto.Signer interface)
func (priv *SLHDSA256sPrivateKey) Public() crypto.PublicKey {
	return &SLHDSA256sPublicKey{key: priv.publicKey}
}

// Sign signs digest with the private key using EVP SLH-DSA-SHA2-256s (deterministic)
func (priv *SLHDSA256sPrivateKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	if priv.pkey == nil {
		return nil, errors.New("private key is nil")
	}

	var sig *C.uchar
	var sigLen C.size_t

	// Convert Go slice to C parameters
	msgPtr := (*C.uchar)(C.CBytes(digest))
	defer C.free(unsafe.Pointer(msgPtr))

	// Sign using the existing EVP_PKEY
	result := C.evp_slh_dsa_sign_with_pkey(
		(*C.EVP_PKEY)(priv.pkey),
		msgPtr, C.size_t(len(digest)),
		&sig, &sigLen,
	)

	if result == 0 {
		errStr := C.get_openssl_error()
		if errStr != nil {
			return nil, errors.New("failed to sign message with EVP SLH-DSA-SHA2-256s: " + C.GoString(errStr))
		}
		return nil, errors.New("failed to sign message with EVP SLH-DSA-SHA2-256s")
	}

	// Copy signature to Go memory
	signature := C.GoBytes(unsafe.Pointer(sig), C.int(sigLen))

	// Free C memory
	C.free(unsafe.Pointer(sig))

	return signature, nil
}

// PublicKeyBytes returns the raw public key bytes
func (pub *SLHDSA256sPublicKey) Bytes() []byte {
	return pub.key
}

// Verify verifies a signature against a message using the EVP public key
func (pub *SLHDSA256sPublicKey) Verify(message, signature []byte) error {
	msgPtr := (*C.uchar)(C.CBytes(message))
	defer C.free(unsafe.Pointer(msgPtr))

	sigPtr := (*C.uchar)(C.CBytes(signature))
	defer C.free(unsafe.Pointer(sigPtr))

	pubKeyPtr := (*C.uchar)(C.CBytes(pub.key))
	defer C.free(unsafe.Pointer(pubKeyPtr))

	result := C.evp_slh_dsa_verify(
		pubKeyPtr, C.size_t(len(pub.key)),
		msgPtr, C.size_t(len(message)),
		sigPtr, C.size_t(len(signature)),
	)

	if result == 0 {
		return errors.New("EVP SLH-DSA-SHA2-256s signature verification failed")
	}

	return nil
}

// SignerOpts for SLH-DSA (no pre-hashing, sign message directly)
type SLHDSA256sSignerOpts struct{}

func (opts SLHDSA256sSignerOpts) HashFunc() crypto.Hash {
	// Return crypto.Hash(0) to indicate direct message signing (no pre-hashing)
	return crypto.Hash(0)
}