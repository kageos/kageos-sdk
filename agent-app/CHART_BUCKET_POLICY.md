# Chart Time Bucket Policy

图表接口经常需要在“看清短时间波动”和“避免一次返回过多点位”之间取平衡。SDK 提供了一组通用 helper，用来统一处理时间粒度推荐、点位估算、可选的自动放粗，以及 SQL 时间分桶表达式。

这组能力只服务图表响应，不会改变表格、搜索、明细查询的过滤逻辑。

## 核心 API

```go
app.ResolveChartBucket(policy app.ChartBucketPolicy) app.ChartBucketDecision
app.RecommendChartTimeBucket(start, end time.Time) app.TimeBucket
app.EstimateChartBucketCount(start, end time.Time, bucket app.TimeBucket) int
app.DateTimeBucketExpr(db, column, bucket) (selectExpr, groupExpr string)
app.ChartBucketMetadata(decision) map[string]interface{}
```

支持的粒度：

```go
app.TimeBucketAuto
app.TimeBucketMinute
app.TimeBucket5Minute
app.TimeBucketHour
app.TimeBucketDay
app.TimeBucketMonth
```

## 默认行为

`ResolveChartBucket` 默认只做推荐和估算，不会硬性禁止细粒度。

如果 `ChartBucketPolicy.MaxValues <= 0`，SDK 会根据时间窗口选择一个推荐粒度，并保留这个粒度返回。也就是说，即使估算点位较多，也不会自动放粗。

默认推荐规则：

| 时间窗口 | 推荐粒度 |
| --- | --- |
| `<= 2h` | `minute` |
| `<= 24h` | `5_minute` |
| `<= 14d` | `hour` |
| `<= 120d` | `day` |
| `> 120d` | `month` |

这样短窗口默认能保留比较清晰的波动，长窗口默认避免无意义的超细粒度。

## 可选前端保护

如果某个图表可能返回太多点，业务可以传 `MaxValues` 打开自动放粗：

```go
decision := app.ResolveChartBucket(app.ChartBucketPolicy{
	Requested:   app.TimeBucketAuto,
	WindowStart: start,
	WindowEnd:   end,
	SeriesCount: len(seriesNames),
	MaxValues:   1200,
})
```

估算值按 `时间桶数量 * SeriesCount` 计算。开启 `MaxValues` 后，SDK 会从当前粒度开始，按候选粒度逐级放粗，直到估算图表值不超过预算，或已经到达最粗粒度。

如果业务明确允许大数据量细粒度展示，不传 `MaxValues` 即可。

## 用户指定粒度

前端可以把粒度参数传给业务接口，业务再映射成 SDK 粒度：

```go
func requestedBucket(value string) app.TimeBucket {
	switch strings.TrimSpace(value) {
	case "按分钟":
		return app.TimeBucketMinute
	case "按5分钟":
		return app.TimeBucket5Minute
	case "按小时":
		return app.TimeBucketHour
	case "按天":
		return app.TimeBucketDay
	case "按月":
		return app.TimeBucketMonth
	default:
		return app.TimeBucketAuto
	}
}
```

显式传入较粗粒度时，SDK 会尊重用户选择。传入未知值时，会按 `auto` 处理。

## 查询示例

```go
decision := app.ResolveChartBucket(app.ChartBucketPolicy{
	Requested:   requestedBucket(req.Bucket),
	WindowStart: window.Start,
	WindowEnd:   window.End,
	SeriesCount: len(targets),
})

selectExpr, groupExpr := app.DateTimeBucketExpr(db, "checked_at", decision.Bucket)

err := db.Model(&CheckLog{}).
	Select(selectExpr+" AS time_bucket, target_id, AVG(latency_ms) AS avg_latency_ms").
	Where("checked_at >= ? AND checked_at <= ?", window.Start, window.End).
	Group(groupExpr + ", target_id").
	Order("time_bucket ASC").
	Scan(&rows).Error
```

返回图表时建议合并 SDK 元数据，方便前端或排查时知道实际粒度和估算规模：

```go
metadata := map[string]interface{}{
	"时间范围": window.Label,
}
for key, value := range app.ChartBucketMetadata(decision) {
	metadata[key] = value
}
```

## 使用建议

监控、价格、行情等趋势图，默认时间范围宜短一些，例如最近 1 天，并使用 `auto` 粒度。这样用户打开默认图表时能看到细波动。

长时间窗口、很多 series、或者默认会展示大量对象的图表，可以传 `MaxValues` 做保护。是否保护由业务场景决定，SDK 不默认替业务硬拦。

表格搜索、明细查询、导出类能力不要复用这个粒度决策来裁剪数据。它们应该继续使用自己的分页、过滤、导出限制。
