package common

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/yeying-community/router/common/config"
)

type UcanCapability struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

type ucanRootProof struct {
	Type string           `json:"type"`
	Iss  string           `json:"iss"`
	Aud  string           `json:"aud"`
	Cap  []UcanCapability `json:"cap"`
	Exp  int64            `json:"exp"`
	Nbf  *int64           `json:"nbf,omitempty"`
	Siwe struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	} `json:"siwe"`
}

type ucanStatement struct {
	Aud string           `json:"aud"`
	Cap []UcanCapability `json:"cap"`
	Exp int64            `json:"exp"`
	Nbf *int64           `json:"nbf,omitempty"`
}

type ucanPayload struct {
	Iss string            `json:"iss"`
	Aud string            `json:"aud"`
	Cap []UcanCapability  `json:"cap"`
	Exp int64             `json:"exp"`
	Nbf *int64            `json:"nbf,omitempty"`
	Prf []json.RawMessage `json:"prf"`
}

func ResolveUcanAudience() string {
	value := strings.TrimSpace(config.UcanAud)
	if value != "" {
		return value
	}
	if envPort := strings.TrimSpace(os.Getenv("PORT")); envPort != "" {
		return fmt.Sprintf("did:web:localhost:%s", envPort)
	}
	if Port != nil && *Port != 0 {
		return fmt.Sprintf("did:web:localhost:%d", *Port)
	}
	return "did:web:localhost"
}

func IsUcanToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return false
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return false
	}
	if typ, ok := header["typ"].(string); ok && typ == "UCAN" {
		return true
	}
	if alg, ok := header["alg"].(string); ok && alg == "EdDSA" {
		return true
	}
	return false
}

func VerifyUcanInvocation(token string, expectedAud string, required []UcanCapability) (string, error) {
	payload, exp, err := verifyUcanJws(token)
	if err != nil {
		return "", err
	}
	if payload.Aud != expectedAud {
		return "", errors.New("UCAN audience mismatch")
	}
	if !capsAllow(payload.Cap, required) {
		return "", errors.New("UCAN capability denied")
	}
	iss, err := verifyProofChain(payload.Iss, payload.Cap, exp, payload.Prf)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(iss, "did:pkh:eth:") {
		return "", errors.New("invalid UCAN issuer")
	}
	return strings.TrimPrefix(iss, "did:pkh:eth:"), nil
}

func base58Decode(input string) ([]byte, error) {
	bytes := []byte{0}
	for _, r := range input {
		index := strings.IndexRune(base58Alphabet, r)
		if index < 0 {
			return nil, errors.New("invalid base58 character")
		}
		carry := index
		for i := 0; i < len(bytes); i++ {
			carry += int(bytes[i]) * 58
			bytes[i] = byte(carry & 0xff)
			carry >>= 8
		}
		for carry > 0 {
			bytes = append(bytes, byte(carry&0xff))
			carry >>= 8
		}
	}
	zeros := 0
	for zeros < len(input) && input[zeros] == '1' {
		zeros++
	}
	output := make([]byte, zeros+len(bytes))
	for i := 0; i < zeros; i++ {
		output[i] = 0
	}
	for i := 0; i < len(bytes); i++ {
		output[len(output)-1-i] = bytes[i]
	}
	return output, nil
}

func didKeyToPublicKey(did string) ([]byte, error) {
	if !strings.HasPrefix(did, "did:key:z") {
		return nil, errors.New("invalid did:key format")
	}
	decoded, err := base58Decode(strings.TrimPrefix(did, "did:key:z"))
	if err != nil {
		return nil, err
	}
	if len(decoded) < 3 || decoded[0] != 0xed || decoded[1] != 0x01 {
		return nil, errors.New("unsupported did:key type")
	}
	return decoded[2:], nil
}

func normalizeEpochMillis(value int64) int64 {
	if value == 0 {
		return 0
	}
	if value < 1e12 {
		return value * 1000
	}
	return value
}

func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == value
}

