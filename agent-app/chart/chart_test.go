package chart

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChartValueAndScan(t *testing.T) {
	source := Chart{
		ChartType: TypeLine,
		Title:     "趋势",
		XAxis:     []string{"2026-06-26"},
		YAxis:     &AxisConfig{ValueFormat: ValueFormatDurationMS},
		Series: []ChartSeries{
			{Name: "耗时", Data: []interface{}{123}},
		},
	}
	value, err := source.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(value.([]byte), &raw); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if raw["widget_type"] != "chart" || raw["data_type"] != "chart" {
		t.Fatalf("raw identifiers = %#v", raw)
	}
	yAxis, ok := raw["y_axis"].(map[string]interface{})
	if !ok || yAxis["value_format"] != ValueFormatDurationMS {
		t.Fatalf("raw y_axis = %#v, want value_format=%q", raw["y_axis"], ValueFormatDurationMS)
	}

	var scanned Chart
	if err := scanned.Scan(value); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if scanned.ChartType != TypeLine || scanned.Title != "趋势" || len(scanned.Series) != 1 || scanned.YAxis == nil || scanned.YAxis.ValueFormat != ValueFormatDurationMS {
		t.Fatalf("scanned chart = %#v", scanned)
	}
}

func TestValidateAxisChart(t *testing.T) {
	warnings := Validate(&LineChart{
		XAxis: []string{"a", "b"},
		Series: []ChartSeries{
			{Name: "series", Data: []interface{}{1}},
		},
	})
	if !containsWarning(warnings, "data 长度是 1") {
		t.Fatalf("Validate() = %#v, want length mismatch warning", warnings)
	}

	warnings = Validate(&LineChart{
		XAxis: []string{"a"},
		YAxis: &AxisConfig{ValueFormat: "duration_seconds"},
		Series: []ChartSeries{
			{Name: "series", Data: []interface{}{1}},
		},
	})
	if !containsWarning(warnings, "y_axis.value_format") {
		t.Fatalf("Validate() = %#v, want y_axis.value_format warning", warnings)
	}
}

func TestValidateAllowsEmptyChartWithEmptyText(t *testing.T) {
	if warnings := Validate(&LineChart{EmptyText: "暂无数据"}); len(warnings) != 0 {
		t.Fatalf("Validate() warnings = %#v, want empty", warnings)
	}
}

func TestValidatePieChart(t *testing.T) {
	warnings := Validate(&PieChart{
		Series: []ChartSeries{
			{Name: "状态码", Data: []interface{}{map[string]interface{}{"name": "200", "value": 3}}},
		},
	})
	if len(warnings) != 0 {
		t.Fatalf("Validate() warnings = %#v, want empty", warnings)
	}

	warnings = Validate(&PieChart{
		Series: []ChartSeries{
			{Name: "状态码", Data: []interface{}{map[string]interface{}{"name": "bad", "value": "3"}}},
		},
	})
	if !containsWarning(warnings, "value 不是数字") {
		t.Fatalf("Validate() = %#v, want pie value warning", warnings)
	}
}

func TestValidateGaugeChart(t *testing.T) {
	warnings := Validate(&GaugeChart{
		Series: []ChartSeries{
			{Name: "成功率", Data: []interface{}{99.5}},
		},
	})
	if len(warnings) != 0 {
		t.Fatalf("Validate() warnings = %#v, want empty", warnings)
	}

	warnings = Validate(&GaugeChart{
		Series: []ChartSeries{
			{Name: "成功率", Data: []interface{}{99.5, 98.1}},
		},
	})
	if !containsWarning(warnings, "exactly one data point") {
		t.Fatalf("Validate() = %#v, want gauge data point warning", warnings)
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
