package app

import (
	"github.com/kageos/kageos-sdk/agent-app/callback"
)

const (
	CallbackTypeOnTableAddRow      = "OnTableAddRow"
	CallbackTypeOnTableUpdateRow   = "OnTableUpdateRow"
	CallbackTypeOnTableDeleteRows  = "OnTableDeleteRows"
	CallbackTypeOnTableExportPlan  = "OnTableExportPlan"
	CallbackTypeOnTableExportChunk = "OnTableExportChunk"
	CallbackTypeOnPageLoad         = "OnPageLoad"
	CallbackTypeOnSelectFuzzy      = "OnSelectFuzzy"

	// CallbackTypeSystemTableGetRows is an SDK-private callback used by the
	// platform to fetch existing table rows by primary key before updates.
	CallbackTypeSystemTableGetRows        = "__table_get_rows"
	CallbackTypeSystemTableGetDeletedRows = "__table_get_deleted_rows"
	CallbackTypeSystemTableRestoreRows    = "__table_restore_rows"
)

type OnTableAddRow func(ctx *Context, req *callback.OnTableAddRowReq) (*callback.OnTableAddRowResp, error)

// OnTableDeleteRows 当返回前端的数据是table类型时候，前端会把数据渲染成表格，这时候表格数据会有删除的行为，实现这个函数用来删除数据
type OnTableDeleteRows func(ctx *Context, req *callback.OnTableDeleteRowsReq) (*callback.OnTableDeleteRowsResp, error)

// OnTableUpdateRow 当返回前端的数据是table类型时候，前端会把数据渲染成表格，这时候表格数据会有更新的行为，实现这个函数用来更新数据
type OnTableUpdateRow func(ctx *Context, req *callback.OnTableUpdateRowReq) (*callback.OnTableUpdateRowResp, error)

// OnTableExportPlan freezes table export membership and returns independently
// selectable blocks. Apps should use a stable unique-key boundary, not OFFSET.
type OnTableExportPlan func(ctx *Context, req *callback.OnTableExportPlanReq) (*callback.OnTableExportPlanResp, error)

// OnTableExportChunk returns rows for one opaque block cursor from the plan.
type OnTableExportChunk func(ctx *Context, req *callback.OnTableExportChunkReq) (*callback.OnTableExportChunkResp, error)

// OnApiCreate 假如某次更新app时候，新增了这个api，则会出发点这个回调
type OnApiCreate func(ctx *Context, req *callback.OnApiCreateReq) (*callback.OnApiCreateResp, error)

// 前端访问该函数时候触发该回调
type OnPageLoad func(ctx *Context, req *callback.OnPageLoadReq) (*callback.OnPageLoadResp, error)

// OnSelectFuzzy 只有select组件才有的，当在select输入框输入关键字时候，如果有这个回调的话，会触发这个回调
type OnSelectFuzzy func(ctx *Context, req *callback.OnSelectFuzzyReq) (*callback.OnSelectFuzzyResp, error)
