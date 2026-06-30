package app

import (
	"testing"
	"time"
)

func TestResolveChartBucketKeepsRecommendedBucketWithoutBudget(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 26, 15, 4, 5, 0, time.Local)
	decision := ResolveChartBucket(ChartBucketPolicy{
		Requested:   TimeBucketAuto,
		WindowStart: now.Add(-24 * time.Hour),
		WindowEnd:   now,
		SeriesCount: 10,
	})

	if decision.Bucket != TimeBucket5Minute {
		t.Fatalf("Bucket = %q, want %q", decision.Bucket, TimeBucket5Minute)
	}
	if decision.Coarsened {
		t.Fatalf("Coarsened = true, want false")
	}
	if decision.MaxValues != 0 {
		t.Fatalf("MaxValues = %d, want 0", decision.MaxValues)
	}
}

func TestResolveChartBucketCoarsensMultiSeriesDayWindowWithBudget(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 26, 15, 4, 5, 0, time.Local)
	decision := ResolveChartBucket(ChartBucketPolicy{
		Requested:   TimeBucketAuto,
		WindowStart: now.Add(-24 * time.Hour),
		WindowEnd:   now,
		SeriesCount: 10,
		MaxValues:   600,
	})

	if decision.InitialBucket != TimeBucket5Minute {
		t.Fatalf("InitialBucket = %q, want %q", decision.InitialBucket, TimeBucket5Minute)
	}
	if decision.Bucket != TimeBucketHour {
		t.Fatalf("Bucket = %q, want %q", decision.Bucket, TimeBucketHour)
	}
	if !decision.Coarsened {
		t.Fatalf("Coarsened = false, want true")
	}
	if decision.EstimatedValues > decision.MaxValues {
		t.Fatalf("EstimatedValues = %d should be <= MaxValues = %d", decision.EstimatedValues, decision.MaxValues)
	}
}

func TestResolveChartBucketKeepsSelectedTargetDetail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 26, 15, 4, 5, 0, time.Local)
	decision := ResolveChartBucket(ChartBucketPolicy{
		Requested:   TimeBucketAuto,
		WindowStart: now.Add(-24 * time.Hour),
		WindowEnd:   now,
		SeriesCount: 2,
	})

	if decision.Bucket != TimeBucket5Minute {
		t.Fatalf("Bucket = %q, want %q", decision.Bucket, TimeBucket5Minute)
	}
	if decision.Coarsened {
		t.Fatalf("Coarsened = true, want false")
	}
}

func TestResolveChartBucketKeepsSevenDayMultiSeriesWithoutBudget(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 26, 15, 4, 5, 0, time.Local)
	decision := ResolveChartBucket(ChartBucketPolicy{
		Requested:   TimeBucketAuto,
		WindowStart: now.AddDate(0, 0, -7),
		WindowEnd:   now,
		SeriesCount: 10,
	})

	if decision.Bucket != TimeBucketHour {
		t.Fatalf("Bucket = %q, want %q", decision.Bucket, TimeBucketHour)
	}
	if decision.Coarsened {
		t.Fatalf("Coarsened = true, want false")
	}
}

func TestResolveChartBucketCoarsensSevenDayMultiSeriesWithBudget(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 26, 15, 4, 5, 0, time.Local)
	decision := ResolveChartBucket(ChartBucketPolicy{
		Requested:   TimeBucketAuto,
		WindowStart: now.AddDate(0, 0, -7),
		WindowEnd:   now,
		SeriesCount: 10,
		MaxValues:   600,
	})

	if decision.InitialBucket != TimeBucketHour {
		t.Fatalf("InitialBucket = %q, want %q", decision.InitialBucket, TimeBucketHour)
	}
	if decision.Bucket != TimeBucketDay {
		t.Fatalf("Bucket = %q, want %q", decision.Bucket, TimeBucketDay)
	}
}

func TestResolveChartBucketHonorsExplicitCoarseBucket(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 26, 15, 4, 5, 0, time.Local)
	decision := ResolveChartBucket(ChartBucketPolicy{
		Requested:   TimeBucketDay,
		WindowStart: now.Add(-6 * time.Hour),
		WindowEnd:   now,
		SeriesCount: 10,
		MaxValues:   600,
	})

	if decision.Bucket != TimeBucketDay {
		t.Fatalf("Bucket = %q, want %q", decision.Bucket, TimeBucketDay)
	}
	if decision.Coarsened {
		t.Fatalf("Coarsened = true, want false")
	}
}

func TestChartBucketMetadataIncludesCoarseningContext(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 26, 15, 4, 5, 0, time.Local)
	decision := ResolveChartBucket(ChartBucketPolicy{
		Requested:   TimeBucketAuto,
		WindowStart: now.Add(-24 * time.Hour),
		WindowEnd:   now,
		SeriesCount: 10,
		MaxValues:   600,
	})
	metadata := ChartBucketMetadata(decision)

	if metadata["聚合粒度"] != "按小时" {
		t.Fatalf("聚合粒度 = %#v, want 按小时", metadata["聚合粒度"])
	}
	if metadata["自动放粗"] != true {
		t.Fatalf("自动放粗 = %#v, want true", metadata["自动放粗"])
	}
	if metadata["原始聚合粒度"] != "按5分钟" {
		t.Fatalf("原始聚合粒度 = %#v, want 按5分钟", metadata["原始聚合粒度"])
	}
}
