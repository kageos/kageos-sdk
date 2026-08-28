package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/kageos/kageos-sdk/agent-app/callback"
	"github.com/kageos/kageos-sdk/agent-app/response"
	"github.com/kageos/kageos-sdk/pkg/logger"
	"gorm.io/gorm"
)

type PackageContext struct {
	RouterGroup string        `json:"router_group"`
	Name        string        `json:"name,omitempty"` // 包名称（可选）
	Desc        string        `json:"desc,omitempty"` // 包描述（可选）
	AgentTasks  []AgentTask   `json:"agent_tasks,omitempty"`
	Docs        []DocManifest `json:"docs,omitempty"`
}

// RegisterOptions 路由注册选项
type RegisterOptions struct {
	PackagePath string // 服务目录路径（package路径），用于获取对应的数据库连接
}

func (r *RegisterOptions) GetDBName(user string, app string) string {
	trim := strings.Trim(r.PackagePath, "/")
	split := strings.Split(trim, "/")
	join := strings.Join(split, "-")
	dbName := fmt.Sprintf("%s.db", join)
	return dbName
}

// BuildFullRouter 构建完整路由路径
// router: 相对路由路径（如 "extract_text"）
// 返回: 完整路由路径（如 "/tools/pdftools/extract_text"）
func (p *PackageContext) BuildFullRouter(router string) string {
	packagePath := strings.Trim(p.RouterGroup, "/")
	return fmt.Sprintf("/%s/%s", packagePath, strings.Trim(router, "/"))
}

// GetGormDB 已废弃。应用业务数据库必须通过请求或回调 Context 的 ctx.GetGormDB() 获取。
func (p *PackageContext) GetGormDB() (*gorm.DB, error) {
	return GetDBByPackagePath(p.RouterGroup)
}

// register 通用的注册方法，构建路由路径并注册
func (p *PackageContext) register(method string, router string, handleFunc HandleFunc, templater Templater) {
	// 确保 app 已初始化
	if app == nil {
		initApp()
	}

	// 如果初始化失败，app 可能仍然是 nil，延迟注册到 Run() 时
	if app == nil {
		logger.Errorf(context.Background(), "Cannot register router %s %s: app initialization failed", method, router)
		return
	}

	// 构建完整路由路径：RouterGroup + "/" + router
	// 例如："/tools/pdftools" + "/" + "extract_text" -> "/tools/pdftools/extract_text"
	fullRouter := p.BuildFullRouter(router)
	packagePath := strings.Trim(p.RouterGroup, "/") // 从 RouterGroup 提取 PackagePath

	app.packageContexts[packagePath] = p

	// 创建 options，设置 PackagePath（用于获取对应的数据库连接）
	options := &RegisterOptions{
		PackagePath: packagePath,
	}

	// 直接调用 app.addRoute，跳过中间层
	if err := app.addRoute(fullRouter, method, handleFunc, templater, options); err != nil {
		logger.Errorf(context.Background(), "Failed to register router %s %s: %v", method, fullRouter, err)
		panic(err) // 注册失败时 panic，避免静默失败
	}
}

// POST 注册 POST 路由
func (p *PackageContext) POST(router string, handleFunc HandleFunc, templater Templater) {
	p.register("POST", router, handleFunc, templater)
}

// GET 注册 GET 路由
func (p *PackageContext) GET(router string, handleFunc HandleFunc, templater Templater) {
	p.register("GET", router, handleFunc, templater)
}

// PUT 注册 PUT 路由
func (p *PackageContext) PUT(router string, handleFunc HandleFunc, templater Templater) {
	p.register("PUT", router, handleFunc, templater)
}

// DELETE 注册 DELETE 路由
func (p *PackageContext) DELETE(router string, handleFunc HandleFunc, templater Templater) {
	p.register("DELETE", router, handleFunc, templater)
}

// routerKey 构建路由 key（URL 唯一，不包含 method）
func routerKey(router string) string {
	return strings.Trim(router, "/")
}

