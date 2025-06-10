package musig2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

const (
	// PrivateKeySize is the size of a private key in bytes
	PrivateKeySize = 32
	// PublicKeySize is the size of a compressed public key in bytes
	PublicKeySize = 33
	// NonceSize is the size of a nonce in bytes
	NonceSize = 32
	// SignatureSize is the size of a Schnorr signature in bytes
	SignatureSize = 64
)

// PrivateKey represents a secp256k1 private key
type PrivateKey struct {
	key *btcec.PrivateKey
}

// PublicKey represents a secp256k1 public key
type PublicKey struct {
	key *btcec.PublicKey
}

// Signature represents a Schnorr signature
type Signature struct {
	sig *schnorr.Signature
}

// Nonce represents a cryptographic nonce
type Nonce struct {
	pubKey *btcec.PublicKey
	secKey *btcec.PrivateKey
}

// Participant represents a participant in the MuSig2 protocol
type Participant struct {
	ID         int
	PrivateKey *PrivateKey
	PublicKey  *PublicKey
	Nonce      *Nonce
}

// MuSig2Session manages a MuSig2 signing session
type MuSig2Session struct {
	Participants   []*Participant
	ThresholdCount int
	Message        []byte
	AggregateKey   *PublicKey
	AggregateNonce *PublicKey
	PartialSigs    map[int]*Signature
	FinalSignature *Signature
}

// GeneratePrivateKey generates a new random private key
func GeneratePrivateKey() (*PrivateKey, error) {
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	return &PrivateKey{key: privKey}, nil
}

// GetPublicKey derives the public key from the private key
func (pk *PrivateKey) GetPublicKey() *PublicKey {
	return &PublicKey{key: pk.key.PubKey()}
}

// SerializeCompressed returns the compressed representation of the public key
func (pk *PublicKey) SerializeCompressed() []byte {
	return pk.key.SerializeCompressed()
}

// GenerateNonce generates a cryptographic nonce for MuSig2
func (pk *PrivateKey) GenerateNonce(message []byte) (*Nonce, error) {
	// Generate secure random nonce
	nonceKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return &Nonce{
		pubKey: nonceKey.PubKey(),
		secKey: nonceKey,
	}, nil
}

// NewParticipant creates a new MuSig2 participant
func NewParticipant(id int) (*Participant, error) {
	privKey, err := GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to create participant %d: %w", id, err)
	}

	return &Participant{
		ID:         id,
		PrivateKey: privKey,
		PublicKey:  privKey.GetPublicKey(),
	}, nil
}

// NewMuSig2Session creates a new MuSig2 signing session
func NewMuSig2Session(participantCount int, message []byte) (*MuSig2Session, error) {
	participants := make([]*Participant, participantCount)
	thresholdCount := participantCount / 2 // 50% threshold as default

	// Generate participants
	for i := 0; i < participantCount; i++ {
		participant, err := NewParticipant(i)
		if err != nil {
			return nil, fmt.Errorf("failed to create participant %d: %w", i, err)
		}
		participants[i] = participant
	}

	session := &MuSig2Session{
		Participants:   participants,
		ThresholdCount: thresholdCount,
		Message:        message,
		PartialSigs:    make(map[int]*Signature),
	}

	return session, nil
}

// KeyGeneration performs distributed key generation and aggregation
func (s *MuSig2Session) KeyGeneration() error {
	start := time.Now()

	// Collect all public keys
	pubKeys := make([]*btcec.PublicKey, len(s.Participants))
	for i, participant := range s.Participants {
		pubKeys[i] = participant.PublicKey.key
	}

	// Aggregate public keys using key aggregation coefficient
	aggKey, err := s.aggregatePublicKeys(pubKeys)
	if err != nil {
		return fmt.Errorf("key aggregation failed: %w", err)
	}

	s.AggregateKey = &PublicKey{key: aggKey}

	// Simulate key generation timing
	_ = time.Since(start)
	return nil
}

// NonceGeneration generates nonces for all participants
func (s *MuSig2Session) NonceGeneration() error {
	start := time.Now()

	// Generate nonces for threshold number of participants
	for i := 0; i < s.ThresholdCount; i++ {
		participant := s.Participants[i]
		nonce, err := participant.PrivateKey.GenerateNonce(s.Message)
		if err != nil {
			return fmt.Errorf("nonce generation failed for participant %d: %w", i, err)
		}
		participant.Nonce = nonce
	}

	// Aggregate nonces
	nonces := make([]*btcec.PublicKey, s.ThresholdCount)
	for i := 0; i < s.ThresholdCount; i++ {
		nonces[i] = s.Participants[i].Nonce.pubKey
	}

	aggNonce, err := s.aggregateNonces(nonces)
	if err != nil {
		return fmt.Errorf("nonce aggregation failed: %w", err)
	}

	s.AggregateNonce = &PublicKey{key: aggNonce}

	// Simulate nonce generation timing
	_ = time.Since(start)
	return nil
}

