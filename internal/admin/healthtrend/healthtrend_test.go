package healthtrend

import "testing"

func TestBuildPointsAggregatesFiveMinuteBuckets(t *testing.T) {
	now := int64(1700001800)
	bucket := BucketStart(now)
	points := BuildPoints(now, []Aggregate{
		{
			BucketStart:  bucket,
			SuccessCount: 2,
			FailureCount: 1,
			LatencyTotal: 450,
			LatencyCount: 3,
		},
	})
	if len(points) != BucketCount {
		t.Fatalf("points len=%d, want %d", len(points), BucketCount)
	}
	latest := points[len(points)-1]
	if latest.BucketStart != bucket {
		t.Fatalf("latest bucket start=%d, want %d", latest.BucketStart, bucket)
	}
	if latest.State != StateWarning {
		t.Fatalf("latest state=%q, want %q", latest.State, StateWarning)
	}
	if latest.SuccessCount != 2 || latest.FailureCount != 1 || latest.TotalCount != 3 {
		t.Fatalf("latest counts=%d/%d/%d, want 2/1/3", latest.SuccessCount, latest.FailureCount, latest.TotalCount)
	}
	if latest.AvgLatencyMs != 150 {
		t.Fatalf("latest avg latency=%d, want 150", latest.AvgLatencyMs)
	}
	if latest.PassRate < 0.6666 || latest.PassRate > 0.6667 {
		t.Fatalf("latest pass rate=%f, want about 0.6667", latest.PassRate)
	}
	if points[0].State != StateUnknown || points[0].TotalCount != 0 {
		t.Fatalf("first point=%+v, want empty unknown bucket", points[0])
	}
}

func TestSummarizePointsUsesObservedBuckets(t *testing.T) {
	now := int64(1700001800)
	points := BuildPoints(now, []Aggregate{
		{BucketStart: BucketStart(now) - BucketIntervalSeconds, SuccessCount: 1, LatencyTotal: 100, LatencyCount: 1},
		{BucketStart: BucketStart(now), FailureCount: 2},
	})
	summary := Summarize(points)
	if summary.SuccessCount != 1 || summary.FailureCount != 2 || summary.TotalCount != 3 {
		t.Fatalf("summary counts=%d/%d/%d, want 1/2/3", summary.SuccessCount, summary.FailureCount, summary.TotalCount)
	}
	if summary.ObservedBucketCount != 2 {
		t.Fatalf("observed buckets=%d, want 2", summary.ObservedBucketCount)
	}
	if summary.AvgLatencyMs != 100 {
		t.Fatalf("summary avg latency=%d, want 100", summary.AvgLatencyMs)
	}
	if summary.LastObservedAt != BucketStart(now)+BucketIntervalSeconds-1 {
		t.Fatalf("last observed=%d, want latest bucket end", summary.LastObservedAt)
	}
}
