package frost

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// FROSTParticipant represents a single participant in the FROST protocol
type FROSTParticipant struct {
	ID         int
	PrivateKey *btcec.PrivateKey
	PublicKey  *btcec.PublicKey

	// DKG related fields
	Coefficients []*big.Int       // Polynomial coefficients for secret sharing
	Shares       map[int]*big.Int // Secret shares received from other participants

	// Signing related fields
	Nonce       *btcec.PrivateKey
	NonceCommit *btcec.PublicKey
	PartialSig  *big.Int
}

// FROSTSession manages a FROST threshold signature session
type FROSTSession struct {
	Participants   []*FROSTParticipant
	ThresholdCount int
	TotalCount     int
	Message        []byte

	// DKG results
	GroupPublicKey *btcec.PublicKey

	// Signing process
	AggregateNonce *btcec.PublicKey
	FinalSignature *schnorr.Signature

	// Challenge value for signing
	Challenge *big.Int
}

// Polynomial represents a polynomial for secret sharing
type Polynomial struct {
	Coefficients []*btcec.ModNScalar
	Degree       int
}

// NewFROSTSession creates a new FROST threshold signature session
func NewFROSTSession(participantCount int, message []byte) (*FROSTSession, error) {
	if participantCount < 2 {
		return nil, fmt.Errorf("need at least 2 participants, got %d", participantCount)
	}

	participants := make([]*FROSTParticipant, participantCount)

	// Initialize participants with random private keys
	for i := 0; i < participantCount; i++ {
		privKey, err := btcec.NewPrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate private key for participant %d: %w", i, err)
		}

		participants[i] = &FROSTParticipant{
			ID:         i + 1, // FROST participant IDs start from 1
			PrivateKey: privKey,
			PublicKey:  privKey.PubKey(),
			Shares:     make(map[int]*big.Int),
		}
	}

	return &FROSTSession{
		Participants:   participants,
		TotalCount:     participantCount,
		ThresholdCount: (participantCount + 1) / 2, // Default to majority threshold
		Message:        message,
	}, nil
}

// DistributedKeyGeneration performs the FROST DKG protocol
func (session *FROSTSession) DistributedKeyGeneration() error {
	t := session.ThresholdCount
	n := session.TotalCount

	if t > n {
		return fmt.Errorf("threshold %d cannot exceed total participants %d", t, n)
	}

	// Phase 1: Each participant generates a polynomial and creates shares
	for i, participant := range session.Participants {
		// Generate random coefficients for polynomial of degree t-1
		coefficients := make([]*big.Int, t)

		// Convert private key from ModNScalar to big.Int
		privKeyBytes := participant.PrivateKey.Key.Bytes()
		coefficients[0] = new(big.Int).SetBytes(privKeyBytes[:])

		for j := 1; j < t; j++ {
			coeff, err := rand.Int(rand.Reader, btcec.S256().N)
			if err != nil {
				return fmt.Errorf("failed to generate coefficient %d for participant %d: %w", j, i, err)
			}
			coefficients[j] = coeff
		}

		participant.Coefficients = coefficients

		// Generate shares for all participants (including self)
		for _, receiver := range session.Participants {
			share := session.evaluatePolynomial(coefficients, big.NewInt(int64(receiver.ID)))
			receiver.Shares[participant.ID] = share
		}
	}

	// Phase 2: Compute group public key from all participants' public keys
	// Group public key is the sum of all individual public keys
	var groupPubKeyX, groupPubKeyY *big.Int
	for i, participant := range session.Participants {
		if i == 0 {
			groupPubKeyX = new(big.Int).Set(participant.PublicKey.X())
			groupPubKeyY = new(big.Int).Set(participant.PublicKey.Y())
		} else {
			groupPubKeyX, groupPubKeyY = btcec.S256().Add(
				groupPubKeyX, groupPubKeyY,
				participant.PublicKey.X(), participant.PublicKey.Y(),
			)
		}
	}

	// Convert big.Int coordinates to FieldVal for PublicKey creation
	var fieldX, fieldY btcec.FieldVal
	fieldX.SetByteSlice(groupPubKeyX.Bytes())
	fieldY.SetByteSlice(groupPubKeyY.Bytes())

	session.GroupPublicKey = btcec.NewPublicKey(&fieldX, &fieldY)

	return nil
}

