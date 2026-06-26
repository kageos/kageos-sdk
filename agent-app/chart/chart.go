package chart

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const (
	TypeBar   = "bar"
	TypeLine  = "line"
	TypePie   = "pie"
	TypeGauge = "gauge"

	// ValueFormatCompact keeps the default frontend behavior for generic numbers:
	// large axis labels are abbreviated as K/M.
	ValueFormatCompact = "compact"
	// ValueFormatPlain renders numeric values without K/M abbreviation.
	ValueFormatPlain = "plain"
	// ValueFormatDurationMS means series data is stored in milliseconds; the
	// frontend formats labels and tooltips as ms/s/min without changing data.
	ValueFormatDurationMS = "duration_ms"
	// ValueFormatPercent appends % to axis labels and tooltips.
	ValueFormatPercent = "percent"
)

// AxisConfig describes how a numeric axis should be displayed by frontend chart
// renderers. Keep raw series data in its business unit and declare ValueFormat
// here instead of encoding display rules in Metadata or series names.
type AxisConfig struct {
	// ValueFormat controls value labels, crosshair labels, and tooltips.
	// Supported values: chart.ValueFormatCompact, chart.ValueFormatPlain,
	// chart.ValueFormatDurationMS, chart.ValueFormatPercent.
	// Empty value keeps the frontend default, equivalent to ValueFormatCompact.
	ValueFormat string `json:"value_format,omitempty"`
}

// Renderable 图表响应接口：GetChartType 返回类型，SetChartType 由框架调用以注入 ChartType（及 Series.Type），无需反射
type Renderable interface {
	GetChartType() string
	SetChartType(typ string)
	GetSeries() []ChartSeries
	GetXAxis() []string
	GetEmptyText() string
}

// ChartSeries 数据系列
type ChartSeries struct {
	// 系列名称
	Name string `json:"name"`
	// 数据点（必需）
	// - bar/line: []interface{}，如 [100, 200, 150]
	//   支持多系列：例如 3 个状态维度可返回 3 个 ChartSeries，前端会并列渲染为多根柱子或多条折线
	// - pie: []map[string]interface{}，如 [{"name": "A", "value": 100}]
	// - gauge: []interface{}，如 [75]（单值）
	Data []interface{} `json:"data"`
	// Type：由 resp.Chart() 注入，业务无需填
	Type string `json:"type,omitempty"`
	// 系列配置（可选）
	Config map[string]interface{} `json:"config,omitempty"`
}

// Chart 是通用图表结构，适合需要动态 chart_type 或数据库读写图表 JSON 的场景。
type Chart struct {
	// ChartType：必填，代表图表类型。建议使用 app.ChartTypeBar、app.ChartTypeLine 等常量。
	ChartType string                 `json:"chart_type"`
	Title     string                 `json:"title,omitempty"`
	XAxis     []string               `json:"x_axis,omitempty"`
	YAxis     *AxisConfig            `json:"y_axis,omitempty"`
	Series    []ChartSeries          `json:"series"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	EmptyText string                 `json:"empty_text,omitempty"`

	WidgetType string `json:"widget_type,omitempty"`
	DataType   string `json:"data_type,omitempty"`
}

func (c *Chart) GetChartType() string {
	return c.ChartType
}

func (c *Chart) SetChartType(typ string) {
	c.ChartType = typ
	fillEmptySeriesType(c.Series, typ)
}

func (c *Chart) GetSeries() []ChartSeries {
	return c.Series
}

func (c *Chart) GetXAxis() []string {
	return c.XAxis
}

func (c *Chart) GetEmptyText() string {
	return c.EmptyText
}

func (c *Chart) Scan(value interface{}) error {
	if value == nil {
		*c = Chart{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("cannot scan %T into Chart", value)
	}

	return json.Unmarshal(data, c)
}

func (c Chart) Value() (driver.Value, error) {
	c.WidgetType = "chart"
	c.DataType = "chart"
	return json.Marshal(c)
}

// LineChart 折线图
// - XAxis 为横轴分类（常见为日期）
// - Series 支持单条线或多条线；多维趋势对比时，通常一个维度值对应一个 ChartSeries
type LineChart struct {
	ChartType string                 `json:"chart_type"` // 由 resp.Chart() 注入，业务无需填
	Title     string                 `json:"title,omitempty"`
	XAxis     []string               `json:"x_axis,omitempty"`
	YAxis     *AxisConfig            `json:"y_axis,omitempty"`
	Series    []ChartSeries          `json:"series"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	EmptyText string                 `json:"empty_text,omitempty"`
}

func (c *LineChart) GetChartType() string { return TypeLine }
func (c *LineChart) SetChartType(typ string) {
	c.ChartType = typ
	fillEmptySeriesType(c.Series, typ)
}

func (c *LineChart) GetSeries() []ChartSeries { return c.Series }
func (c *LineChart) GetXAxis() []string       { return c.XAxis }
func (c *LineChart) GetEmptyText() string     { return c.EmptyText }

