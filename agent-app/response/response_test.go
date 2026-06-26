package response

import (
	"strings"
	"testing"

	"github.com/kageos/kageos-sdk/agent-app/chart"
	"github.com/kageos/kageos-sdk/pkg/gormx/query"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type tableQueryTestItem struct {
	ID       int `gorm:"primaryKey"`
	Name     string
	Category string
	Score    int
}

func setupTableQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&tableQueryTestItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	items := []tableQueryTestItem{
		{Name: "alpha", Category: "a", Score: 50},
		{Name: "beta", Category: "b", Score: 40},
		{Name: "gamma", Category: "a", Score: 30},
		{Name: "delta", Category: "b", Score: 20},
		{Name: "epsilon", Category: "a", Score: 10},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func TestTableQueryDefaultsToFirstPage(t *testing.T) {
	db := setupTableQueryTestDB(t)
	var rows []tableQueryTestItem
	var total int64
	if err := db.Model(&tableQueryTestItem{}).Count(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	pageInfo := &query.PageSortReq{}
	if err := db.Model(&tableQueryTestItem{}).
		Offset(pageInfo.GetOffset()).
		Limit(pageInfo.GetLimit()).
		Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}

	resp := (&RunFunctionResp{}).Table(TableResult{
		Items:      rows,
		TotalCount: total,
		PageInfo:   pageInfo,
	})
	if err := resp.Build(); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(rows) != 5 {
		t.Fatalf("len(rows) = %d, want 5", len(rows))
	}
	got := resp.(*RunFunctionResp).TableData.Paginated
	if got == nil {
		t.Fatal("Paginated = nil")
	}
	if got.CurrentPage != 1 || got.PageSize != 20 || got.TotalCount != 5 || got.TotalPages != 1 {
		t.Fatalf("Paginated = %#v", got)
	}
}

func TestTableQuerySortsAndPaginates(t *testing.T) {
	db := setupTableQueryTestDB(t)
	var rows []tableQueryTestItem

	pageInfo := &query.PageSortReq{Page: 2, PageSize: 2, Sorts: "-score"}
	queryDB := db.Model(&tableQueryTestItem{})
	var total int64
	if err := queryDB.Count(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if order := pageInfo.GetOrder(); order != "" {
		queryDB = queryDB.Order(order)
	}
	if err := queryDB.Offset(pageInfo.GetOffset()).Limit(pageInfo.GetLimit()).Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	resp := (&RunFunctionResp{}).Table(TableResult{
		Items:      rows,
		TotalCount: total,
		PageInfo:   pageInfo,
	})
	if err := resp.Build(); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Name != "gamma" || rows[1].Name != "delta" {
		t.Fatalf("rows = %#v", rows)
	}
	got := resp.(*RunFunctionResp).TableData.Paginated
	if got.CurrentPage != 2 || got.PageSize != 2 || got.TotalCount != 5 || got.TotalPages != 3 {
		t.Fatalf("Paginated = %#v", got)
	}
}

func TestTableQueryUsesCallerWhereWithoutSearchProtocol(t *testing.T) {
	db := setupTableQueryTestDB(t)
	var rows []tableQueryTestItem

	queryDB := db.Model(&tableQueryTestItem{}).Where("category = ?", "missing")
	pageInfo := &query.PageSortReq{PageSize: 2}
	var total int64
	if err := queryDB.Count(&total).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if err := queryDB.Offset(pageInfo.GetOffset()).Limit(pageInfo.GetLimit()).Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	resp := (&RunFunctionResp{}).Table(TableResult{
		Items:      rows,
		TotalCount: total,
		PageInfo:   pageInfo,
	})
	if err := resp.Build(); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(rows) != 0 {
		t.Fatalf("len(rows) = %d, want 0", len(rows))
	}
	got := resp.(*RunFunctionResp).TableData.Paginated
	if got.TotalCount != 0 || got.TotalPages != 0 || got.PageSize != 2 {
		t.Fatalf("Paginated = %#v", got)
	}
}

func TestChartPreservesExplicitSeriesType(t *testing.T) {
	c := &chart.Chart{
		ChartType: chart.TypeLine,
		XAxis:     []string{"a"},
		Series: []chart.ChartSeries{
			{Name: "bar data", Type: chart.TypeBar, Data: []interface{}{1}},
			{Name: "line data", Data: []interface{}{2}},
		},
	}
	resp := (&RunFunctionResp{}).Chart(c)
	if err := resp.Build(); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if c.Series[0].Type != chart.TypeBar {
		t.Fatalf("explicit series type overwritten: %q", c.Series[0].Type)
	}
	if c.Series[1].Type != chart.TypeLine {
		t.Fatalf("empty series type = %q, want %q", c.Series[1].Type, chart.TypeLine)
	}
}

func TestChartWarnsNilAndEmptyType(t *testing.T) {
	nilResp := (&RunFunctionResp{}).Chart(nil)
	if err := nilResp.Build(); err != nil {
		t.Fatalf("nil chart Build() error = %v, want nil", err)
	}
	nilData := nilResp.(*RunFunctionResp).ChartData
	if nilData == nil || !containsWarning(nilData.Warnings, "chart 为空") {
		t.Fatalf("nil chart warnings = %#v, want chart empty warning", nilData)
	}

	emptyResp := (&RunFunctionResp{}).Chart(&chart.Chart{})
	if err := emptyResp.Build(); err != nil {
		t.Fatalf("empty chart Build() error = %v, want nil", err)
	}
	emptyData := emptyResp.(*RunFunctionResp).ChartData
	if emptyData == nil || !containsWarning(emptyData.Warnings, "chart_type 为空") {
		t.Fatalf("empty chart warnings = %#v, want chart_type warning", emptyData)
	}
}

func TestChartWarnsTemplateTypeMismatch(t *testing.T) {
	resp := (&RunFunctionResp{ExpectedChartType: chart.TypeBar}).Chart(&chart.LineChart{
		XAxis: []string{"a"},
		Series: []chart.ChartSeries{
			{Name: "line data", Data: []interface{}{1}},
		},
	})
	if err := resp.Build(); err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	data := resp.(*RunFunctionResp).ChartData
	if data == nil || !containsWarning(data.Warnings, "不一致") {
		t.Fatalf("chart mismatch warnings = %#v, want mismatch warning", data)
	}
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
