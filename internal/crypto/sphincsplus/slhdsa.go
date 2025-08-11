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
	"unsafe"
)

// SLHDSA256sPrivateKey satisfies crypto.Signer for SLH-DSA-SHA2-256s.
type SLHDSA256sPrivateKey struct {
	pkey      unsafe.Pointer // *C.EVP_PKEY (owned)
	publicKey []byte         // raw bytes
}

type SLHDSA256sPublicKey struct {
	key []byte
}

func init() {
	// Best effort – ignore failure; availability is checked explicitly later.
	C.ensure_default_provider()
}

// CheckSLHDSAAvailability returns an error if the algorithm is unavailable.
func CheckSLHDSAAvailability() error {
	if C.test_slh_dsa_available() == 0 {
		version := C.GoString(C.get_openssl_version())
		return errors.New("SLH-DSA-SHA2-256s not available (OpenSSL " + version + ")")
	}
	return nil
}

// GenerateKey creates a new deterministic SLH-DSA keypair.
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

	pubBytes := C.GoBytes(unsafe.Pointer(pub), C.int(pubLen))
	C.secure_free(unsafe.Pointer(pub), pubLen)

	pk := &SLHDSA256sPrivateKey{
		pkey:      unsafe.Pointer(p),
		publicKey: pubBytes,
	}
	runtime.SetFinalizer(pk, (*SLHDSA256sPrivateKey).finalize)
	return pk, nil
}

func (pk *SLHDSA256sPrivateKey) finalize() {
	if pk.pkey != nil {
		C.free_evp_pkey((*C.EVP_PKEY)(pk.pkey))
		pk.pkey = nil
	}
}

// Public returns the corresponding public key.
func (pk *SLHDSA256sPrivateKey) Public() crypto.PublicKey {
	return &SLHDSA256sPublicKey{key: pk.publicKey}
}

// Sign deterministically signs the provided message (digest is the entire message).
func (pk *SLHDSA256sPrivateKey) Sign(_ io.Reader, digest []byte, _ crypto.SignerOpts) ([]byte, error) {
	if pk.pkey == nil {
		return nil, errors.New("private key has been cleared")
	}

	msgPtr := (*C.uchar)(C.CBytes(digest))
	defer C.secure_free(unsafe.Pointer(msgPtr), C.size_t(len(digest)))

	var sig *C.uchar
	var sigLen C.size_t

	ok := C.evp_slh_dsa_sign_with_pkey((*C.EVP_PKEY)(pk.pkey), msgPtr, C.size_t(len(digest)), &sig, &sigLen)
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
	msgPtr := (*C.uchar)(C.CBytes(message))
	defer C.secure_free(unsafe.Pointer(msgPtr), C.size_t(len(message)))

	sigPtr := (*C.uchar)(C.CBytes(signature))
	defer C.secure_free(unsafe.Pointer(sigPtr), C.size_t(len(signature)))

	pubPtr := (*C.uchar)(C.CBytes(pub.key))
	defer C.free(unsafe.Pointer(pubPtr)) // public keys are not sensitive

	ok := C.evp_slh_dsa_verify(pubPtr, C.size_t(len(pub.key)), msgPtr, C.size_t(len(message)), sigPtr, C.size_t(len(signature)))
	if ok == 0 {
		return errors.New("signature verification failed")
	}
	return nil
}

// SLHDSA256sSignerOpts signals that we sign the message directly (no hash).
type SLHDSA256sSignerOpts struct{}

func (SLHDSA256sSignerOpts) HashFunc() crypto.Hash { return crypto.Hash(0) }