func initRouter(a *App) {

	// ⚠️ 重要：必须直接操作 a.routerInfo，不能调用 a.registerRouter() 或 PackageContext.register()
	//
	// 原因：死锁问题
	// 1. initRouter() 在 NewApp() 中被调用
	// 2. NewApp() 本身在 initApp() 的 sync.Once.Do() 中执行
	// 3. 此时全局变量 app 还没有被赋值（NewApp() 还没返回）
	// 4. 如果调用 PackageContext.register()，它会检查 app == nil，然后再次调用 initApp()
	// 5. sync.Once.Do() 会阻塞等待第一次执行完成，但第一次执行就是 NewApp()
	// 6. 而 NewApp() 又调用了 initRouter()，形成死锁
	//
	// 解决方案：直接操作传入的 App 实例的 routerInfo，避免触发全局 app 的检查
	//
	// ✅ 改造后：URL 唯一，/_callback 只注册一次，method 设为 "ANY" 表示支持所有 method
	key := routerKey("/_callback")
	if _, exists := a.routerInfo[key]; exists {
		panic(fmt.Errorf("路由 /_callback 已存在，不允许重复注册"))
	}

	a.routerInfo[key] = &routerInfo{
		HandleFunc: a.CallbackRouter,
		Router:     "/_callback",
		Method:     "ANY", // 支持所有 method（GET、POST、PUT、DELETE）
		Options:    nil,   // 系统路由没有 PackagePath
		Template:   &FormTemplate{},
	}

	key = routerKey(runtimePythonRouter)
	if _, exists := a.routerInfo[key]; exists {
		panic(fmt.Errorf("路由 %s 已存在，不允许重复注册", runtimePythonRouter))
	}

	a.routerInfo[key] = &routerInfo{
		HandleFunc: a.RuntimePython,
		Router:     runtimePythonRouter,
		Method:     "POST",
		Options:    nil,
		Template:   &FormTemplate{},
	}
}

type CallbackRouterReq struct {
	Type   string `json:"type" binding:"required" example:""`
	Method string `json:"method" binding:"required" example:""`
	Router string `json:"router" binding:"required" example:"/users/app/xxxx"`
	Body   []byte `json:"body" example:"eyJpZCI6MX0="`
}

