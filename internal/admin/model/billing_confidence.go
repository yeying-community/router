package model

// Billing confidence semantics — the single source of truth for "how trustworthy
// is this cost?" See docs/商业计费/成本与盈利核算标准.md §4.
//
// The procurement/billing layer historically graded trust with several overlapping
// enums (cost_source, cost_status, readiness, attribution status, settlement truth
// mode, cost confidence). They collapse onto TWO orthogonal axes plus one lifecycle
// dimension:
//
//	Axis A — cost basis confidence (how trustworthy the cost NUMBER is):
//	    actual  >  zero_cost  >  estimated  >  unconfigured/none
//	Axis B — usage confidence (how trustworthy the consumed QUANTITY is):
//	    returned_usage > hybrid_usage > local_estimate > unit_based > unmetered
//	    (defined in internal/relay/billing/procurement_observability.go)
//
//	Lifecycle (NOT confidence): cost_status ∈ {active, cost_unconfigured, exhausted,
//	    expired, disabled} — a batch's state over time, orthogonal to both axes.
//
// readiness (ready/missing/estimated/exhausted/expired/unit_mismatch/untracked) is a
// PROJECTION of these axes onto the "is this model ready?" view, not a seventh enum.
//
// This file centralizes Axis A membership so the same two cost-source sets are not
// re-spelled inline at every call site. Two sets matter:
//
//	Formal  = {actual, zero_cost}            — production-grade cost basis. Only these
//	                                            back publish gating and margin truth.
//	Usable  = {actual, estimated, zero_cost} — may enter estimate/consume/attribution;
//	                                            "estimated" is planning-grade, not formal.

// FormalProcurementCostSources returns the production-grade cost-source set
// (actual, zero_cost). A fresh slice is returned on each call so callers may pass it
// straight into a GORM `IN ?` clause without risking mutation of shared state.
func FormalProcurementCostSources() []string {
	return []string{ProcurementCostSourceActual, ProcurementCostSourceZeroCost}
}

// UsableProcurementCostSources returns the set that may back estimate/consume/
// attribution (actual, estimated, zero_cost). Unlike the formal set this includes
// planning-grade estimates. A fresh slice is returned per call (see above).
func UsableProcurementCostSources() []string {
	return []string{
		ProcurementCostSourceActual,
		ProcurementCostSourceEstimated,
		ProcurementCostSourceZeroCost,
	}
}

// IsFormalProcurementCostSource reports whether the source is production-grade
// (actual or explicitly zero_cost). Input is normalized first.
func IsFormalProcurementCostSource(source string) bool {
	switch normalizeProcurementCostSource(source) {
	case ProcurementCostSourceActual, ProcurementCostSourceZeroCost:
		return true
	default:
		return false
	}
}

// IsUsableProcurementCostSource reports whether the source may enter estimate/
// consume/attribution (actual, estimated, or zero_cost). Input is normalized first.
func IsUsableProcurementCostSource(source string) bool {
	switch normalizeProcurementCostSource(source) {
	case ProcurementCostSourceActual,
		ProcurementCostSourceEstimated,
		ProcurementCostSourceZeroCost:
		return true
	default:
		return false
	}
}