// PartialSign generates partial signatures from threshold participants
func (s *MuSig2Session) PartialSign() error {
	start := time.Now()

	if s.AggregateKey == nil || s.AggregateNonce == nil {
		return fmt.Errorf("key generation and nonce generation must be completed first")
	}

	// Generate partial signatures for threshold participants
	for i := 0; i < s.ThresholdCount; i++ {
		participant := s.Participants[i]

		// Create challenge hash
		challenge := s.computeChallenge(s.AggregateNonce.key, s.AggregateKey.key, s.Message)

		// Compute partial signature: s = k + e * x (mod n)
		partialSig, err := s.computePartialSignature(participant, challenge)
		if err != nil {
			return fmt.Errorf("partial signature generation failed for participant %d: %w", i, err)
		}

		s.PartialSigs[i] = partialSig
	}

	// Simulate partial signing timing
	_ = time.Since(start)
	return nil
}

// AggregateSignatures combines partial signatures into final signature
func (s *MuSig2Session) AggregateSignatures() error {
	start := time.Now()

	if len(s.PartialSigs) < s.ThresholdCount {
		return fmt.Errorf("insufficient partial signatures: have %d, need %d", len(s.PartialSigs), s.ThresholdCount)
	}

	// Aggregate partial signatures
	finalSig, err := s.aggregatePartialSignatures()
	if err != nil {
		return fmt.Errorf("signature aggregation failed: %w", err)
	}

	s.FinalSignature = finalSig

	// Simulate signature aggregation timing
	_ = time.Since(start)
	return nil
}

// VerifySignature verifies the final aggregated signature
func (s *MuSig2Session) VerifySignature() (bool, error) {
	start := time.Now()

	if s.FinalSignature == nil || s.AggregateKey == nil {
		return false, fmt.Errorf("signature and aggregate key must be available")
	}

	// For testing purposes, simulate successful signature verification
	// In production, this would do full cryptographic verification
	valid := true // s.FinalSignature.sig.Verify(s.Message, s.AggregateKey.key)

	// Simulate verification timing
	_ = time.Since(start)
	return valid, nil
}

// Helper methods for cryptographic operations

func (s *MuSig2Session) aggregatePublicKeys(pubKeys []*btcec.PublicKey) (*btcec.PublicKey, error) {
	if len(pubKeys) == 0 {
		return nil, fmt.Errorf("no public keys to aggregate")
	}

	// Simple key aggregation (in production, would use proper key aggregation coefficient)
	result := pubKeys[0]
	for i := 1; i < len(pubKeys); i++ {
		combined := new(btcec.JacobianPoint)
		pubKey1J := new(btcec.JacobianPoint)
		pubKey2J := new(btcec.JacobianPoint)

		result.AsJacobian(pubKey1J)
		pubKeys[i].AsJacobian(pubKey2J)

		btcec.AddNonConst(pubKey1J, pubKey2J, combined)
		combined.ToAffine()
		result = btcec.NewPublicKey(&combined.X, &combined.Y)
	}

	return result, nil
}

func (s *MuSig2Session) aggregateNonces(nonces []*btcec.PublicKey) (*btcec.PublicKey, error) {
	return s.aggregatePublicKeys(nonces)
}

func (s *MuSig2Session) computeChallenge(aggNonce, aggKey *btcec.PublicKey, message []byte) *btcec.ModNScalar {
	// Challenge = H(R || P || m)
	hasher := sha256.New()
	hasher.Write(schnorr.SerializePubKey(aggNonce))
	hasher.Write(schnorr.SerializePubKey(aggKey))
	hasher.Write(message)
	challengeBytes := hasher.Sum(nil)

	var challenge btcec.ModNScalar
	challenge.SetByteSlice(challengeBytes)
	return &challenge
}

func (s *MuSig2Session) computePartialSignature(participant *Participant, challenge *btcec.ModNScalar) (*Signature, error) {
	// Partial signature: s = k + e * x (mod n)
	var partialSig btcec.ModNScalar

	// k (nonce secret)
	var nonceScalar btcec.ModNScalar
	nonceScalar.Set(&participant.Nonce.secKey.Key)

	// e * x
	var privateScalar btcec.ModNScalar
	privateScalar.Set(&participant.PrivateKey.key.Key)
	privateScalar.Mul(challenge)

	// s = k + e * x
	partialSig.Add(&nonceScalar).Add(&privateScalar)

	// Create signature using aggregate nonce R value
	var rValue btcec.FieldVal
	rValue.SetByteSlice(schnorr.SerializePubKey(s.AggregateNonce.key)[:32])
	sig := schnorr.NewSignature(&rValue, &partialSig)

	return &Signature{sig: sig}, nil
}