func (a *App) CallbackRouter(ctx *Context, resp response.Response) error {
	var req CallbackRouterReq
	if err := json.Unmarshal(ctx.body, &req); err != nil {
		logger.Errorf(ctx, "CallbackRouter unmarshal failed: bodyLen=%d err=%v", len(ctx.body), err)
		return err
	}

	router, err := a.getRoute(req.Router)
	if err != nil {
		return err
	}

	//callback只是代理路由，要重定向到真正的路由
	ctx.msg.Router = req.Router
	ctx.msg.Method = req.Method
	ctx.body = req.Body
	// 设置 routerInfo，方便后续获取 PackagePath
	ctx.routerInfo = router

	switch req.Type {
	case CallbackTypeSystemTableGetRows:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		var onTableReq callback.TableGetRowsReq
		if err := json.Unmarshal(ctx.body, &onTableReq); err != nil {
			return err
		}
		onTableResp, err := handleSystemTableGetRows(ctx, v, &onTableReq)
		if err != nil {
			return err
		}
		if err := resp.Form(onTableResp).Build(); err != nil {
			logger.Errorf(ctx, "callback %s router:%s error:%s", req.Type, req.Router, err.Error())
			return err
		}
		logger.Debugf(ctx, "CallbackRouter %s success", req.Type)
		return nil
	case CallbackTypeSystemTableGetDeletedRows:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		var callbackReq callback.TableGetDeletedRowsReq
		if err := json.Unmarshal(ctx.body, &callbackReq); err != nil {
			return err
		}
		callbackResp, err := handleSystemTableGetDeletedRows(ctx, v, &callbackReq)
		if err != nil {
			return err
		}
		return resp.Form(callbackResp).Build()
	case CallbackTypeSystemTableRestoreRows:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		var callbackReq callback.TableRestoreRowsReq
		if err := json.Unmarshal(ctx.body, &callbackReq); err != nil {
			return err
		}
		callbackResp, err := handleSystemTableRestoreRows(ctx, v, &callbackReq)
		if err != nil {
			return err
		}
		return resp.Form(callbackResp).Build()
	case CallbackTypeOnTableAddRow:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		if v.OnTableAddRow == nil {
			return errors.New("callback OnTableAddRow is not registered")
		}
		var onTableReq callback.OnTableAddRowReq
		onTableResp, err := v.OnTableAddRow(ctx, &onTableReq)
		if err != nil {
			logger.Errorf(ctx, "callback onTableAddRow router:%s call error:%s", req.Type, err.Error())
			return err
		}
		err = resp.Form(onTableResp).Build()
		if err != nil {
			logger.Errorf(ctx, "callback onTableAddRow  router:%s Build error:%s", req.Type, err.Error())
			return err
		}
		logger.Debugf(ctx, "CallbackRouter onTableAddRow success")
		return nil
	case CallbackTypeOnTableUpdateRow:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		if v.OnTableUpdateRow == nil {
			return errors.New("callback OnTableUpdateRow is not registered")
		}
		var onTableReq callback.OnTableUpdateRowReq
		// ⚠️ 关键：现在解析整个结构，包括 id、updates、old_values
		// 前端传递格式：{"id": 2, "updates": {"name": "802"}, "old_values": {"name": "801"}}
		err := json.Unmarshal(ctx.body, &onTableReq)
		if err != nil {
			return err
		}
		if onTableReq.ChangedFieldsBindMap == nil {
			onTableReq.ChangedFieldsBindMap = make(map[string]interface{})
		}
		for k, vv := range onTableReq.Updates {
			onTableReq.ChangedFieldsBindMap[k] = vv
		}
		onTableResp, err := v.OnTableUpdateRow(ctx, &onTableReq)
		if err != nil {
			return err
		}
		err = resp.Form(onTableResp).Build()
		if err != nil {
			logger.Errorf(ctx, "callback OnTableUpdateRows router:%s error:%s", req.Type, err.Error())
			return err
		}
		logger.Debugf(ctx, "CallbackRouter OnTableUpdateRows success")
		return nil
	case CallbackTypeOnTableDeleteRows:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		if v.OnTableDeleteRows == nil {
			return errors.New("callback OnTableDeleteRows is not registered")
		}
		var onTableReq callback.OnTableDeleteRowsReq
		err := json.Unmarshal(ctx.body, &onTableReq)
		if err != nil {
			return err
		}
		onTableResp, err := v.OnTableDeleteRows(ctx, &onTableReq)
		if err != nil {
			return err
		}
		if err := stampSystemSoftDeletedBy(ctx, v, onTableReq.GetIds()); err != nil {
			// 删除已经成功，补写审计字段失败不能把成功响应伪装成删除失败。
			logger.Errorf(ctx, "callback OnTableDeleteRows stamp deleted_by failed router:%s error:%s", req.Router, err.Error())
		}
		err = resp.Form(onTableResp).Build()
		if err != nil {
			logger.Errorf(ctx, "callback OnTableDeleteRows router:%s error:%s", req.Type, err.Error())
			return err
		}
		logger.Debugf(ctx, "CallbackRouter OnTableDeleteRows success")
		return nil
	case CallbackTypeOnTableExportPlan:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		if v.OnTableExportPlan == nil || v.OnTableExportChunk == nil {
			return errors.New("table export callbacks must be registered together")
		}
		var callbackReq callback.OnTableExportPlanReq
		if err := json.Unmarshal(ctx.body, &callbackReq); err != nil {
			return err
		}
		callbackResp, err := v.OnTableExportPlan(ctx, &callbackReq)
		if err != nil {
			return err
		}
		if callbackResp == nil || callbackResp.Total < 0 || callbackResp.Snapshot == "" {
			return errors.New("OnTableExportPlan returned an invalid export plan")
		}
		return resp.Form(callbackResp).Build()
	case CallbackTypeOnTableExportChunk:
		v, ok := router.Template.(*TableTemplate)
		if !ok {
			return errors.New("invalid type of TableTemplate")
		}
		if v.OnTableExportPlan == nil || v.OnTableExportChunk == nil {
			return errors.New("table export callbacks must be registered together")
		}
		var callbackReq callback.OnTableExportChunkReq
		if err := json.Unmarshal(ctx.body, &callbackReq); err != nil {
			return err
		}
		if callbackReq.Snapshot == "" || callbackReq.Cursor == "" || callbackReq.Limit < 1 {
			return errors.New("OnTableExportChunk request is incomplete")
		}
		callbackResp, err := v.OnTableExportChunk(ctx, &callbackReq)
		if err != nil {
			return err
		}
		if callbackResp == nil || callbackResp.Rows == nil {
			return errors.New("OnTableExportChunk returned no rows")
		}
		return resp.Form(callbackResp).Build()
	case CallbackTypeOnSelectFuzzy:
		var onCallback callback.OnSelectFuzzyReq
		base := router.Template.GetBaseConfig()
		err := json.Unmarshal(ctx.body, &onCallback)
		if err != nil {
			return err
		}

		fuzzy := base.OnSelectFuzzyMap[onCallback.Code]
		if fuzzy == nil {
			return errors.New("invalid code " + onCallback.Code)
		}
		fuzzyResp, err := fuzzy(ctx, &onCallback)
		if err != nil {
			return err
		}
		err = resp.Form(fuzzyResp).Build()
		if err != nil {
			logger.Errorf(ctx, "callback OnSelectFuzzy router:%s error:%s", req.Type, err.Error())
			return err
		}
		logger.Debugf(ctx, "CallbackRouter OnSelectFuzzy success")
	}
	return nil

}