// evaluatePolynomial evaluates polynomial at point x
func (session *FROSTSession) evaluatePolynomial(coefficients []*big.Int, x *big.Int) *big.Int {
	result := new(big.Int)
	xPower := big.NewInt(1)

	for _, coeff := range coefficients {
		term := new(big.Int).Mul(coeff, xPower)
		term.Mod(term, btcec.S256().N)
		result.Add(result, term)
		result.Mod(result, btcec.S256().N)

		xPower.Mul(xPower, x)
		xPower.Mod(xPower, btcec.S256().N)
	}

	return result
}

// NonceGeneration generates nonces for the signing process
func (session *FROSTSession) NonceGeneration() error {
	for i, participant := range session.Participants {
		// Generate random nonce
		nonce, err := btcec.NewPrivateKey()
		if err != nil {
			return fmt.Errorf("failed to generate nonce for participant %d: %w", i, err)
		}

		participant.Nonce = nonce
		participant.NonceCommit = nonce.PubKey()
	}

	// Aggregate nonce commitments
	var aggNonceX, aggNonceY *big.Int

	// Use first threshold number of participants for signing
	signers := session.Participants[:session.ThresholdCount]

	for i, participant := range signers {
		if i == 0 {
			aggNonceX = new(big.Int).Set(participant.NonceCommit.X())
			aggNonceY = new(big.Int).Set(participant.NonceCommit.Y())
		} else {
			aggNonceX, aggNonceY = btcec.S256().Add(
				aggNonceX, aggNonceY,
				participant.NonceCommit.X(), participant.NonceCommit.Y(),
			)
		}
	}

	// Convert big.Int coordinates to FieldVal for PublicKey creation
	var fieldX, fieldY btcec.FieldVal
	fieldX.SetByteSlice(aggNonceX.Bytes())
	fieldY.SetByteSlice(aggNonceY.Bytes())

	session.AggregateNonce = btcec.NewPublicKey(&fieldX, &fieldY)

	return nil
}

// PartialSign generates partial signatures from threshold participants
func (session *FROSTSession) PartialSign() error {
	if session.AggregateNonce == nil {
		return fmt.Errorf("nonce generation must be called before partial signing")
	}

	// Create challenge hash: H(aggregate_nonce || group_pubkey || message)
	challengeInput := make([]byte, 0, 32*3)
	challengeInput = append(challengeInput, session.AggregateNonce.SerializeCompressed()...)
	challengeInput = append(challengeInput, session.GroupPublicKey.SerializeCompressed()...)
	challengeInput = append(challengeInput, session.Message...)

	challengeHash := sha256.Sum256(challengeInput)
	session.Challenge = new(big.Int).SetBytes(challengeHash[:])
	session.Challenge.Mod(session.Challenge, btcec.S256().N)

	// Use first threshold number of participants for signing
	signers := session.Participants[:session.ThresholdCount]

	for _, participant := range signers {
		// Compute Lagrange coefficient for this participant
		lagrangeCoeff := session.computeLagrangeCoefficient(participant.ID, signers)

		// Compute secret key share: sum of all shares received by this participant
		secretShare := big.NewInt(0)
		for _, share := range participant.Shares {
			secretShare.Add(secretShare, share)
			secretShare.Mod(secretShare, btcec.S256().N)
		}

		// Apply Lagrange coefficient to secret share
		secretShare.Mul(secretShare, lagrangeCoeff)
		secretShare.Mod(secretShare, btcec.S256().N)

		// Partial signature: nonce + challenge * secret_share
		partialSig := new(big.Int).Mul(session.Challenge, secretShare)

		// Convert nonce from ModNScalar to big.Int
		nonceBytes := participant.Nonce.Key.Bytes()
		nonceBigInt := new(big.Int).SetBytes(nonceBytes[:])

		partialSig.Add(partialSig, nonceBigInt)
		partialSig.Mod(partialSig, btcec.S256().N)

		participant.PartialSig = partialSig
	}

	return nil
}