func capsAllow(available []UcanCapability, required []UcanCapability) bool {
	if len(available) == 0 {
		return false
	}
	for _, req := range required {
		matched := false
		for _, cap := range available {
			if matchPattern(cap.Resource, req.Resource) && matchPattern(cap.Action, req.Action) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func extractUcanStatement(message string) (*ucanStatement, error) {
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "UCAN-AUTH") {
			jsonPart := strings.TrimSpace(trimmed[len("UCAN-AUTH"):])
			jsonPart = strings.TrimSpace(strings.TrimPrefix(jsonPart, ":"))
			var statement ucanStatement
			if err := json.Unmarshal([]byte(jsonPart), &statement); err != nil {
				return nil, err
			}
			return &statement, nil
		}
	}
	return nil, errors.New("missing UCAN statement")
}

func recoverEthAddress(message, signature string) (string, error) {
	sig := strings.TrimPrefix(signature, "0x")
	raw, err := hex.DecodeString(sig)
	if err != nil {
		return "", err
	}
	if len(raw) != 65 {
		return "", errors.New("invalid signature length")
	}
	if raw[64] >= 27 {
		raw[64] -= 27
	}
	hash := accounts.TextHash([]byte(message))
	pub, err := crypto.SigToPub(hash, raw)
	if err != nil {
		return "", err
	}
	addr := crypto.PubkeyToAddress(*pub)
	return strings.ToLower(addr.Hex()), nil
}

func verifyRootProof(root ucanRootProof) (ucanStatement, string, error) {
	if root.Type != "siwe" || root.Siwe.Message == "" || root.Siwe.Signature == "" {
		return ucanStatement{}, "", errors.New("invalid root proof")
	}
	recovered, err := recoverEthAddress(root.Siwe.Message, root.Siwe.Signature)
	if err != nil {
		return ucanStatement{}, "", err
	}
	iss := "did:pkh:eth:" + recovered
	if root.Iss != "" && root.Iss != iss {
		return ucanStatement{}, "", errors.New("root issuer mismatch")
	}

	statement, err := extractUcanStatement(root.Siwe.Message)
	if err != nil {
		return ucanStatement{}, "", err
	}

	aud := statement.Aud
	if aud == "" {
		aud = root.Aud
	}
	exp := normalizeEpochMillis(statement.Exp)
	if exp == 0 {
		exp = normalizeEpochMillis(root.Exp)
	}
	cap := statement.Cap
	if len(cap) == 0 {
		cap = root.Cap
	}

	if aud == "" || exp == 0 || len(cap) == 0 {
		return ucanStatement{}, "", errors.New("invalid root claims")
	}
	if root.Aud != "" && root.Aud != aud {
		return ucanStatement{}, "", errors.New("root audience mismatch")
	}
	if root.Exp != 0 && normalizeEpochMillis(root.Exp) != exp {
		return ucanStatement{}, "", errors.New("root expiry mismatch")
	}

	var nbf *int64
	if statement.Nbf != nil {
		value := normalizeEpochMillis(*statement.Nbf)
		nbf = &value
	} else if root.Nbf != nil {
		value := normalizeEpochMillis(*root.Nbf)
		nbf = &value
	}

	nowMs := time.Now().UnixMilli()
	if nbf != nil && nowMs < *nbf {
		return ucanStatement{}, "", errors.New("root not active")
	}
	if nowMs > exp {
		return ucanStatement{}, "", errors.New("root expired")
	}

	statement.Aud = aud
	statement.Exp = exp
	statement.Cap = cap
	statement.Nbf = nbf

	return *statement, iss, nil
}

func decodeUcanToken(token string) (map[string]interface{}, ucanPayload, []byte, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ucanPayload{}, nil, "", errors.New("invalid UCAN token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ucanPayload{}, nil, "", err
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ucanPayload{}, nil, "", err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ucanPayload{}, nil, "", err
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ucanPayload{}, nil, "", err
	}
	var payload ucanPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, ucanPayload{}, nil, "", err
	}
	return header, payload, sig, parts[0] + "." + parts[1], nil
}

func verifyUcanJws(token string) (ucanPayload, int64, error) {
	header, payload, sig, signingInput, err := decodeUcanToken(token)
	if err != nil {
		return ucanPayload{}, 0, err
	}
	if alg, ok := header["alg"].(string); ok && alg != "EdDSA" {
		return ucanPayload{}, 0, errors.New("unsupported UCAN alg")
	}

	rawKey, err := didKeyToPublicKey(payload.Iss)
	if err != nil {
		return ucanPayload{}, 0, err
	}
	if !ed25519.Verify(rawKey, []byte(signingInput), sig) {
		return ucanPayload{}, 0, errors.New("invalid UCAN signature")
	}

	exp := normalizeEpochMillis(payload.Exp)
	nbf := int64(0)
	if payload.Nbf != nil {
		nbf = normalizeEpochMillis(*payload.Nbf)
	}
	nowMs := time.Now().UnixMilli()
	if nbf != 0 && nowMs < nbf {
		return ucanPayload{}, 0, errors.New("UCAN not active")
	}
	if exp != 0 && nowMs > exp {
		return ucanPayload{}, 0, errors.New("UCAN expired")
	}

	return payload, exp, nil
}

func verifyProofChain(currentDid string, required []UcanCapability, requiredExp int64, proofs []json.RawMessage) (string, error) {
	if len(proofs) == 0 {
		return "", errors.New("missing UCAN proof chain")
	}
	first := proofs[0]
	if len(first) > 0 && first[0] == '"' {
		var token string
		if err := json.Unmarshal(first, &token); err != nil {
			return "", err
		}
		payload, proofExp, err := verifyUcanJws(token)
		if err != nil {
			return "", err
		}
		if payload.Aud != currentDid {
			return "", errors.New("UCAN audience mismatch")
		}
		if !capsAllow(payload.Cap, required) {
			return "", errors.New("UCAN capability denied")
		}
		if proofExp != 0 && requiredExp != 0 && proofExp < requiredExp {
			return "", errors.New("UCAN proof expired")
		}
		nextProofs := payload.Prf
		if len(nextProofs) == 0 && len(proofs) > 1 {
			nextProofs = proofs[1:]
		}
		return verifyProofChain(payload.Iss, payload.Cap, proofExp, nextProofs)
	}

	var root ucanRootProof
	if err := json.Unmarshal(first, &root); err != nil {
		return "", err
	}
	statement, iss, err := verifyRootProof(root)
	if err != nil {
		return "", err
	}
	if statement.Aud != currentDid {
		return "", errors.New("root audience mismatch")
	}
	if !capsAllow(statement.Cap, required) {
		return "", errors.New("root capability denied")
	}
	if requiredExp != 0 && statement.Exp < requiredExp {
		return "", errors.New("root expired")
	}
	return iss, nil
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
