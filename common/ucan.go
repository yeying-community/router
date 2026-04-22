package common

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
)

type UcanCapability struct {
	// UCAN canonical fields (recommended)
	With string `json:"with,omitempty"`
	Can  string `json:"can,omitempty"`
	NB   any    `json:"nb,omitempty"`

	// Compatibility fields used by existing clients.
	Resource string `json:"resource,omitempty"`
	Action   string `json:"action,omitempty"`
}

type ucanRootProof struct {
	Type         string                            `json:"type"`
	Iss          string                            `json:"iss"`
	Aud          string                            `json:"aud"`
	Cap          []UcanCapability                  `json:"cap,omitempty"`
	Capabilities []UcanCapability                  `json:"capabilities,omitempty"`
	Att          map[string]map[string]interface{} `json:"att,omitempty"`
	Exp          int64                             `json:"exp"`
	Nbf          *int64                            `json:"nbf,omitempty"`
	Siwe         struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	} `json:"siwe"`
}

type ucanStatement struct {
	Aud          string                            `json:"aud"`
	Cap          []UcanCapability                  `json:"cap,omitempty"`
	Capabilities []UcanCapability                  `json:"capabilities,omitempty"`
	Att          map[string]map[string]interface{} `json:"att,omitempty"`
	Exp          int64                             `json:"exp"`
	Nbf          *int64                            `json:"nbf,omitempty"`
}

type ucanPayload struct {
	Iss          string                            `json:"iss"`
	Aud          string                            `json:"aud"`
	Sub          string                            `json:"sub"`
	Cap          []UcanCapability                  `json:"cap,omitempty"`
	Capabilities []UcanCapability                  `json:"capabilities,omitempty"`
	Att          map[string]map[string]interface{} `json:"att,omitempty"`
	Exp          int64                             `json:"exp"`
	Nbf          *int64                            `json:"nbf,omitempty"`
	Prf          []json.RawMessage                 `json:"prf"`
}

func ResolveUcanAudience() string {
	value := strings.TrimSpace(config.UcanAud)
	if value != "" {
		return value
	}
	if Port != nil && *Port != 0 {
		return fmt.Sprintf("did:web:localhost:%d", *Port)
	}
	return "did:web:localhost"
}

func resolveDefaultUcanResource() string {
	aud := strings.TrimSpace(ResolveUcanAudience())
	const prefix = "did:web:"
	if strings.HasPrefix(strings.ToLower(aud), prefix) {
		host := strings.TrimSpace(aud[len(prefix):])
		if host != "" {
			return config.DefaultUcanResourcePrefix + host
		}
	}
	return config.DefaultUcanResourcePrefix + "localhost"
}

func normalizeAppResource(resource string) string {
	trimmed := strings.TrimSpace(resource)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "app:") {
		return trimmed
	}
	suffix := strings.TrimSpace(trimmed[len("app:"):])
	if suffix == "" || suffix == "*" || strings.Contains(suffix, ":") {
		// Keep wildcard and non-canonical legacy forms (e.g. app:127.0.0.1:8001) unchanged.
		return trimmed
	}
	return "app:all:" + suffix
}

func normalizeUcanCapability(cap UcanCapability) (UcanCapability, bool) {
	resource := strings.TrimSpace(cap.Resource)
	if resource == "" {
		resource = strings.TrimSpace(cap.With)
	}
	action := strings.TrimSpace(cap.Action)
	if action == "" {
		action = strings.TrimSpace(cap.Can)
	}
	resource = normalizeAppResource(resource)
	if resource == "" || action == "" {
		return UcanCapability{}, false
	}
	return UcanCapability{
		With:     resource,
		Can:      action,
		NB:       cap.NB,
		Resource: resource,
		Action:   action,
	}, true
}