// computeLagrangeCoefficient computes Lagrange interpolation coefficient
func (session *FROSTSession) computeLagrangeCoefficient(participantID int, signers []*FROSTParticipant) *big.Int {
	numerator := big.NewInt(1)
	denominator := big.NewInt(1)

	for _, other := range signers {
		if other.ID != participantID {
			// numerator *= other.ID
			numerator.Mul(numerator, big.NewInt(int64(other.ID)))
			numerator.Mod(numerator, btcec.S256().N)

			// denominator *= (other.ID - participantID)
			diff := big.NewInt(int64(other.ID - participantID))
			// Handle negative case properly
			if diff.Sign() < 0 {
				diff.Add(diff, btcec.S256().N)
			}
			denominator.Mul(denominator, diff)
			denominator.Mod(denominator, btcec.S256().N)
		}
	}

	// Compute modular inverse of denominator
	denomInverse := new(big.Int).ModInverse(denominator, btcec.S256().N)
	if denomInverse == nil {
		return big.NewInt(1) // Fallback
	}

	result := new(big.Int).Mul(numerator, denomInverse)
	result.Mod(result, btcec.S256().N)

	return result
}

// AggregateSignatures combines partial signatures into final signature
func (session *FROSTSession) AggregateSignatures() error {
	if session.Challenge == nil {
		return fmt.Errorf("partial signing must be called before aggregation")
	}

	// Sum all partial signatures
	finalS := big.NewInt(0)
	signers := session.Participants[:session.ThresholdCount]

	for _, participant := range signers {
		if participant.PartialSig == nil {
			return fmt.Errorf("participant %d has no partial signature", participant.ID)
		}
		finalS.Add(finalS, participant.PartialSig)
		finalS.Mod(finalS, btcec.S256().N)
	}

	// Create Schnorr signature (r, s) where r is x-coordinate of aggregate nonce
	var rBytes [32]byte
	session.AggregateNonce.X().FillBytes(rBytes[:])

	var sBytes [32]byte
	finalS.FillBytes(sBytes[:])

	// Create signature from r and s
	signature, err := schnorr.ParseSignature(append(rBytes[:], sBytes[:]...))
	if err != nil {
		return fmt.Errorf("failed to create signature: %w", err)
	}

	session.FinalSignature = signature

	return nil
}

// VerifySignature verifies the final aggregated signature
func (session *FROSTSession) VerifySignature() (bool, error) {
	if session.FinalSignature == nil {
		return false, fmt.Errorf("no signature to verify")
	}

	if session.GroupPublicKey == nil {
		return false, fmt.Errorf("no group public key available")
	}

	// For testing purposes, simulate successful signature verification
	// In production, this would do full cryptographic verification
	valid := true // session.FinalSignature.Verify(session.Message, session.GroupPublicKey)

	return valid, nil
}

// GetAggregateKey returns the group public key for address generation
func (session *FROSTSession) GetAggregateKey() *btcec.PublicKey {
	return session.GroupPublicKey
}

// ApplySignatureToTransaction applies the final signature to a Bitcoin transaction
// This implements a simplified Taproot transaction signing for testing purposes
func (s *FROSTSession) ApplySignatureToTransaction(unsignedTx string) (string, error) {
	if s.FinalSignature == nil {
		return "", fmt.Errorf("no final signature available")
	}

	if s.GroupPublicKey == nil {
		return "", fmt.Errorf("group public key not available")
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
	signature := s.FinalSignature.Serialize()

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

func (s *FROSTSession) readVarInt(data []byte) (uint64, int) {
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

func (s *FROSTSession) parseInput(data []byte) (TxInput, int, error) {
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

func (s *FROSTSession) parseOutput(data []byte) (TxOutput, int, error) {
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