// BarChart 柱状图
// - XAxis 为横轴分类（常见为优先级、部门、月份等）
// - Series 支持单系列或多系列；多维对比时可返回多个 ChartSeries 做分组柱状图
type BarChart struct {
	ChartType string                 `json:"chart_type"`
	Title     string                 `json:"title,omitempty"`
	XAxis     []string               `json:"x_axis,omitempty"`
	YAxis     *AxisConfig            `json:"y_axis,omitempty"`
	Series    []ChartSeries          `json:"series"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	EmptyText string                 `json:"empty_text,omitempty"`
}

func (c *BarChart) GetChartType() string { return TypeBar }
func (c *BarChart) SetChartType(typ string) {
	c.ChartType = typ
	fillEmptySeriesType(c.Series, typ)
}

func (c *BarChart) GetSeries() []ChartSeries { return c.Series }
func (c *BarChart) GetXAxis() []string       { return c.XAxis }
func (c *BarChart) GetEmptyText() string     { return c.EmptyText }

// PieChart 饼图
type PieChart struct {
	ChartType string                 `json:"chart_type"`
	Title     string                 `json:"title,omitempty"`
	Series    []ChartSeries          `json:"series"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	EmptyText string                 `json:"empty_text,omitempty"`
}

func (c *PieChart) GetChartType() string { return TypePie }
func (c *PieChart) SetChartType(typ string) {
	c.ChartType = typ
	fillEmptySeriesType(c.Series, typ)
}

func (c *PieChart) GetSeries() []ChartSeries { return c.Series }
func (c *PieChart) GetXAxis() []string       { return nil }
func (c *PieChart) GetEmptyText() string     { return c.EmptyText }