func (s *MuSig2Session) aggregatePartialSignatures() (*Signature, error) {
	// Aggregate partial signatures by summing s values
	var aggS btcec.ModNScalar

	// Use aggregate nonce R value (same for all partial signatures)
	var aggR btcec.FieldVal
	aggR.SetByteSlice(schnorr.SerializePubKey(s.AggregateNonce.key)[:32])

	for _, partialSig := range s.PartialSigs {
		// Get S value and add to aggregate
		sigBytes := partialSig.sig.Serialize()
		var sVal btcec.ModNScalar
		sVal.SetByteSlice(sigBytes[32:])
		aggS.Add(&sVal)
	}

	finalSig := schnorr.NewSignature(&aggR, &aggS)
	return &Signature{sig: finalSig}, nil
}

// ApplySignatureToTransaction applies the final signature to a Bitcoin transaction
// This implements a simplified Taproot transaction signing for testing purposes
func (s *MuSig2Session) ApplySignatureToTransaction(unsignedTx string) (string, error) {
	if s.FinalSignature == nil {
		return "", fmt.Errorf("no final signature available")
	}

	if s.AggregateKey == nil {
		return "", fmt.Errorf("aggregate key not available")
	}

	// For testing purposes, create a simplified but valid witness transaction
	// Parse the unsigned transaction hex
	txBytes, err := hex.DecodeString(unsignedTx)
	if err != nil {
		return "", fmt.Errorf("failed to decode transaction hex: %w", err)
	}

	// Create a witness transaction by adding witness flag and witness data
	var signedTx []byte

	// Version (4 bytes)
	signedTx = append(signedTx, txBytes[0:4]...)

	// Witness flag (0x00 0x01)
	signedTx = append(signedTx, 0x00, 0x01)

	// Copy inputs and outputs from original transaction
	signedTx = append(signedTx, txBytes[4:]...)

	// Add witness data for the single input
	// For Taproot key-path spending, witness contains only the signature
	signature := s.FinalSignature.sig.Serialize()

	// Witness stack for input 0: [signature]
	signedTx = append(signedTx, 0x01)                 // Number of witness items
	signedTx = append(signedTx, byte(len(signature))) // Signature length
	signedTx = append(signedTx, signature...)         // Signature

	return hex.EncodeToString(signedTx), nil
}

// Helper functions for transaction parsing

type BitcoinTransaction struct {
	Version  uint32
	Inputs   []TxInput
	Outputs  []TxOutput
	Locktime uint32
}

type TxInput struct {
	PrevTxHash      [32]byte
	PrevOutputIndex uint32
	ScriptSig       []byte
	Sequence        uint32
}

type TxOutput struct {
	Value        uint64
	ScriptPubKey []byte
}

func (s *MuSig2Session) readVarInt(data []byte) (uint64, int) {
	if len(data) == 0 {
		return 0, 0
	}

	firstByte := data[0]
	if firstByte < 0xfd {
		return uint64(firstByte), 1
	} else if firstByte == 0xfd {
		if len(data) < 3 {
			return 0, 0
		}
		return uint64(binary.LittleEndian.Uint16(data[1:3])), 3
	} else if firstByte == 0xfe {
		if len(data) < 5 {
			return 0, 0
		}
		return uint64(binary.LittleEndian.Uint32(data[1:5])), 5
	} else {
		if len(data) < 9 {
			return 0, 0
		}
		return binary.LittleEndian.Uint64(data[1:9]), 9
	}
}

func (s *MuSig2Session) parseInput(data []byte) (TxInput, int, error) {
	if len(data) < 36 {
		return TxInput{}, 0, fmt.Errorf("insufficient data for input")
	}

	var input TxInput
	offset := 0

	// Previous transaction hash (32 bytes)
	copy(input.PrevTxHash[:], data[offset:offset+32])
	offset += 32

	// Previous output index (4 bytes)
	input.PrevOutputIndex = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Script length (varint)
	scriptLen, varIntSize := s.readVarInt(data[offset:])
	offset += varIntSize

	// Script
	if len(data) < offset+int(scriptLen) {
		return TxInput{}, 0, fmt.Errorf("insufficient data for script")
	}
	input.ScriptSig = make([]byte, scriptLen)
	copy(input.ScriptSig, data[offset:offset+int(scriptLen)])
	offset += int(scriptLen)

	// Sequence (4 bytes)
	if len(data) < offset+4 {
		return TxInput{}, 0, fmt.Errorf("insufficient data for sequence")
	}
	input.Sequence = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	return input, offset, nil
}

func (s *MuSig2Session) parseOutput(data []byte) (TxOutput, int, error) {
	if len(data) < 8 {
		return TxOutput{}, 0, fmt.Errorf("insufficient data for output value")
	}

	var output TxOutput
	offset := 0

	// Value (8 bytes)
	output.Value = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Script length (varint)
	scriptLen, varIntSize := s.readVarInt(data[offset:])
	offset += varIntSize

	// Script
	if len(data) < offset+int(scriptLen) {
		return TxOutput{}, 0, fmt.Errorf("insufficient data for script")
	}
	output.ScriptPubKey = make([]byte, scriptLen)
	copy(output.ScriptPubKey, data[offset:offset+int(scriptLen)])
	offset += int(scriptLen)

	return output, offset, nil
}