func handleSystemTableGetRows(ctx *Context, template *TableTemplate, req *callback.TableGetRowsReq) (*callback.TableGetRowsResp, error) {
	if template == nil {
		return nil, errors.New("invalid type of TableTemplate")
	}
	ids := req.GetIDs()
	if len(ids) == 0 {
		return &callback.TableGetRowsResp{Rows: []map[string]interface{}{}}, nil
	}

	model := template.EffectiveAutoCrudTable()
	if model == nil {
		return nil, errors.New("[系统错误]-[__table_get_rows] 表格未配置 AutoCrudTable，无法按 id 查询旧值")
	}
	rowsPtr, err := newRowsSlicePtr(model)
	if err != nil {
		return nil, fmt.Errorf("[系统错误]-[__table_get_rows] 构造查询结果失败: %w", err)
	}
	db := ctx.GetGormDB()
	if db == nil {
		return nil, errors.New("[系统错误]-[__table_get_rows] 应用数据库不可用")
	}
	if err := db.Model(model).Where("id IN ?", ids).Find(rowsPtr.Interface()).Error; err != nil {
		return nil, fmt.Errorf("[系统错误]-[__table_get_rows] 查询旧值失败: %w", err)
	}
	return &callback.TableGetRowsResp{Rows: rowsPtr.Elem().Interface()}, nil
}

func handleSystemTableGetDeletedRows(ctx *Context, template *TableTemplate, req *callback.TableGetDeletedRowsReq) (*callback.TableGetDeletedRowsResp, error) {
	model, db, err := systemSoftDeleteTable(ctx, template, CallbackTypeSystemTableGetDeletedRows)
	if err != nil {
		return nil, err
	}
	if req == nil {
		req = &callback.TableGetDeletedRowsReq{}
	}
	page, pageSize := normalizeSystemDeletedRowsPage(req.Page, req.PageSize)
	query := db.Unscoped().Model(model).Where("deleted_at IS NOT NULL")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("[系统错误]-[%s] 查询已删除记录总数失败: %w", CallbackTypeSystemTableGetDeletedRows, err)
	}
	rowsPtr, err := newRowsSlicePtr(model)
	if err != nil {
		return nil, fmt.Errorf("[系统错误]-[%s] 构造查询结果失败: %w", CallbackTypeSystemTableGetDeletedRows, err)
	}
	if err := query.Order("deleted_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(rowsPtr.Interface()).Error; err != nil {
		return nil, fmt.Errorf("[系统错误]-[%s] 查询已删除记录失败: %w", CallbackTypeSystemTableGetDeletedRows, err)
	}
	rows, err := systemRowsToVisibleMaps(rowsPtr.Elem())
	if err != nil {
		return nil, fmt.Errorf("[系统错误]-[%s] 序列化已删除记录失败: %w", CallbackTypeSystemTableGetDeletedRows, err)
	}
	statement := &gorm.Statement{DB: db}
	tableName := ""
	if err := statement.Parse(model); err == nil && statement.Schema != nil {
		tableName = statement.Schema.Table
	}
	packagePath := ""
	if ctx.routerInfo != nil && ctx.routerInfo.Options != nil {
		packagePath = ctx.routerInfo.Options.PackagePath
	}
	return &callback.TableGetDeletedRowsResp{Rows: rows, Total: total, Page: page, PageSize: pageSize, Table: tableName, PackagePath: packagePath}, nil
}