// GaugeChart 仪表盘
type GaugeChart struct {
	ChartType string                 `json:"chart_type"`
	Title     string                 `json:"title,omitempty"`
	Series    []ChartSeries          `json:"series"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	EmptyText string                 `json:"empty_text,omitempty"`
}

func (c *GaugeChart) GetChartType() string { return TypeGauge }
func (c *GaugeChart) SetChartType(typ string) {
	c.ChartType = typ
	fillEmptySeriesType(c.Series, typ)
}

func (c *GaugeChart) GetSeries() []ChartSeries { return c.Series }
func (c *GaugeChart) GetXAxis() []string       { return nil }
func (c *GaugeChart) GetEmptyText() string     { return c.EmptyText }

func fillEmptySeriesType(series []ChartSeries, typ string) {
	for i := range series {
		if series[i].Type == "" {
			series[i].Type = typ
		}
	}
}

func IsSupportedType(chartType string) bool {
	switch strings.TrimSpace(chartType) {
	case TypeBar, TypeLine, TypePie, TypeGauge:
		return true
	default:
		return false
	}
}

func IsSupportedValueFormat(valueFormat string) bool {
	switch strings.TrimSpace(valueFormat) {
	case "", ValueFormatCompact, ValueFormatPlain, ValueFormatDurationMS, ValueFormatPercent:
		return true
	default:
		return false
	}
}

func Validate(c Renderable) []string {
	return validateWarnings(c)
}

func ValidateWarnings(c Renderable) []string {
	return validateWarnings(c)
}

func validateWarnings(c Renderable) []string {
	if c == nil {
		return []string{"图表响应数据校验警告：chart 为空。SDK 会继续返回响应，但前端可能无法渲染图表；请返回 chart.LineChart、chart.BarChart、chart.PieChart、chart.GaugeChart 或 chart.Chart。"}
	}
	var warnings []string
	chartType := strings.TrimSpace(c.GetChartType())
	if chartType == "" {
		warnings = append(warnings, "图表响应数据校验警告：chart_type 为空。SDK 会继续返回响应，但前端可能无法确认渲染组件；请使用具体图表结构，或在 chart.Chart.ChartType 中填写 line/bar/pie/gauge。")
	} else if !IsSupportedType(chartType) {
		warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：chart_type=%q 不是 SDK 内置类型（支持 line/bar/pie/gauge）。SDK 会继续返回响应，请确认前端是否支持该类型。", chartType))
	}

	series := c.GetSeries()
	if len(series) == 0 {
		if strings.TrimSpace(c.GetEmptyText()) != "" {
			return warnings
		}
		warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：%s 图表没有 series，也没有 empty_text。SDK 会继续返回响应；如果确实暂无数据，请设置 empty_text。", displayChartType(chartType)))
		return warnings
	}
	for i, item := range series {
		if strings.TrimSpace(item.Name) == "" {
			warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：%s 图表 series[%d].name 为空。SDK 会继续返回响应；建议补上系列名称，方便前端图例和排查。", displayChartType(chartType), i))
		}
		if len(item.Data) == 0 {
			warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：%s 图表 series[%d](%q).data 为空。SDK 会继续返回响应；如果确实暂无数据，请考虑使用 empty_text。", displayChartType(chartType), i, item.Name))
		}
		if item.Type != "" && !IsSupportedType(item.Type) {
			warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：%s 图表 series[%d](%q).type=%q 不是 SDK 内置类型。SDK 会继续返回响应，请确认前端是否支持该系列类型。", displayChartType(chartType), i, item.Name, item.Type))
		}
	}

	switch chartType {
	case TypeLine, TypeBar:
		if axis := yAxisConfig(c); axis != nil && !IsSupportedValueFormat(axis.ValueFormat) {
			warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：%s 图表 y_axis.value_format=%q 不是 SDK 内置格式（支持 compact/plain/duration_ms/percent）。SDK 会继续返回响应，请确认前端是否支持该格式。", displayChartType(chartType), axis.ValueFormat))
		}
		warnings = append(warnings, validateAxisChartWarnings(chartType, c.GetXAxis(), series)...)
	case TypePie:
		warnings = append(warnings, validatePieChartWarnings(series)...)
	case TypeGauge:
		warnings = append(warnings, validateGaugeChartWarnings(series)...)
	}
	return warnings
}

func yAxisConfig(c Renderable) *AxisConfig {
	switch value := c.(type) {
	case *Chart:
		return value.YAxis
	case *LineChart:
		return value.YAxis
	case *BarChart:
		return value.YAxis
	default:
		return nil
	}
}

func validateAxisChartWarnings(chartType string, xAxis []string, series []ChartSeries) []string {
	var warnings []string
	if len(xAxis) == 0 {
		warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：%s 图表 x_axis 为空。SDK 会继续返回响应；折线图/柱状图通常需要 x_axis 作为横轴。", displayChartType(chartType)))
	}
	for i, item := range series {
		if len(item.Data) != len(xAxis) {
			warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：%s 图表 series[%d](%q).data 长度是 %d，但 x_axis 长度是 %d。SDK 会继续返回响应；如果是采集缺点，建议用 nil 补齐对应时间点。", displayChartType(chartType), i, item.Name, len(item.Data), len(xAxis)))
		}
	}
	return warnings
}

func validatePieChartWarnings(series []ChartSeries) []string {
	var warnings []string
	for i, item := range series {
		for j, point := range item.Data {
			if issue := describePiePointIssue(point); issue != "" {
				warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：pie 图表 series[%d](%q).data[%d] 不符合 {name, value} 形态：%s。SDK 会继续返回响应，请确认前端是否能渲染。", i, item.Name, j, issue))
			}
		}
	}
	return warnings
}

func validateGaugeChartWarnings(series []ChartSeries) []string {
	var warnings []string
	if len(series) != 1 {
		warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：gauge 图表期望 exactly one series，但当前有 %d 个。SDK 会继续返回响应，请确认是否符合前端预期。", len(series)))
	}
	if len(series) == 0 {
		return warnings
	}
	if len(series[0].Data) != 1 {
		warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：gauge 图表 series[0](%q).data 期望 exactly one data point，但当前有 %d 个。SDK 会继续返回响应，请确认是否符合前端预期。", series[0].Name, len(series[0].Data)))
	}
	if len(series[0].Data) > 0 && !isNumericValue(series[0].Data[0]) {
		warnings = append(warnings, fmt.Sprintf("图表响应数据校验警告：gauge 图表 series[0](%q).data[0] 期望数字，但当前类型是 %T。SDK 会继续返回响应，请确认前端是否能渲染。", series[0].Name, series[0].Data[0]))
	}
	return warnings
}

func isPiePoint(point interface{}) bool {
	return describePiePointIssue(point) == ""
}

func describePiePointIssue(point interface{}) string {
	if point == nil {
		return "数据点是 nil"
	}
	switch value := point.(type) {
	case map[string]interface{}:
		if strings.TrimSpace(fmt.Sprint(value["name"])) == "" {
			return "缺少非空 name"
		}
		if !isNumericValue(value["value"]) {
			return fmt.Sprintf("value 不是数字，当前类型是 %T", value["value"])
		}
		return ""
	}
	reflected := reflect.ValueOf(point)
	if reflected.Kind() == reflect.Struct {
		name := reflected.FieldByName("Name")
		value := reflected.FieldByName("Value")
		if !name.IsValid() || !name.CanInterface() {
			return "结构体缺少可读取的 Name 字段"
		}
		if !value.IsValid() || !value.CanInterface() {
			return "结构体缺少可读取的 Value 字段"
		}
		if strings.TrimSpace(fmt.Sprint(name.Interface())) == "" {
			return "Name 为空"
		}
		if !isNumericValue(value.Interface()) {
			return fmt.Sprintf("Value 不是数字，当前类型是 %T", value.Interface())
		}
		return ""
	}
	return fmt.Sprintf("数据点类型是 %T，不是 map[string]interface{} 或包含 Name/Value 的结构体", point)
}

func isNumericValue(value interface{}) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	default:
		return false
	}
}

func displayChartType(chartType string) string {
	if strings.TrimSpace(chartType) == "" {
		return "unknown"
	}
	return chartType
}
