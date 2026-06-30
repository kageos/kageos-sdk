package app

import (
	"strings"

	"gorm.io/gorm"
)

type DBDialect string

const (
	DBDialectMySQL   DBDialect = "mysql"
	DBDialectUnknown DBDialect = "unknown"
)

type TimeBucket string

const (
	TimeBucketAuto    TimeBucket = "auto"
	TimeBucketMinute  TimeBucket = "minute"
	TimeBucket5Minute TimeBucket = "5_minute"
	TimeBucketHour    TimeBucket = "hour"
	TimeBucketDay     TimeBucket = "day"
	TimeBucketMonth   TimeBucket = "month"
)

// DetectDBDialect returns the normalized dialect name for the current GORM DB.
func DetectDBDialect(db *gorm.DB) DBDialect {
	if db == nil || db.Dialector == nil {
		return DBDialectUnknown
	}
	return detectDBDialectName(db.Dialector.Name())
}

// DateTimeBucketExpr returns SQL expressions for grouping a native datetime column by time bucket.
func DateTimeBucketExpr(db *gorm.DB, column string, bucket TimeBucket) (selectExpr, groupExpr string) {
	return DateTimeBucketExprForDialect(DetectDBDialect(db), column, bucket)
}

// DateTimeBucketExprForDialect returns MySQL expressions for the runtime-managed app business DB.
// The dialect argument is kept for source compatibility; Kageos app business DB is MySQL-only.
func DateTimeBucketExprForDialect(_ DBDialect, column string, bucket TimeBucket) (selectExpr, groupExpr string) {
	column = strings.TrimSpace(column)
	if column == "" {
		column = "created_at"
	}

	var expr string
	switch normalizeTimeBucket(bucket) {
	case TimeBucketMinute:
		expr = "DATE_FORMAT(" + column + ", '%Y-%m-%d %H:%i:00')"
	case TimeBucket5Minute:
		expr = "FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(" + column + ") / 300) * 300, '%Y-%m-%d %H:%i:00')"
	case TimeBucketHour:
		expr = "DATE_FORMAT(" + column + ", '%Y-%m-%d %H:00:00')"
	case TimeBucketMonth:
		expr = "DATE_FORMAT(" + column + ", '%Y-%m')"
	default:
		expr = "DATE_FORMAT(" + column + ", '%Y-%m-%d')"
	}

	return expr, expr
}

func detectDBDialectName(name string) DBDialect {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case string(DBDialectMySQL):
		return DBDialectMySQL
	default:
		return DBDialectUnknown
	}
}

func normalizeTimeBucket(bucket TimeBucket) TimeBucket {
	switch TimeBucket(strings.ToLower(strings.TrimSpace(string(bucket)))) {
	case TimeBucketMinute, "min":
		return TimeBucketMinute
	case TimeBucket5Minute, "5minute", "5_min", "5min", "5m":
		return TimeBucket5Minute
	case TimeBucketHour:
		return TimeBucketHour
	case TimeBucketMonth:
		return TimeBucketMonth
	default:
		return TimeBucketDay
	}
}