func handleSystemTableRestoreRows(ctx *Context, template *TableTemplate, req *callback.TableRestoreRowsReq) (*callback.TableRestoreRowsResp, error) {
	if req == nil || len(req.IDs) == 0 {
		return nil, errors.New("[参数错误]-[__table_restore_rows] ids 不能为空")
	}
	ids := make([]int64, 0, len(req.IDs))
	seen := make(map[int64]struct{}, len(req.IDs))
	for _, id := range req.IDs {
		if id <= 0 {
			return nil, errors.New("[参数错误]-[__table_restore_rows] ids 必须为正整数")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	model, db, err := systemSoftDeleteTable(ctx, template, CallbackTypeSystemTableRestoreRows)
	if err != nil {
		return nil, err
	}
	rowsPtr, err := newRowsSlicePtr(model)
	if err != nil {
		return nil, fmt.Errorf("[系统错误]-[%s] 构造恢复快照失败: %w", CallbackTypeSystemTableRestoreRows, err)
	}
	query := db.Unscoped().Model(model).Where("id IN ? AND deleted_at IS NOT NULL", ids)
	if err := query.Find(rowsPtr.Interface()).Error; err != nil {
		return nil, fmt.Errorf("[系统错误]-[%s] 查询待恢复记录失败: %w", CallbackTypeSystemTableRestoreRows, err)
	}
	rows, err := systemRowsToVisibleMaps(rowsPtr.Elem())
	if err != nil {
		return nil, fmt.Errorf("[系统错误]-[%s] 序列化恢复快照失败: %w", CallbackTypeSystemTableRestoreRows, err)
	}
	if len(rows) != len(ids) {
		return nil, fmt.Errorf("[参数错误]-[%s] 部分记录不存在或未被删除", CallbackTypeSystemTableRestoreRows)
	}
	updates := map[string]interface{}{"deleted_at": nil}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(model); err == nil && statement.Schema.LookUpField("DeletedBy") != nil {
		updates["deleted_by"] = ""
	}
	result := db.Unscoped().Model(model).Where("id IN ? AND deleted_at IS NOT NULL", ids).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("[业务错误]-[%s] 恢复记录失败，可能存在唯一值冲突: %w", CallbackTypeSystemTableRestoreRows, result.Error)
	}
	return &callback.TableRestoreRowsResp{Rows: rows, Restored: result.RowsAffected}, nil
}

func stampSystemSoftDeletedBy(ctx *Context, template *TableTemplate, ids []int) error {
	if ctx == nil || template == nil || len(ids) == 0 {
		return nil
	}
	deletedBy := strings.TrimSpace(ctx.GetRequestUser())
	if deletedBy == "" {
		return nil
	}
	model := template.EffectiveAutoCrudTable()
	if model == nil {
		return nil
	}
	db := ctx.GetGormDB()
	if db == nil {
		return errors.New("应用数据库不可用")
	}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(model); err != nil {
		return fmt.Errorf("解析表结构失败: %w", err)
	}
	if statement.Schema.LookUpField("DeletedAt") == nil || statement.Schema.LookUpField("DeletedBy") == nil {
		return nil
	}
	return db.Unscoped().Model(model).
		Where("id IN ? AND deleted_at IS NOT NULL", ids).
		Update("deleted_by", deletedBy).Error
}

func systemSoftDeleteTable(ctx *Context, template *TableTemplate, callbackType string) (interface{}, *gorm.DB, error) {
	if template == nil {
		return nil, nil, errors.New("invalid type of TableTemplate")
	}
	model := template.EffectiveAutoCrudTable()
	if model == nil {
		return nil, nil, fmt.Errorf("[系统错误]-[%s] 表格未配置 AutoCrudTable", callbackType)
	}
	if ctx == nil {
		return nil, nil, fmt.Errorf("[系统错误]-[%s] 请求上下文不可用", callbackType)
	}
	db := ctx.GetGormDB()
	if db == nil {
		return nil, nil, fmt.Errorf("[系统错误]-[%s] 应用数据库不可用", callbackType)
	}
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(model); err != nil {
		return nil, nil, fmt.Errorf("[系统错误]-[%s] 解析表结构失败: %w", callbackType, err)
	}
	if statement.Schema.LookUpField("DeletedAt") == nil {
		return nil, nil, fmt.Errorf("[业务错误]-[%s] 当前表格不支持软删除", callbackType)
	}
	return model, db, nil
}

func normalizeSystemDeletedRowsPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func systemRowsToVisibleMaps(rows reflect.Value) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, rows.Len())
	for i := 0; i < rows.Len(); i++ {
		row := rows.Index(i)
		raw, err := json.Marshal(row.Interface())
		if err != nil {
			return nil, err
		}
		visible := make(map[string]interface{})
		if err := json.Unmarshal(raw, &visible); err != nil {
			return nil, err
		}
		for row.Kind() == reflect.Ptr {
			row = row.Elem()
		}
		if row.Kind() == reflect.Struct {
			if field := row.FieldByName("DeletedAt"); field.IsValid() && field.CanInterface() {
				visible["deleted_at"] = field.Interface()
			}
			if field := row.FieldByName("DeletedBy"); field.IsValid() && field.CanInterface() {
				visible["deleted_by"] = field.Interface()
			}
		}
		result = append(result, visible)
	}
	return result, nil
}

func newRowsSlicePtr(model interface{}) (reflect.Value, error) {
	modelType := reflect.TypeOf(model)
	if modelType == nil {
		return reflect.Value{}, errors.New("model is nil")
	}
	for modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	if modelType.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("model must be struct or pointer to struct, got %s", modelType.Kind())
	}
	return reflect.New(reflect.SliceOf(modelType)), nil
}
