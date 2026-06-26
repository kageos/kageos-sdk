package response

import (
	"fmt"
	"strings"

	"github.com/kageos/kageos-sdk/agent-app/chart"
)

type RunFunctionResp struct {
	Type      string     `json:"type"`
	TableData *TableData `json:"table_data"`
	FormData  *FormData  `json:"form_data"`
	ChartData *ChartData `json:"chart_data"`

	ExpectedChartType string `json:"-"`
	warnings          []string

	//系统错误
	err error

	//是否是业务错误？
	BizError interface{}
}

func (r *RunFunctionResp) Data() interface{} {
	if r.Type == "form" {
		return r.FormData.Data
	}
	if r.Type == "table" {
		return r.TableData
	}
	if r.Type == "chart" {
		return r.ChartData
	}
	return nil
}

type BizErr struct {
	Msg string `json:"msg"`
}

func (e *BizErr) Error() string {
	return e.Msg
}

func (r *RunFunctionResp) Build() error {
	if r.err != nil {
		return r.err
	}

	if r.BizError != nil {
		return &BizErr{Msg: fmt.Sprintf("%v", r.BizError)}
	}

	if r.Type == "form" {
		return nil
	}

	if r.Type == "chart" {
		return nil
	}

	return nil
}

type TableData struct {
	Items     interface{} `json:"items"`
	Paginated *Paginated  `json:"paginated"`
}
type FormData struct {
	Data interface{} `json:"data"`
}

type ChartData struct {
	Chart    chart.Renderable `json:"chart"` // Renderable 实现体，resp.Chart() 时调用 SetChartType 注入 ChartType / Series.Type
	Warnings []string         `json:"warnings,omitempty"`
}

type Builder interface {
	Build() error
}

type Response interface {
	Form(data interface{}) Form
	BizErrorf(format string, a ...any) Form
	Table(result TableResult) Table
	Chart(c chart.Renderable) Chart
}

func (r *RunFunctionResp) Form(data interface{}) Form {
	r.Type = "form"
	r.FormData = &FormData{
		Data: data,
	}
	return r
}

// Chart 接收 chart.Renderable 接口；调用 SetChartType(GetChartType()) 注入 ChartType（及 Series.Type），无需反射
func (r *RunFunctionResp) Chart(c chart.Renderable) Chart {
	r.Type = "chart"
	if c == nil {
		warnings := chart.Validate(nil)
		r.addWarnings(warnings...)
		r.ChartData = &ChartData{Warnings: r.Warnings()}
		return r
	}
	chartType := strings.TrimSpace(c.GetChartType())
	if expected := strings.TrimSpace(r.ExpectedChartType); expected != "" && chartType != expected {
		r.addWarnings(fmt.Sprintf("图表响应数据校验警告：handler 返回的 chart_type=%q 与模板声明的 chart_type=%q 不一致。SDK 会继续返回响应，请确认是否符合预期。", chartType, expected))
	}
	if chartType != "" {
		c.SetChartType(chartType)
	}
	warnings := chart.Validate(c)
	r.addWarnings(warnings...)
	r.ChartData = &ChartData{Chart: c, Warnings: r.Warnings()}
	return r
}

func (r *RunFunctionResp) Warnings() []string {
	if r == nil || len(r.warnings) == 0 {
		return nil
	}
	return append([]string(nil), r.warnings...)
}

func (r *RunFunctionResp) addWarnings(warnings ...string) {
	if r == nil {
		return
	}
	for _, warning := range warnings {
		if strings.TrimSpace(warning) == "" {
			continue
		}
		r.warnings = append(r.warnings, warning)
	}
}
