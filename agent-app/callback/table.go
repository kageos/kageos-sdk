package callback

import (
	"encoding/json"
	"fmt"
)

type OnTableAddRowReq struct {
	Body interface{} `json:"body"`
}

type OnTableAddRowResp struct {
	Data interface{} `json:"data"`
}

type OnTableDeleteRowsReq struct {
	Ids []int `json:"ids"`
}

func (c *OnTableDeleteRowsReq) GetIds() []int {
	return c.Ids
}

type OnTableDeleteRowsResp struct {
}

// TableExportBlock describes one independently exportable block in a stable
// snapshot. Cursor is opaque to the platform and is passed back unchanged.
type TableExportBlock struct {
	Index    int    `json:"index"`
	StartRow int64  `json:"start_row"`
	EndRow   int64  `json:"end_row"`
	RowCount int    `json:"row_count"`
	Cursor   string `json:"cursor"`
}

// OnTableExportPlanReq asks the app to freeze export membership for the
// current filters. Filters contain the same business search values as the
// table list request.
type OnTableExportPlanReq struct {
	Filters   map[string]interface{} `json:"filters"`
	ChunkSize int                    `json:"chunk_size"`
}

type OnTableExportPlanResp struct {
	Snapshot string             `json:"snapshot"`
	Total    int64              `json:"total"`
	Blocks   []TableExportBlock `json:"blocks"`
}

// OnTableExportChunkReq reads one block from a previously created snapshot.
// Snapshot and Cursor are opaque values produced by OnTableExportPlan.
type OnTableExportChunkReq struct {
	Snapshot string                 `json:"snapshot"`
	Cursor   string                 `json:"cursor"`
	Limit    int                    `json:"limit"`
	Filters  map[string]interface{} `json:"filters"`
}

type OnTableExportChunkResp struct {
	Rows interface{} `json:"rows"`
}

type TableGetRowsReq struct {
	IDs []int64 `json:"ids"`
}

func (c *TableGetRowsReq) GetIDs() []int64 {
	if c == nil {
		return nil
	}
	return c.IDs
}

type TableGetRowsResp struct {
	Rows interface{} `json:"rows"`
}

type TableGetDeletedRowsReq struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type TableGetDeletedRowsResp struct {
	Rows        interface{} `json:"rows"`
	Total       int64       `json:"total"`
	Page        int         `json:"page"`
	PageSize    int         `json:"page_size"`
	Table       string      `json:"table,omitempty"`
	PackagePath string      `json:"package_path,omitempty"`
}

type TableRestoreRowsReq struct {
	IDs []int64 `json:"ids"`
}

type TableRestoreRowsResp struct {
	Rows     interface{} `json:"rows"`
	Restored int64       `json:"restored"`
}

type OnTableUpdateRowReq struct {
	ID                   int                    `json:"id"`
	ChangedFieldsBindMap map[string]interface{} `json:"changed_fields_bind_map"` // 前端提交的原始更新值备份，用于需要按原始表单值绑定结构体的场景。

	Updates   map[string]interface{} `json:"updates"`
	OldValues map[string]interface{} `json:"old_values"`
}

func (c *OnTableUpdateRowReq) GetId() int {
	// ⚠️ 关键：ID 现在由前端直接传递，不再从 Updates 中获取
	if c.ID != 0 {
		return c.ID
	}
	// 如果既没有 ID 字段，Updates 中也没有 id，返回 0（由业务层处理错误）
	return 0
}

// ChangedFields 获取本次更新字段（只包含变更的字段）。
//
// 返回值适合直接配合 GORM Updates：
//
//	updates := req.ChangedFields()
//	db.Model(&Model{}).Where("id = ?", req.GetId()).Updates(updates)
func (c *OnTableUpdateRowReq) ChangedFields() map[string]interface{} {
	if c.Updates == nil {
		return make(map[string]interface{})
	}
	return c.Updates
}

// IsFieldUpdated 判断指定字段是否在此次更新中被变更
//
// 这是一个快捷方法，用于替代 `if _, hasField := updates["field"]; hasField` 的写法
//
// 示例：
//
//	if req.IsFieldUpdated("quantity") {
//	    // quantity 字段在此次更新中被变更
//	}
//	if req.IsFieldUpdated("unit_price") {
//	    // unit_price 字段在此次更新中被变更
//	}
func (c *OnTableUpdateRowReq) IsFieldUpdated(fieldName string) bool {
	if c.Updates == nil {
		return false
	}
	_, exists := c.Updates[fieldName]
	return exists
}

// GetOldValues 获取旧值（用于审计）
func (c *OnTableUpdateRowReq) GetOldValues() map[string]interface{} {
	if c.OldValues == nil {
		return make(map[string]interface{})
	}
	return c.OldValues
}

// BindChangedFields 将本次变更字段绑定到目标结构体。
//
// ⚠️ 重要说明：
//   - Updates 只包含此次更新中变更的字段，未更新的字段不在 Updates 中
//   - 绑定后，目标结构体中只有更新的字段有值，未更新的字段为零值
//   - 如果需要访问未更新的字段，应该从数据库中查询当前记录
//
// 使用 JSON 序列化/反序列化的方式，确保类型正确转换
//
// 示例：
//
//	var updateFields CrmMeetingRoom
//	if err := req.BindChangedFields(&updateFields); err != nil {
//	    return nil, err
//	}
//	// 此时 updateFields 中只有更新的字段有值，例如：
//	// 如果只更新了 name，则 updateFields.Name 有值，其他字段为零值
//
// 判断字段是否参与本次更新时，应配合 IsFieldUpdated 使用，不要只看零值。
func (c *OnTableUpdateRowReq) BindChangedFields(target interface{}) error {
	if c.ChangedFieldsBindMap == nil || len(c.ChangedFieldsBindMap) == 0 {
		return nil
	}

	// 将 map 序列化为 JSON
	jsonData, err := json.Marshal(c.ChangedFieldsBindMap)
	if err != nil {
		return fmt.Errorf("序列化 updates 失败: %w", err)
	}

	// 反序列化到目标结构体
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("反序列化到目标结构体失败, json: %s, err: %w", string(jsonData), err)
	}

	return nil
}

type OnTableUpdateRowResp struct {
}
