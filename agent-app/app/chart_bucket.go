package app

import (
	"math"
	"strings"
	"time"
)

type ChartBucketPolicy struct {
	Requested   TimeBucket
	WindowStart time.Time
	WindowEnd   time.Time
	SeriesCount int
	MaxValues   int
	Candidates  []TimeBucket
}

type ChartBucketDecision struct {
	Requested               TimeBucket
	InitialBucket           TimeBucket
	Bucket                  TimeBucket
	SeriesCount             int
	MaxValues               int
	InitialEstimatedBuckets int
	InitialEstimatedValues  int
	EstimatedBuckets        int
	EstimatedValues         int
	Coarsened               bool
}

func ResolveChartBucket(policy ChartBucketPolicy) ChartBucketDecision {
	seriesCount := policy.SeriesCount
	if seriesCount <= 0 {
		seriesCount = 1
	}
	maxValues := policy.MaxValues
	candidates := normalizeChartBucketCandidates(policy.Candidates)
	requested := normalizeChartRequestedBucket(policy.Requested)
	initialBucket := requested
	if initialBucket == TimeBucketAuto {
		initialBucket = RecommendChartTimeBucket(policy.WindowStart, policy.WindowEnd)
	}
	initialIndex := chartBucketIndex(candidates, initialBucket)
	if initialIndex < 0 {
		initialIndex = 0
		initialBucket = candidates[initialIndex]
	}

	initialEstimatedBuckets := EstimateChartBucketCount(policy.WindowStart, policy.WindowEnd, initialBucket)
	initialEstimatedValues := initialEstimatedBuckets * seriesCount
	bucketIndex := initialIndex
	if maxValues > 0 {
		for bucketIndex < len(candidates)-1 {
			estimatedValues := EstimateChartBucketCount(policy.WindowStart, policy.WindowEnd, candidates[bucketIndex]) * seriesCount
			if estimatedValues <= maxValues {
				break
			}
			bucketIndex++
		}
	}

	bucket := candidates[bucketIndex]
	estimatedBuckets := EstimateChartBucketCount(policy.WindowStart, policy.WindowEnd, bucket)
	estimatedValues := estimatedBuckets * seriesCount
	return ChartBucketDecision{
		Requested:               requested,
		InitialBucket:           initialBucket,
		Bucket:                  bucket,
		SeriesCount:             seriesCount,
		MaxValues:               maxValues,
		InitialEstimatedBuckets: initialEstimatedBuckets,
		InitialEstimatedValues:  initialEstimatedValues,
		EstimatedBuckets:        estimatedBuckets,
		EstimatedValues:         estimatedValues,
		Coarsened:               bucket != initialBucket,
	}
}

func RecommendChartTimeBucket(start, end time.Time) TimeBucket {
	duration := end.Sub(start)
	switch {
	case duration <= 2*time.Hour:
		return TimeBucketMinute
	case duration <= 24*time.Hour:
		return TimeBucket5Minute
	case duration <= 14*24*time.Hour:
		return TimeBucketHour
	case duration <= 120*24*time.Hour:
		return TimeBucketDay
	default:
		return TimeBucketMonth
	}
}

func EstimateChartBucketCount(start, end time.Time, bucket TimeBucket) int {
	duration := end.Sub(start)
	if duration <= 0 {
		return 1
	}
	switch normalizeTimeBucket(bucket) {
	case TimeBucketMinute:
		return int(math.Ceil(duration.Minutes())) + 1
	case TimeBucket5Minute:
		return int(math.Ceil(float64(duration)/float64(5*time.Minute))) + 1
	case TimeBucketHour:
		return int(math.Ceil(duration.Hours())) + 1
	case TimeBucketDay:
		return int(math.Ceil(duration.Hours()/24)) + 1
	case TimeBucketMonth:
		startMonth := start.Year()*12 + int(start.Month())
		endMonth := end.Year()*12 + int(end.Month())
		if endMonth < startMonth {
			return 1
		}
		return endMonth - startMonth + 1
	default:
		return 1
	}
}

func ChartBucketMetadata(decision ChartBucketDecision) map[string]interface{} {
	metadata := map[string]interface{}{
		"聚合粒度":  TimeBucketLabel(decision.Bucket),
		"估算时间桶": decision.EstimatedBuckets,
		"估算图表值": decision.EstimatedValues,
		"自动放粗":  decision.Coarsened,
	}
	if decision.MaxValues > 0 {
		metadata["最大图表值"] = decision.MaxValues
	}
	if decision.Coarsened {
		metadata["原始聚合粒度"] = TimeBucketLabel(decision.InitialBucket)
		metadata["原始估算图表值"] = decision.InitialEstimatedValues
	}
	return metadata
}

func TimeBucketLabel(bucket TimeBucket) string {
	value := strings.TrimSpace(string(bucket))
	normalized := normalizeChartRequestedBucket(bucket)
	if normalized == TimeBucketAuto && value != "" && strings.ToLower(value) != string(TimeBucketAuto) {
		return value
	}
	switch normalized {
	case TimeBucketAuto:
		return "自动"
	case TimeBucketMinute:
		return "按分钟"
	case TimeBucket5Minute:
		return "按5分钟"
	case TimeBucketHour:
		return "按小时"
	case TimeBucketDay:
		return "按天"
	case TimeBucketMonth:
		return "按月"
	default:
		return strings.TrimSpace(string(bucket))
	}
}

func normalizeChartBucketCandidates(candidates []TimeBucket) []TimeBucket {
	if len(candidates) == 0 {
		return []TimeBucket{TimeBucketMinute, TimeBucket5Minute, TimeBucketHour, TimeBucketDay, TimeBucketMonth}
	}
	normalized := make([]TimeBucket, 0, len(candidates))
	seen := map[TimeBucket]bool{}
	for _, candidate := range candidates {
		bucket := normalizeChartRequestedBucket(candidate)
		if bucket == TimeBucketAuto || seen[bucket] {
			continue
		}
		seen[bucket] = true
		normalized = append(normalized, bucket)
	}
	if len(normalized) == 0 {
		return []TimeBucket{TimeBucketMinute, TimeBucket5Minute, TimeBucketHour, TimeBucketDay, TimeBucketMonth}
	}
	return normalized
}

func normalizeChartRequestedBucket(bucket TimeBucket) TimeBucket {
	value := strings.ToLower(strings.TrimSpace(string(bucket)))
	switch TimeBucket(value) {
	case "", TimeBucketAuto:
		return TimeBucketAuto
	case TimeBucketMinute, "min":
		return TimeBucketMinute
	case TimeBucket5Minute, "5minute", "5_min", "5min", "5m":
		return TimeBucket5Minute
	case TimeBucketHour:
		return TimeBucketHour
	case TimeBucketDay:
		return TimeBucketDay
	case TimeBucketMonth:
		return TimeBucketMonth
	default:
		return TimeBucketAuto
	}
}

func chartBucketIndex(candidates []TimeBucket, bucket TimeBucket) int {
	for i, candidate := range candidates {
		if candidate == bucket {
			return i
		}
	}
	return -1
}
