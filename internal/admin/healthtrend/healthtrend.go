package healthtrend

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	BucketIntervalSeconds int64 = 5 * 60
	BucketCount                 = 60

	StateSuccess = "success"
	StateWarning = "warning"
	StateFailure = "failure"
	StateUnknown = "unknown"
)

type Aggregate struct {
	BucketStart    int64
	SuccessCount   int64
	FailureCount   int64
	LatencyTotal   int64
	LatencyCount   int64
	LastObservedAt int64
}

type Point struct {
	State          string  `json:"state"`
	BucketStart    int64   `json:"bucket_start"`
	BucketEnd      int64   `json:"bucket_end"`
	SuccessCount   int64   `json:"success_count"`
	FailureCount   int64   `json:"failure_count"`
	TotalCount     int64   `json:"total_count"`
	AvgLatencyMs   int64   `json:"avg_latency_ms"`
	LatencyCount   int64   `json:"latency_count"`
	PassRate       float64 `json:"pass_rate"`
	LastObservedAt int64   `json:"last_observed_at"`
}

type Summary struct {
	SuccessCount        int64
	FailureCount        int64
	TotalCount          int64
	AvgLatencyMs        int64
	LastObservedAt      int64
	ObservedBucketCount int
}

func BucketStart(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	return ts - ts%BucketIntervalSeconds
}

func WindowStart(now int64) int64 {
	current := BucketStart(now)
	if current <= 0 {
		return 0
	}
	return current - int64(BucketCount-1)*BucketIntervalSeconds
}

func SQLBucketExpression(db *gorm.DB, column string) string {
	return SQLBucketExpressionForInterval(db, column, BucketIntervalSeconds)
}

func SQLBucketExpressionForInterval(db *gorm.DB, column string, intervalSeconds int64) string {
	normalizedColumn := strings.TrimSpace(column)
	if normalizedColumn == "" {
		normalizedColumn = "created_at"
	}
	if intervalSeconds <= 0 {
		intervalSeconds = BucketIntervalSeconds
	}
	switch strings.ToLower(strings.TrimSpace(dbDialectName(db))) {
	case "mysql":
		return fmt.Sprintf("(FLOOR(%s / %d) * %d)", normalizedColumn, intervalSeconds, intervalSeconds)
	case "sqlite":
		return fmt.Sprintf("(CAST(%s / %d AS INTEGER) * %d)", normalizedColumn, intervalSeconds, intervalSeconds)
	default:
		return fmt.Sprintf("((%s / %d) * %d)", normalizedColumn, intervalSeconds, intervalSeconds)
	}
}

func dbDialectName(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return ""
	}
	return db.Dialector.Name()
}

func BuildPoints(now int64, rows []Aggregate) []Point {
	start := WindowStart(now)
	end := BucketStart(now) + BucketIntervalSeconds - 1
	return BuildPointsForWindow(start, end, BucketIntervalSeconds, rows)
}

func BuildPointsForWindow(startAt int64, endAt int64, intervalSeconds int64, rows []Aggregate) []Point {
	if startAt <= 0 || endAt < startAt {
		return []Point{}
	}
	if intervalSeconds <= 0 {
		intervalSeconds = BucketIntervalSeconds
	}
	start := startAt - startAt%intervalSeconds
	end := endAt - endAt%intervalSeconds
	count := int((end-start)/intervalSeconds) + 1
	if count <= 0 {
		return []Point{}
	}
	points := make([]Point, 0, count)
	latencyCounts := make([]int64, count)
	for i := 0; i < count; i++ {
		bucketStart := start + int64(i)*intervalSeconds
		points = append(points, Point{
			State:       StateUnknown,
			BucketStart: bucketStart,
			BucketEnd:   bucketStart + intervalSeconds - 1,
		})
	}
	for _, row := range rows {
		bucketStart := row.BucketStart - row.BucketStart%intervalSeconds
		index := int((bucketStart - start) / intervalSeconds)
		if index < 0 || index >= len(points) {
			continue
		}
		points[index].SuccessCount += row.SuccessCount
		points[index].FailureCount += row.FailureCount
		points[index].TotalCount = points[index].SuccessCount + points[index].FailureCount
		if row.LatencyTotal > 0 && row.LatencyCount > 0 {
			points[index].AvgLatencyMs += row.LatencyTotal
			latencyCounts[index] += row.LatencyCount
		}
		if row.LastObservedAt > points[index].LastObservedAt {
			points[index].LastObservedAt = row.LastObservedAt
		}
	}
	for i := range points {
		if latencyCounts[i] > 0 {
			points[i].AvgLatencyMs = points[i].AvgLatencyMs / latencyCounts[i]
			points[i].LatencyCount = latencyCounts[i]
		}
		points[i].State = StateForCounts(points[i].SuccessCount, points[i].FailureCount)
		if points[i].TotalCount > 0 {
			points[i].PassRate = float64(points[i].SuccessCount) / float64(points[i].TotalCount)
			// Older aggregate callers identify only their five-minute bucket.
			// New callers report the precise final observation timestamp.
			if points[i].LastObservedAt <= 0 {
				points[i].LastObservedAt = points[i].BucketEnd
			}
		}
	}
	return points
}

func StateForCounts(successCount int64, failureCount int64) string {
	totalCount := successCount + failureCount
	switch {
	case totalCount <= 0:
		return StateUnknown
	case failureCount <= 0:
		return StateSuccess
	case successCount > 0:
		return StateWarning
	default:
		return StateFailure
	}
}

func Summarize(points []Point) Summary {
	summary := Summary{}
	latencyTotal := int64(0)
	latencyCount := int64(0)
	for _, point := range points {
		summary.SuccessCount += point.SuccessCount
		summary.FailureCount += point.FailureCount
		summary.TotalCount += point.TotalCount
		if point.TotalCount > 0 {
			summary.ObservedBucketCount++
			if point.LastObservedAt > summary.LastObservedAt {
				summary.LastObservedAt = point.LastObservedAt
			}
		}
		if point.AvgLatencyMs > 0 && point.LatencyCount > 0 {
			latencyTotal += point.AvgLatencyMs * point.LatencyCount
			latencyCount += point.LatencyCount
		}
	}
	if latencyCount > 0 {
		summary.AvgLatencyMs = latencyTotal / latencyCount
	}
	return summary
}
