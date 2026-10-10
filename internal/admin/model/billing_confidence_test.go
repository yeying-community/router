package model

import (
	"reflect"
	"testing"
)

func TestFormalProcurementCostSources(t *testing.T) {
	want := []string{ProcurementCostSourceActual, ProcurementCostSourceZeroCost}
	if got := FormalProcurementCostSources(); !reflect.DeepEqual(got, want) {
		t.Fatalf("FormalProcurementCostSources() = %v, want %v", got, want)
	}
	// Fresh slice per call so GORM IN ? callers can't mutate shared state.
	a := FormalProcurementCostSources()
	a[0] = "mutated"
	if b := FormalProcurementCostSources(); b[0] != ProcurementCostSourceActual {
		t.Fatalf("FormalProcurementCostSources() not isolated across calls: %v", b)
	}
}

func TestUsableProcurementCostSources(t *testing.T) {
	want := []string{
		ProcurementCostSourceActual,
		ProcurementCostSourceEstimated,
		ProcurementCostSourceZeroCost,
	}
	if got := UsableProcurementCostSources(); !reflect.DeepEqual(got, want) {
		t.Fatalf("UsableProcurementCostSources() = %v, want %v", got, want)
	}
}

func TestIsFormalProcurementCostSource(t *testing.T) {
	cases := map[string]bool{
		ProcurementCostSourceActual:    true,
		ProcurementCostSourceZeroCost:  true,
		ProcurementCostSourceEstimated: false,
		ProcurementCostSourceNone:      false,
		"":                             false,
		"garbage":                      false,
		"  Actual  ":                   true, // normalized (trim + lower)
	}
	for source, want := range cases {
		if got := IsFormalProcurementCostSource(source); got != want {
			t.Fatalf("IsFormalProcurementCostSource(%q) = %v, want %v", source, got, want)
		}
	}
}

func TestIsUsableProcurementCostSource(t *testing.T) {
	cases := map[string]bool{
		ProcurementCostSourceActual:    true,
		ProcurementCostSourceEstimated: true,
		ProcurementCostSourceZeroCost:  true,
		ProcurementCostSourceNone:      false,
		"":                             false,
		"garbage":                      false,
		"ZERO_COST":                    true, // normalized
	}
	for source, want := range cases {
		if got := IsUsableProcurementCostSource(source); got != want {
			t.Fatalf("IsUsableProcurementCostSource(%q) = %v, want %v", source, got, want)
		}
	}
}

// Formal is a strict subset of Usable: every formal source is usable, and the only
// usable-but-not-formal source is the planning-grade estimate.
func TestFormalSubsetOfUsable(t *testing.T) {
	usable := map[string]bool{}
	for _, s := range UsableProcurementCostSources() {
		usable[s] = true
	}
	for _, s := range FormalProcurementCostSources() {
		if !usable[s] {
			t.Fatalf("formal source %q is not in the usable set", s)
		}
	}
	if IsFormalProcurementCostSource(ProcurementCostSourceEstimated) {
		t.Fatalf("estimated must not be formal")
	}
	if !IsUsableProcurementCostSource(ProcurementCostSourceEstimated) {
		t.Fatalf("estimated must be usable")
	}
}
