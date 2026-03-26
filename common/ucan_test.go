package common

import (
	"encoding/json"
	"testing"

	"github.com/yeying-community/router/common/config"
)

func TestResolveUcanRequiredCapabilitySets_DefaultIncludesCompatAliases(t *testing.T) {
	prevResource := config.UcanResource
	prevAction := config.UcanAction
	prevAud := config.UcanAud
	defer func() {
		config.UcanResource = prevResource
		config.UcanAction = prevAction
		config.UcanAud = prevAud
	}()

	config.UcanResource = ""
	config.UcanAction = ""
	config.UcanAud = "did:web:router.example.com"

	sets := ResolveUcanRequiredCapabilitySets()
	assertHasSingleCapabilitySet(t, sets, UcanCapability{
		Resource: "llm:router.example.com",
		Action:   config.DefaultUcanAction,
	})
	assertHasSingleCapabilitySet(t, sets, UcanCapability{
		Resource: config.AppScopedCompatUcanResource,
		Action:   config.AppScopedCompatUcanAction,
	})
	assertHasSingleCapabilitySet(t, sets, UcanCapability{
		Resource: config.AppCompatUcanResource,
		Action:   config.AppCompatUcanAction,
	})
	assertHasSingleCapabilitySet(t, sets, UcanCapability{
		Resource: config.CompatUcanResource,
		Action:   config.CompatUcanAction,
	})
	assertHasSingleCapabilitySet(t, sets, UcanCapability{
		Resource: config.ProfileCompatUcanResource,
		Action:   config.ProfileCompatUcanAction,
	})
}

func TestResolveUcanRequiredCapabilitySets_CustomCapabilityNoCompatFallback(t *testing.T) {
	prevResource := config.UcanResource
	prevAction := config.UcanAction
	defer func() {
		config.UcanResource = prevResource
		config.UcanAction = prevAction
	}()

	config.UcanResource = "custom:capability"
	config.UcanAction = "read"

	sets := ResolveUcanRequiredCapabilitySets()
	if len(sets) != 1 || len(sets[0]) != 1 {
		t.Fatalf("expected only one required capability set, got %#v", sets)
	}
	if !capabilityEquals(sets[0][0], UcanCapability{
		Resource: "custom:capability",
		Action:   "read",
	}) {
		t.Fatalf("unexpected required capability: %#v", sets[0][0])
	}
}

func TestCapsAllow_AppWildcardRequiredMatchesExactAvailable(t *testing.T) {
	available := []UcanCapability{
		{Resource: "app:localhost-3020", Action: "invoke"},
	}
	required := []UcanCapability{
		{Resource: "app:*", Action: "invoke"},
	}
	if !capsAllow(available, required) {
		t.Fatalf("expected app wildcard requirement to match exact app capability")
	}
}

func TestCapsAllow_WithCanMatchesResourceAction(t *testing.T) {
	available := []UcanCapability{
		{With: "app:localhost-3020", Can: "invoke"},
	}
	required := []UcanCapability{
		{Resource: "app:all:localhost-3020", Action: "invoke"},
	}
	if !capsAllow(available, required) {
		t.Fatalf("expected with/can capability to match resource/action requirement")
	}
}

func TestCollectUcanCapabilities_FromRecapAtt(t *testing.T) {
	att := map[string]map[string]interface{}{
		"app:all:chat.example.com": {
			"invoke": []map[string]any{
				{"model": "gpt-5.4"},
			},
		},
	}
	caps := collectUcanCapabilities(nil, nil, att)
	if len(caps) != 1 {
		t.Fatalf("expected one capability from att, got %#v", caps)
	}
	if caps[0].Resource != "app:all:chat.example.com" || caps[0].Action != "invoke" {
		t.Fatalf("unexpected capability from att: %#v", caps[0])
	}
	if caps[0].NB == nil {
		t.Fatalf("expected constraints(nb) to be kept")
	}
}

func TestExtractUcanStatement_SupportsRecapAtt(t *testing.T) {
	statementLine := `UCAN-AUTH {"aud":"did:web:router.example.com","exp":1893456000000,"att":{"app:all:chat.example.com":{"invoke":[{"model":"gpt-5.4"}]}}}`
	statement, err := extractUcanStatement(statementLine)
	if err != nil {
		t.Fatalf("extractUcanStatement failed: %v", err)
	}
	caps := collectUcanCapabilities(statement.Cap, statement.Capabilities, statement.Att)
	if len(caps) != 1 {
		raw, _ := json.Marshal(statement)
		t.Fatalf("expected one normalized capability, got caps=%#v statement=%s", caps, string(raw))
	}
	if caps[0].Resource != "app:all:chat.example.com" || caps[0].Action != "invoke" {
		t.Fatalf("unexpected normalized capability: %#v", caps[0])
	}
}

func TestCapsAllow_RequiredInvokeDoesNotMatchWrite(t *testing.T) {
	available := []UcanCapability{
		{Resource: "app:localhost-3020", Action: "write"},
	}
	required := []UcanCapability{
		{Resource: "app:*", Action: "invoke"},
	}
	if capsAllow(available, required) {
		t.Fatalf("expected invoke requirement to reject write-only capability")
	}
}

func assertHasSingleCapabilitySet(t *testing.T, sets [][]UcanCapability, target UcanCapability) {
	t.Helper()
	for _, set := range sets {
		if len(set) != 1 {
			continue
		}
		if capabilityEquals(set[0], target) {
			return
		}
	}
	t.Fatalf("missing capability set: %#v in %#v", target, sets)
}