func normalizeUcanCapabilities(caps []UcanCapability) []UcanCapability {
	if len(caps) == 0 {
		return nil
	}
	result := make([]UcanCapability, 0, len(caps))
	seen := make(map[string]struct{}, len(caps))
	for _, cap := range caps {
		normalized, ok := normalizeUcanCapability(cap)
		if !ok {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(normalized.Resource)) + "|" + strings.ToLower(strings.TrimSpace(normalized.Action))
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func capabilitiesFromAtt(att map[string]map[string]interface{}) []UcanCapability {
	if len(att) == 0 {
		return nil
	}
	caps := make([]UcanCapability, 0)
	for resource, actions := range att {
		resource = strings.TrimSpace(resource)
		if resource == "" {
			continue
		}
		for action, constraints := range actions {
			action = strings.TrimSpace(action)
			if action == "" {
				continue
			}
			caps = append(caps, UcanCapability{
				With:     resource,
				Can:      action,
				NB:       constraints,
				Resource: resource,
				Action:   action,
			})
		}
	}
	return normalizeUcanCapabilities(caps)
}

func collectUcanCapabilities(cap []UcanCapability, capabilities []UcanCapability, att map[string]map[string]interface{}) []UcanCapability {
	merged := make([]UcanCapability, 0, len(cap)+len(capabilities))
	merged = append(merged, cap...)
	merged = append(merged, capabilities...)
	merged = append(merged, capabilitiesFromAtt(att)...)
	return normalizeUcanCapabilities(merged)
}

func capabilityEquals(a UcanCapability, b UcanCapability) bool {
	na, okA := normalizeUcanCapability(a)
	nb, okB := normalizeUcanCapability(b)
	if !okA || !okB {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(na.Resource), strings.TrimSpace(nb.Resource)) &&
		strings.EqualFold(strings.TrimSpace(na.Action), strings.TrimSpace(nb.Action))
}

func appendCapabilitySetIfMissing(sets [][]UcanCapability, cap UcanCapability) [][]UcanCapability {
	normalized, ok := normalizeUcanCapability(cap)
	if !ok {
		return sets
	}
	target := []UcanCapability{normalized}
	for _, existing := range sets {
		if len(existing) != 1 {
			continue
		}
		if capabilityEquals(existing[0], target[0]) {
			return sets
		}
	}
	return append(sets, target)
}

func ResolveUcanRequiredCapabilitySets() [][]UcanCapability {
	resource := strings.TrimSpace(config.UcanResource)
	action := strings.TrimSpace(config.UcanAction)
	if resource == "" {
		resource = resolveDefaultUcanResource()
	}
	if action == "" {
		action = config.DefaultUcanAction
	}

	current := UcanCapability{With: resource, Can: action, Resource: resource, Action: action}
	defaultCap := UcanCapability{Resource: resolveDefaultUcanResource(), Action: config.DefaultUcanAction}
	compatCaps := []UcanCapability{
		{Resource: config.AppScopedCompatUcanResource, Action: config.AppScopedCompatUcanAction},
		{Resource: config.AppCompatUcanResource, Action: config.AppCompatUcanAction},
		{Resource: config.CompatUcanResource, Action: config.CompatUcanAction},
		{Resource: config.ProfileCompatUcanResource, Action: config.ProfileCompatUcanAction},
	}

	knownCaps := make([]UcanCapability, 0, len(compatCaps)+1)
	knownCaps = append(knownCaps, defaultCap)
	knownCaps = append(knownCaps, compatCaps...)

	sets := [][]UcanCapability{{current}}
	isKnown := false
	for _, cap := range knownCaps {
		if capabilityEquals(current, cap) {
			isKnown = true
			break
		}
	}
	if !isKnown {
		return sets
	}

	for _, cap := range knownCaps {
		sets = appendCapabilitySetIfMissing(sets, cap)
	}
	return sets
}

func isCapabilityDeniedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "capability denied")
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
	if !isEquivalentAudience(payload.Aud, expectedAud) {
		logger.Loginf(nil, "UCAN audience mismatch expected=%s actual=%s", expectedAud, payload.Aud)
		return "", errors.New("UCAN audience mismatch")
	}
	if !capsAllow(payload.Cap, required) {
		return "", errors.New("UCAN capability denied")
	}
	if isTrustedUcanIssuerDid(payload.Iss) {
		subject := normalizeWalletSubject(payload.Sub)
		if subject == "" {
			return "", errors.New("invalid UCAN subject")
		}
		return subject, nil
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

func VerifyUcanInvocationAny(token string, expectedAud string, requiredSets [][]UcanCapability) (string, error) {
	if len(requiredSets) == 0 {
		return "", errors.New("missing UCAN capability requirement")
	}
	var lastCapabilityErr error
	for _, required := range requiredSets {
		if len(required) == 0 {
			continue
		}
		address, err := VerifyUcanInvocation(token, expectedAud, required)
		if err == nil {
			return address, nil
		}
		if !isCapabilityDeniedError(err) {
			return "", err
		}
		lastCapabilityErr = err
	}
	if lastCapabilityErr != nil {
		return "", lastCapabilityErr
	}
	return "", errors.New("missing UCAN capability requirement")
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
	pattern = strings.TrimSpace(pattern)
	value = strings.TrimSpace(value)
	if pattern == "" || value == "" {
		return false
	}
	patternLower := strings.ToLower(normalizeLoopbackAlias(pattern))
	valueLower := strings.ToLower(normalizeLoopbackAlias(value))
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(patternLower, "*") {
		return strings.HasPrefix(valueLower, strings.TrimSuffix(patternLower, "*"))
	}
	return patternLower == valueLower
}

func capsAllow(available []UcanCapability, required []UcanCapability) bool {
	normalizedAvailable := normalizeUcanCapabilities(available)
	normalizedRequired := normalizeUcanCapabilities(required)
	if len(normalizedAvailable) == 0 || len(normalizedRequired) == 0 {
		return false
	}
	for _, req := range normalizedRequired {
		matched := false
		for _, cap := range normalizedAvailable {
			resourceMatched :=
				matchPattern(cap.Resource, req.Resource) ||
					matchPattern(req.Resource, cap.Resource)
			actionMatched :=
				matchPattern(cap.Action, req.Action) ||
					matchPattern(req.Action, cap.Action)
			if resourceMatched && actionMatched {
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
	cap := collectUcanCapabilities(statement.Cap, statement.Capabilities, statement.Att)
	if len(cap) == 0 {
		cap = collectUcanCapabilities(root.Cap, root.Capabilities, root.Att)
	}

	if aud == "" || exp == 0 || len(cap) == 0 {
		return ucanStatement{}, "", errors.New("invalid root claims")
	}
	if root.Aud != "" && !isEquivalentAudience(root.Aud, aud) {
		logger.Loginf(nil, "UCAN root audience mismatch expected=%s actual=%s", root.Aud, aud)
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
	statement.Capabilities = cap
	statement.Att = nil
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

	payload.Cap = collectUcanCapabilities(payload.Cap, payload.Capabilities, payload.Att)
	payload.Capabilities = payload.Cap
	payload.Att = nil

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
		if !isEquivalentAudience(payload.Aud, currentDid) {
			logger.Loginf(nil, "UCAN audience mismatch expected=%s actual=%s", currentDid, payload.Aud)
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
	if !isEquivalentAudience(statement.Aud, currentDid) {
		logger.Loginf(nil, "UCAN root audience mismatch expected=%s actual=%s", currentDid, statement.Aud)
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

func normalizeLoopbackAlias(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return strings.ReplaceAll(trimmed, "127.0.0.1", "localhost")
}

func isEquivalentAudience(left, right string) bool {
	leftTrimmed := strings.TrimSpace(left)
	rightTrimmed := strings.TrimSpace(right)
	if leftTrimmed == "" || rightTrimmed == "" {
		return false
	}
	if strings.EqualFold(leftTrimmed, rightTrimmed) {
		return true
	}
	return strings.EqualFold(normalizeLoopbackAlias(leftTrimmed), normalizeLoopbackAlias(rightTrimmed))
}

func isTrustedUcanIssuerDid(did string) bool {
	target := strings.TrimSpace(did)
	if target == "" {
		return false
	}
	for _, issuerDid := range config.UcanTrustedIssuerDIDs {
		if strings.EqualFold(strings.TrimSpace(issuerDid), target) {
			return true
		}
	}
	return false
}

func normalizeWalletSubject(subject string) string {
	trimmed := strings.TrimSpace(subject)
	if len(trimmed) != 42 || !strings.HasPrefix(strings.ToLower(trimmed), "0x") {
		return ""
	}
	if _, err := hex.DecodeString(trimmed[2:]); err != nil {
		return ""
	}
	return strings.ToLower(trimmed)
}
