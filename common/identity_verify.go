package common

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// JCS canonicalization (RFC 8785) — produces a deterministic JSON serialization
// for Ed25519 signature verification of wallet identity presentations and documents.

func jcsCanonicalize(value any) (string, error) {
	return jcsCanonical(value)
}

func jcsCanonical(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "null", nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case string:
		return jsonString(v)
	case float64:
		return formatNumber(v), nil
	case int:
		return formatNumber(float64(v)), nil
	case int64:
		return formatNumber(float64(v)), nil
	case json.Number:
		s := v.String()
		return s, nil
	case map[string]any:
		return jcsObject(v)
	case []any:
		return jcsArray(v)
	default:
		// Fallback: marshal to JSON then re-parse
		raw, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		var reparsed any
		if err := json.Unmarshal(raw, &reparsed); err != nil {
			return "", err
		}
		return jcsCanonical(reparsed)
	}
}

func jsonString(s string) (string, error) {
	// Use json.Marshal for proper escaping
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func formatNumber(v float64) string {
	// Simplified: use the JSON representation; Go json.Marshal already
	// produces a canonical-enough number format for our purposes since
	// the wallet uses the same JCS canonicalization approach.
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func jcsObject(obj map[string]any) (string, error) {
	// JCS requires keys to be sorted in lexicographic order (by UTF-16 code units).
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	// Sort using a comparison that matches JCS (lexicographic by Unicode code point).
	// Go's standard string sort is byte-wise, which is correct for ASCII and
	// for UTF-8 encoded strings when comparing by code point order.
	stringSliceSort(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		ks, err := jsonString(k)
		if err != nil {
			return "", err
		}
		vs, err := jcsCanonical(obj[k])
		if err != nil {
			return "", err
		}
		parts = append(parts, ks+":"+vs)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func jcsArray(arr []any) (string, error) {
	parts := make([]string, 0, len(arr))
	for _, item := range arr {
		s, err := jcsCanonical(item)
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

func stringSliceSort(s []string) {
	// Simple insertion sort to avoid importing sort if not needed,
	// but sort is in stdlib so use it.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// IdentityPresentation represents the wallet identity presentation structure.
type IdentityPresentation struct {
	Version         int             `json:"version"`
	Holder          string          `json:"holder"`
	Audience        string          `json:"audience"`
	Nonce           string          `json:"nonce"`
	IssuedAt        string          `json:"issuedAt"`
	ExpiresAt       string          `json:"expiresAt"`
	Scopes          []string        `json:"scopes"`
	IdentityDocument json.RawMessage `json:"identityDocument,omitempty"`
	WalletProof     json.RawMessage `json:"walletProof,omitempty"`
	Credentials     []string        `json:"credentials,omitempty"`
	Proof           PresentationProof `json:"proof"`
}

type PresentationProof struct {
	Type              string `json:"type"`
	VerificationMethod string `json:"verificationMethod"`
	Purpose           string `json:"purpose"`
	ProofValue        string `json:"proofValue"`
}

type IdentityController struct {
	ControllerID string   `json:"controllerId"`
	Kind         string   `json:"kind"`
	PublicKey    string   `json:"publicKey"`
	Algorithm    string   `json:"algorithm"`
	Purposes     []string `json:"purposes"`
	Status       string   `json:"status"`
}

type IdentityDocument struct {
	Version      int                  `json:"version"`
	ID           string               `json:"id"`
	WalletIdentityID string           `json:"walletIdentityId"`
	Controllers  []IdentityController `json:"controllers"`
}

// VerifyIdentityPresentation verifies the Ed25519 signature on a wallet identity
// presentation. It does NOT verify credentials (JWT-VC) — the caller must do
// that separately using the issuer's JWKS.
func VerifyIdentityPresentation(presentationJSON []byte, expectedAudience, expectedNonce string) (*IdentityPresentation, error) {
	var pres IdentityPresentation
	if err := json.Unmarshal(presentationJSON, &pres); err != nil {
		return nil, fmt.Errorf("IDENTITY_PRESENTATION_INVALID: %w", err)
	}

	if pres.Version != 1 {
		return nil, errors.New("IDENTITY_PRESENTATION_INVALID")
	}
	if !strings.HasPrefix(pres.Holder, "did:yeying:wid_") {
		return nil, errors.New("IDENTITY_PRESENTATION_INVALID")
	}
	if pres.Proof.Type != "YeyingIdentityPresentationProofV1" || pres.Proof.Purpose != "authentication" {
		return nil, errors.New("IDENTITY_PRESENTATION_INVALID")
	}
	if pres.Audience != expectedAudience {
		return nil, errors.New("IDENTITY_AUDIENCE_MISMATCH")
	}
	if pres.Nonce != expectedNonce {
		return nil, errors.New("IDENTITY_NONCE_MISMATCH")
	}

	// Check time validity
	issuedAt, err := time.Parse(time.RFC3339, pres.IssuedAt)
	if err != nil {
		return nil, errors.New("IDENTITY_PRESENTATION_EXPIRED")
	}
	expiresAt, err := time.Parse(time.RFC3339, pres.ExpiresAt)
	if err != nil {
		return nil, errors.New("IDENTITY_PRESENTATION_EXPIRED")
	}
	now := time.Now()
	skew := 60 * time.Second
	if issuedAt.After(now.Add(skew)) {
		return nil, errors.New("IDENTITY_PRESENTATION_EXPIRED")
	}
	if !expiresAt.After(now.Add(-skew)) {
		return nil, errors.New("IDENTITY_PRESENTATION_EXPIRED")
	}

	// Find the controller public key from the identity document
	if len(pres.IdentityDocument) == 0 {
		return nil, errors.New("IDENTITY_PRESENTATION_KEY_MISSING")
	}
	var doc IdentityDocument
	if err := json.Unmarshal(pres.IdentityDocument, &doc); err != nil {
		return nil, fmt.Errorf("IDENTITY_PRESENTATION_INVALID: %w", err)
	}
	if doc.ID != pres.Holder {
		return nil, errors.New("IDENTITY_PRESENTATION_INVALID")
	}

	controller := findController(doc.Controllers, pres.Proof.VerificationMethod, pres.Holder)
	if controller == nil {
		return nil, errors.New("IDENTITY_CONTROLLER_NOT_AUTHORIZED")
	}
	if !containsPurpose(controller.Purposes, "authentication") || controller.Status != "active" {
		return nil, errors.New("IDENTITY_CONTROLLER_NOT_AUTHORIZED")
	}

	// Decode the public key (base64url raw Ed25519 public key)
	pubBytes, err := base64.RawURLEncoding.DecodeString(controller.PublicKey)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return nil, errors.New("IDENTITY_PRESENTATION_KEY_MISSING")
	}

	// Decode the signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(pres.Proof.ProofValue)
	if err != nil {
		return nil, errors.New("IDENTITY_PRESENTATION_PROOF_INVALID")
	}

	// Canonicalize the unsigned presentation (everything except "proof")
	// Re-marshal without the proof field, then canonicalize
	var fullMap map[string]any
	if err := json.Unmarshal(presentationJSON, &fullMap); err != nil {
		return nil, errors.New("IDENTITY_PRESENTATION_INVALID")
	}
	delete(fullMap, "proof")
	canonical, err := jcsCanonical(fullMap)
	if err != nil {
		return nil, fmt.Errorf("IDENTITY_PRESENTATION_INVALID: %w", err)
	}

	if !ed25519.Verify(ed25519.PublicKey(pubBytes), []byte(canonical), sigBytes) {
		return nil, errors.New("IDENTITY_PRESENTATION_PROOF_INVALID")
	}

	return &pres, nil
}

func findController(controllers []IdentityController, method, holder string) *IdentityController {
	for i := range controllers {
		expected := holder + "#" + controllers[i].ControllerID
		if expected == method {
			return &controllers[i]
		}
	}
	return nil
}

func containsPurpose(purposes []string, target string) bool {
	for _, p := range purposes {
		if p == target {
			return true
		}
	}
	return false
}
