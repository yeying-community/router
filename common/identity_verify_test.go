package common

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestJCSCanonical(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"null", "null", "null"},
		{"bool true", "true", "true"},
		{"bool false", "false", "false"},
		{"string", `"hello"`, `"hello"`},
		{"integer", "42", "42"},
		{"empty object", `{}`, `{}`},
		{"empty array", `[]`, `[]`},
		{"sorted object", `{"b":1,"a":2}`, `{"a":2,"b":1}`},
		{"nested", `{"z":{"b":1,"a":2},"y":[3,2,1]}`, `{"y":[3,2,1],"z":{"a":2,"b":1}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var val any
			if err := json.Unmarshal([]byte(tc.input), &val); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got, err := jcsCanonical(val)
			if err != nil {
				t.Fatalf("canonicalize: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestVerifyIdentityPresentation(t *testing.T) {
	// Generate Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubB64 := base64.RawURLEncoding.EncodeToString(pub)
	holder := "did:yeying:wid_test1234567890abcdefghijklm"
	controllerID := "controller-1"
	now := time.Now().UTC()
	issuedAt := now.Format(time.RFC3339)
	expiresAt := now.Add(5 * time.Minute).Format(time.RFC3339)

	identityDoc := map[string]any{
		"version": 1,
		"id":      holder,
		"controllers": []any{
			map[string]any{
				"controllerId": controllerID,
				"kind":         "wallet_key",
				"publicKey":    pubB64,
				"algorithm":    "Ed25519",
				"purposes":     []any{"authentication", "assertion", "manage"},
				"status":       "active",
			},
		},
	}

	unsigned := map[string]any{
		"version":          1,
		"holder":           holder,
		"audience":         "https://app.example.com",
		"nonce":            "test-nonce-123",
		"issuedAt":         issuedAt,
		"expiresAt":        expiresAt,
		"scopes":           []any{"identity.basic", "identity.wallet"},
		"identityDocument": identityDoc,
	}

	canonical, err := jcsCanonical(unsigned)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	sig := ed25519.Sign(priv, []byte(canonical))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	full := map[string]any{
		"version":          unsigned["version"],
		"holder":           unsigned["holder"],
		"audience":         unsigned["audience"],
		"nonce":            unsigned["nonce"],
		"issuedAt":         unsigned["issuedAt"],
		"expiresAt":        unsigned["expiresAt"],
		"scopes":           unsigned["scopes"],
		"identityDocument": unsigned["identityDocument"],
		"proof": map[string]any{
			"type":              "YeyingIdentityPresentationProofV1",
			"verificationMethod": holder + "#" + controllerID,
			"purpose":           "authentication",
			"proofValue":         sigB64,
		},
	}

	presJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Valid presentation
	pres, err := VerifyIdentityPresentation(presJSON, "https://app.example.com", "test-nonce-123")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if pres.Holder != holder {
		t.Fatalf("holder mismatch: %s != %s", pres.Holder, holder)
	}

	// Wrong audience
	_, err = VerifyIdentityPresentation(presJSON, "https://wrong.example.com", "test-nonce-123")
	if err == nil {
		t.Fatal("expected audience mismatch error")
	}

	// Wrong nonce
	_, err = VerifyIdentityPresentation(presJSON, "https://app.example.com", "wrong-nonce")
	if err == nil {
		t.Fatal("expected nonce mismatch error")
	}

	// Tampered presentation
	full["nonce"] = "tampered"
	tamperedJSON, _ := json.Marshal(full)
	_, err = VerifyIdentityPresentation(tamperedJSON, "https://app.example.com", "tampered")
	if err == nil {
		t.Fatal("expected proof invalid error for tampered presentation")
	}
}
