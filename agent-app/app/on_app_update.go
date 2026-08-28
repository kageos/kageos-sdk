package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kageos/kageos-sdk/agent-app/callback"
	"github.com/kageos/kageos-sdk/dto"
	"github.com/kageos/kageos-sdk/pkg/functionschema"
	"github.com/kageos/kageos-sdk/pkg/logger"
	"github.com/kageos/kageos-sdk/pkg/subjects"

	"github.com/kageos/kageos-sdk/agent-app/env"
	"github.com/kageos/kageos-sdk/agent-app/widget"
	"github.com/nats-io/nats.go"
)

// 获取API日志目录
func (a *App) getApiLogsDir() string {
	return "/app/workplace/api-logs"
}

// 获取当前版本的API文件路径
func (a *App) getCurrentVersionFile() string {
	return filepath.Join(a.getApiLogsDir(), fmt.Sprintf("%s.json", env.Version))
}

// 获取上一版本的API文件路径
func (a *App) getPreviousVersionFile() string {
	// 首先尝试直接推断上一版本号
	// 假设版本号格式为 v1, v2, v3...
	if len(env.Version) > 0 && env.Version[0] == 'v' {
		numStr := env.Version[1:]
		var current int
		if n, err := fmt.Sscanf(numStr, "%d", &current); err == nil && n == 1 {
			if current > 1 {
				prevVersion := fmt.Sprintf("v%d", current-1)
				prevFile := filepath.Join(a.getApiLogsDir(), prevVersion+".json")
				// 检查文件是否存在
				if _, err := os.Stat(prevFile); err == nil {
					return prevFile
				}
			}
		}
	}

	// 如果直接推断失败，再遍历目录查找上一版本
	apiLogsDir := a.getApiLogsDir()
	files, err := os.ReadDir(apiLogsDir)
	if err != nil {
		return ""
	}

	var maxVersion string
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		version := file.Name()[:len(file.Name())-5] // 去掉.json
		if version > maxVersion && version < env.Version {
			maxVersion = version
		}
	}

	if maxVersion != "" {
		return filepath.Join(apiLogsDir, maxVersion+".json")
	}

	return ""
}

// 保存当前版本的API信息到文件
func (a *App) saveCurrentVersion(apis []*ApiInfo) error {
	apiLogsDir := a.getApiLogsDir()

	// 创建目录
	if err := os.MkdirAll(apiLogsDir, 0755); err != nil {
		return fmt.Errorf("failed to create api logs directory: %w", err)
	}

	// 构建版本信息
	versionInfo := &ApiVersion{
		Version:   env.Version,
		Timestamp: time.Now(),
		Apis:      apis,
	}

	// 序列化
	data, err := json.MarshalIndent(versionInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version info: %w", err)
	}

	// 写入文件
	versionFile := a.getCurrentVersionFile()
	if err := os.WriteFile(versionFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write version file: %w", err)
	}

	return nil
}

// 加载指定版本的API信息
func (a *App) loadVersion(versionFile string) ([]*ApiInfo, error) {
	if versionFile == "" {
		return []*ApiInfo{}, nil
	}

	data, err := os.ReadFile(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ApiInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read version file: %w", err)
	}

	var versionInfo ApiVersion
	if err := json.Unmarshal(data, &versionInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal version info: %w", err)
	}

	return versionInfo.Apis, nil
}

// 检查版本是否存在于版本列表中
func (a *App) containsVersion(versions []string, version string) bool {
	for _, v := range versions {
		if v == version {
			return true
		}
	}
	return false
}

// 生成API的唯一键
func (a *App) getApiKey(api *ApiInfo) string {
	return fmt.Sprintf("%s:%s", api.Method, api.Router)
}

// 执行API差异对比（使用已获取的 currentApis，避免重复调用 getApis）
func (a *App) diffApiWithCurrentApis(currentApis []*ApiInfo) (add []*ApiInfo, update []*ApiInfo, delete []*ApiInfo, err error) {
	logger.Debugf(context.Background(), "=== Starting API diff analysis ===")

	logger.Debugf(context.Background(), "Found %d current APIs", len(currentApis))

	// 加载上一版本的API
	previousVersionFile := a.getPreviousVersionFile()
	logger.Debugf(context.Background(), "Previous version file: %s", previousVersionFile)
	previousApis, err := a.loadVersion(previousVersionFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load previous version: %w", err)
	}
	logger.Debugf(context.Background(), "Found %d previous APIs", len(previousApis))

	// 创建API映射
	currentMap := make(map[string]*ApiInfo)
	previousMap := make(map[string]*ApiInfo)

	for _, api := range currentApis {
		key := a.getApiKey(api)
		currentMap[key] = api
		logger.Debugf(context.Background(), "Current API: %s -> %s %s", key, api.Method, api.Router)
	}

	for _, api := range previousApis {
		key := a.getApiKey(api)
		previousMap[key] = api
		logger.Debugf(context.Background(), "Previous API: %s -> %s %s", key, api.Method, api.Router)
	}

	// 找出新增的API
	for key, currentApi := range currentMap {
		if _, exists := previousMap[key]; !exists {
			// 新增的API，设置AddedVersion为当前版本
			newApi := *currentApi
			newApi.AddedVersion = env.Version
			add = append(add, &newApi)
		}
	}

	// 找出删除的API
	for key, api := range previousMap {
		if _, exists := currentMap[key]; !exists {
			delete = append(delete, api)
		}
	}

	// 找出修改的API
	for key, currentApi := range currentMap {
		if previousApi, exists := previousMap[key]; exists {
			logger.Debugf(context.Background(), "Comparing API %s: %s %s", key, currentApi.Method, currentApi.Router)

			// 先比较API是否真的变更了
			isEqual := previousApi.IsEqual(currentApi)
			logger.Debugf(context.Background(), "API %s comparison result: %v", key, isEqual)

			if !isEqual {
				logger.Debugf(context.Background(), "API %s has changed, adding to update list", key)
				// 只有真正变更时才创建修改版本
				modifiedApi := *currentApi
				modifiedApi.AddedVersion = previousApi.AddedVersion

				// 复制原有的更新版本列表
				modifiedApi.UpdateVersions = make([]string, len(previousApi.UpdateVersions))
				copy(modifiedApi.UpdateVersions, previousApi.UpdateVersions)

				// 只有在真正变更时才添加当前版本到更新列表（如果不存在的话）
				if !a.containsVersion(modifiedApi.UpdateVersions, env.Version) {
					modifiedApi.UpdateVersions = append(modifiedApi.UpdateVersions, env.Version)
				}

				update = append(update, &modifiedApi)
			} else {
				logger.Debugf(context.Background(), "API %s unchanged, skipping", key)
			}
			// 如果API没有变更，什么都不做，保持原来的版本信息
		} else {
			logger.Debugf(context.Background(), "API %s not found in previous version", key)
		}
	}

	sortApiInfosByKey(add)
	sortApiInfosByKey(update)
	sortApiInfosByKey(delete)

	logger.Debugf(context.Background(), "=== API diff analysis completed ===")
	logger.Infof(context.Background(), "Added: %d, Updated: %d, Deleted: %d", len(add), len(update), len(delete))
	for i, api := range update {
		logger.Debugf(context.Background(), "Updated API %d: %s %s (AddedVersion: %s, UpdateVersions: %v)",
			i+1, api.Method, api.Router, api.AddedVersion, api.UpdateVersions)
	}

	return add, update, delete, nil
}

func sortApiInfosByKey(apis []*ApiInfo) {
	sort.Slice(apis, func(i, j int) bool {
		left := fmt.Sprintf("%s:%s", apis[i].Method, apis[i].Router)
		right := fmt.Sprintf("%s:%s", apis[j].Method, apis[j].Router)
		return left < right
	})
}

// 获取当前所有API信息
func (a *App) getApis() (apis []*ApiInfo, createTables []interface{}, err error) {
	var errs []error
	for _, info := range sortedRouterInfos(a.routerInfo) {
		if info == nil || info.IsDefaultRouter() {
			continue
		}
		api, tables, err := a.buildApiInfo(info)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		apis = append(apis, api)
		createTables = append(createTables, tables...)
	}
	if err := errors.Join(errs...); err != nil {
		return apis, createTables, err
	}
	return apis, createTables, nil
}

func (a *App) buildApiInfo(info *routerInfo) (*ApiInfo, []interface{}, error) {
	if info == nil {
		return nil, nil, fmt.Errorf("router info is nil")
	}
	if info.Template == nil {
		return nil, nil, fmt.Errorf("router %s template is nil", info.Router)
	}
	base := info.Template.GetBaseConfig()
	if base == nil {
		return nil, nil, fmt.Errorf("router %s base config is nil", info.Router)
	}

	connectorEndpoints := normalizeConnectorEndpoints(base.ConnectorEndpoints)
	api := &ApiInfo{
		Code:               info.getCode(),
		Name:               base.Name,
		Desc:               base.Desc,
		Tags:               base.Tags,
		Router:             info.Router,
		Method:             info.Method,
		Connectors:         normalizeConnectorCodes(append(append([]string{}, base.Connectors...), connectorCodesFromEndpoints(connectorEndpoints)...)),
		ConnectorEndpoints: connectorEndpoints,
		User:               env.User,
		App:                env.App,
		FullCodePath:       fmt.Sprintf("/%s/%s/%s", env.User, env.App, strings.Trim(info.Router, "/")),
		AddedVersion:       "",         // 不预设版本，让diff逻辑来正确设置
		UpdateVersions:     []string{}, // 初始化空的更新版本列表
		routerInfo:         info,
	}

	fieldsCallback := make(map[string][]string)
	fuzzyMap := base.OnSelectFuzzyMap
	if len(fuzzyMap) > 0 {
		for field := range fuzzyMap {
			fieldsCallback[field] = append(fieldsCallback[field], CallbackTypeOnSelectFuzzy)
		}
	}

	templateType := info.Template.TemplateType()
	api.TemplateType = string(templateType)

	var errs []error
	switch templateType {
	case TemplateTypeTable:
		template, ok := info.Template.(*TableTemplate)
		if !ok {
			errs = append(errs, fmt.Errorf("router %s declares template type %q but template is not *TableTemplate", info.Router, templateType))
			break
		}
		table := template.EffectiveAutoCrudTable()
		requestFields, responseFields, err := widget.DecodeTable(fieldsCallback, base.Request, table)
		if err != nil {
			errs = append(errs, fmt.Errorf("router %s table schema decode failed: %w", info.Router, err))
		}
		var callback []string
		if template.OnTableAddRow != nil {
			callback = append(callback, CallbackTypeOnTableAddRow)
		}
		if template.OnTableUpdateRow != nil {
			callback = append(callback, CallbackTypeOnTableUpdateRow)
		}
		if template.OnTableDeleteRows != nil {
			callback = append(callback, CallbackTypeOnTableDeleteRows)
		}
		if (template.OnTableExportPlan == nil) != (template.OnTableExportChunk == nil) {
			errs = append(errs, fmt.Errorf("router %s must register OnTableExportPlan and OnTableExportChunk together", info.Router))
		}
		if template.OnTableExportPlan != nil && template.OnTableExportChunk != nil {
			callback = append(callback, CallbackTypeOnTableExportPlan, CallbackTypeOnTableExportChunk)
		}
		api.Schema = functionschema.NewTable(requestFields, responseFields, callback)

	case TemplateTypeForm:
		template, ok := info.Template.(*FormTemplate)
		if !ok {
			errs = append(errs, fmt.Errorf("router %s declares template type %q but template is not *FormTemplate", info.Router, templateType))
			break
		}
		fields, responseFields, err := widget.DecodeForm(fieldsCallback, base.Request, base.Response)
		if err != nil {
			errs = append(errs, fmt.Errorf("router %s form schema decode failed: %w", info.Router, err))
		}
		schedules, err := compileFormSchedules(info.Router, template.Schedules)
		if err != nil {
			errs = append(errs, err)
		}
		api.Schedules = schedules
		api.Schema = functionschema.NewForm(fields, responseFields, nil)

	case TemplateTypeChart:
		template, ok := info.Template.(*ChartTemplate)
		if !ok {
			errs = append(errs, fmt.Errorf("router %s declares template type %q but template is not *ChartTemplate", info.Router, templateType))
			break
		}
		warnChartTemplateType(info.Router, template.ChartType)
		fields, responseFields, err := widget.DecodeForm(fieldsCallback, base.Request, base.Response)
		if err != nil {
			errs = append(errs, fmt.Errorf("router %s chart schema decode failed: %w", info.Router, err))
		}
		api.Schema = functionschema.NewChartWithType(template.ChartType, fields, responseFields, nil)
	default:
		errs = append(errs, fmt.Errorf("router %s has unsupported template type: %s", info.Router, templateType))
	}

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	api.CreateTableModels = base.CreateTables
	// 提取创建表的名称
	createTables := make([]interface{}, 0, len(base.CreateTables))
	for _, createTable := range base.CreateTables {
		if createTable == nil {
			continue
		}
		createTables = append(createTables, createTable)

		// 使用GORM的Tabler接口获取表名
		if tabler, ok := createTable.(interface{ TableName() string }); ok {
			api.CreateTables = append(api.CreateTables, tabler.TableName())
		}
	}

	return api, createTables, nil
}

// collectPackageInfos 收集当前应用的全量 package 信息
// 每次 update 都会调用，返回的全量列表供 app-server 做目录对账
func (a *App) collectPackageInfos() ([]*PackageInfo, error) {
	seen := make(map[string]*PackageInfo)
	compiledDocs := make(map[string][]CompiledDocManifest)

	addPackagePath := func(pkgPath string) {
		pkgPath = strings.Trim(pkgPath, "/")
		if pkgPath == "" {
			return
		}
		parts := strings.Split(pkgPath, "/")

		for i := 1; i <= len(parts); i++ {
			subPath := strings.Join(parts[:i], "/")
			if _, exists := seen[subPath]; exists {
				continue
			}

			code := parts[i-1]
			seen[subPath] = &PackageInfo{
				Code:        code,
				Name:        code,
				RouterGroup: "/" + subPath,
				FullPath:    fmt.Sprintf("/%s/%s/%s", env.User, env.App, subPath),
			}
		}
	}

	for _, info := range a.routerInfo {
		if info.Options == nil || info.Options.PackagePath == "" {
			continue
		}

		addPackagePath(info.Options.PackagePath)
	}

	for pkgPath, packageContext := range a.packageContexts {
		addPackagePath(pkgPath)
		if packageContext == nil {
			continue
		}

		docs, err := compileDocManifests("/"+strings.Trim(pkgPath, "/"), packageContext.Docs)
		if err != nil {
			return nil, err
		}
		compiledDocs[pkgPath] = docs
		for _, doc := range docs {
			parentPath := path.Dir(doc.Code)
			if parentPath != "." {
				addPackagePath(strings.Trim(pkgPath, "/") + "/" + parentPath)
			}
		}
	}

	for subPath, info := range seen {
		pc, ok := a.packageContexts[subPath]
		if !ok || pc == nil {
			continue
		}
		if pc.Name != "" {
			info.Name = pc.Name
		}
		if pc.Desc != "" {
			info.Desc = pc.Desc
		}
		tasks, err := compileAgentTasks(info.RouterGroup, pc.AgentTasks)
		if err != nil {
			return nil, err
		}
		info.AgentTasks = tasks
		info.Docs = compiledDocs[subPath]
	}

	result := make([]*PackageInfo, 0, len(seen))
	for _, info := range seen {
		result = append(result, info)
	}

	sort.Slice(result, func(i, j int) bool {
		leftDepth := strings.Count(result[i].FullPath, "/")
		rightDepth := strings.Count(result[j].FullPath, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return result[i].FullPath < result[j].FullPath
	})

	return result, nil
}

// onAppUpdate 处理当api更新时候触发
func (a *App) onAppUpdate(msg *nats.Msg) {
	ctx := context.Background()

	// panic 保护：捕获任何 panic 并记录到日志，避免 goroutine 静默死亡
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf(ctx, "[onAppUpdate] ❌ PANIC recovered: %v", r)
			a.respondUpdateError(msg, fmt.Sprintf("onAppUpdate panic: %v", r))
		}
	}()

	logger.Infof(ctx, "OnAppUpdate received: %s, Reply: %s", msg.Subject, msg.Reply)

	// 检查是否有 Reply subject（Request/Reply 模式）
	if msg.Reply == "" {
		logger.Warnf(ctx, "OnAppUpdate: No reply subject, cannot respond")
		return
	}

	dbCapability := appDBCapabilityFromControlMessage(msg)
	if err := a.executeOnAppUpdate(ctx, msg, dbCapability); err != nil {
		logger.Errorf(ctx, "[onAppUpdate] FAILED: %v", err)
		a.respondUpdateError(msg, err.Error())
		return
	}

	logger.Infof(ctx, "[onAppUpdate] ✅ All done! Response sent successfully")
}

func (a *App) executeOnAppUpdate(ctx context.Context, msg *nats.Msg, dbCapability *dto.AppDBCapability) (err error) {
	startedAt := time.Now()
	defer func() {
		if err != nil {
			logger.Errorf(ctx, "[onAppUpdate][timing] phase=total status=error duration_ms=%d error=%v", time.Since(startedAt).Milliseconds(), err)
			return
		}
		logger.Infof(ctx, "[onAppUpdate][timing] phase=total status=ok duration_ms=%d", time.Since(startedAt).Milliseconds())
	}()

	currentApis, err := a.loadCurrentApisForUpdate(ctx)
	if err != nil {
		return err
	}

	if err := a.migrateUpdateDatabases(ctx, currentApis, dbCapability); err != nil {
		return err
	}

	if err := a.persistCurrentUpdateVersion(ctx, currentApis); err != nil {
		return err
	}

	diffData, err := a.buildUpdateDiffData(ctx, currentApis)
	if err != nil {
		return err
	}

	if err := a.runOnAPICreateCallbacks(ctx, diffData.Add, dbCapability); err != nil {
		return err
	}

	return a.respondUpdateSuccess(ctx, msg, diffData)
}

func (a *App) loadCurrentApisForUpdate(ctx context.Context) ([]*ApiInfo, error) {
	startedAt := time.Now()
	logger.Infof(ctx, "[onAppUpdate] Step 1: Getting current APIs...")
	currentApis, _, err := a.getApis()
	if err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 1 FAILED: duration_ms=%d error=%v", time.Since(startedAt).Milliseconds(), err)
		return nil, fmt.Errorf("Failed to get current APIs: %v", err)
	}
	logger.Infof(ctx, "[onAppUpdate] Step 1 OK: got %d APIs duration_ms=%d", len(currentApis), time.Since(startedAt).Milliseconds())
	return currentApis, nil
}

func (a *App) migrateUpdateDatabases(ctx context.Context, currentApis []*ApiInfo, dbCapability *dto.AppDBCapability) error {
	startedAt := time.Now()
	// Always reconcile the models declared by CreateTables against the real
	// database on every update. A source/workplace fingerprint alone cannot
	// prove that the database was not recreated, switched, or changed outside
	// this process, so it must never be used to skip this reconciliation.
	prepareStartedAt := time.Now()
	groups := groupUpdateDatabaseAPIs(ctx, currentApis)
	groups = prepareUpdateDatabaseMigrationGroups(groups)
	modelCount := 0
	for _, group := range groups {
		modelCount += len(group.models)
	}
	prepareDuration := time.Since(prepareStartedAt)
	logger.Infof(
		ctx,
		"[onAppUpdate] Step 2: Initializing databases for %d packages and %d unique models (deduplicated by CreateTables model, concurrency=%d, prepare_ms=%d)...",
		len(groups),
		modelCount,
		updateDatabaseMigrationConcurrency,
		prepareDuration.Milliseconds(),
	)
	err := runUpdateDatabaseMigrationGroups(groups, updateDatabaseMigrationConcurrency, func(group updateDatabaseMigrationGroup) error {
		return a.migrateUpdateDatabaseGroup(ctx, group, dbCapability)
	})
	if err != nil {
		logger.Errorf(ctx, "[onAppUpdate][timing] phase=database_migration status=error packages=%d unique_models=%d duration_ms=%d error=%v",
			len(groups), modelCount, time.Since(startedAt).Milliseconds(), err)
		return err
	}
	logger.Infof(ctx, "[onAppUpdate] Step 2 OK: databases initialized packages=%d unique_models=%d duration_ms=%d",
		len(groups), modelCount, time.Since(startedAt).Milliseconds())
	return nil
}

const updateDatabaseMigrationConcurrency = 6

type updateDatabaseMigrationAPI struct {
	index int
	api   *ApiInfo
}

type updateDatabaseMigrationGroup struct {
	packagePath string
	apis        []updateDatabaseMigrationAPI
	models      []interface{}
}

func groupUpdateDatabaseAPIs(ctx context.Context, currentApis []*ApiInfo) []updateDatabaseMigrationGroup {
	groupIndexes := make(map[string]int)
	groups := make([]updateDatabaseMigrationGroup, 0)
	for index, api := range currentApis {
		if api == nil || api.routerInfo == nil || api.routerInfo.Options == nil {
			name := ""
			if api != nil {
				name = api.Name
			}
			logger.Debugf(ctx, "[onAppUpdate] Step 2: API %d (%s) has no options, skipping DB init", index, name)
			continue
		}
		if len(api.CreateTableModels) == 0 {
			continue
		}

		packagePath := strings.Trim(api.routerInfo.Options.PackagePath, "/")
		groupIndex, ok := groupIndexes[packagePath]
		if !ok {
			groupIndex = len(groups)
			groupIndexes[packagePath] = groupIndex
			groups = append(groups, updateDatabaseMigrationGroup{packagePath: packagePath})
		}
		groups[groupIndex].apis = append(groups[groupIndex].apis, updateDatabaseMigrationAPI{index: index, api: api})
	}
	return groups
}

func prepareUpdateDatabaseMigrationGroups(groups []updateDatabaseMigrationGroup) []updateDatabaseMigrationGroup {
	prepared := make([]updateDatabaseMigrationGroup, 0, len(groups))
	for _, group := range groups {
		models := uniqueUpdateDatabaseMigrationModels(group)
		if len(models) == 0 {
			continue
		}
		group.models = models
		prepared = append(prepared, group)
	}
	// Start packages with more models first. This shortens the worker-pool tail
	// without skipping or reordering models inside a package.
	sort.SliceStable(prepared, func(i, j int) bool {
		if len(prepared[i].models) == len(prepared[j].models) {
			return prepared[i].packagePath < prepared[j].packagePath
		}
		return len(prepared[i].models) > len(prepared[j].models)
	})
	return prepared
}

func uniqueUpdateDatabaseMigrationModels(group updateDatabaseMigrationGroup) []interface{} {
	// GORM derives AutoMigrate metadata from the Go model type. Keep every
	// distinct type (even when two types map to the same table), and collapse
	// only exact repeats caused by multiple APIs declaring the same model in
	// CreateTables.
	seenTypes := make(map[reflect.Type]struct{})
	models := make([]interface{}, 0)

	for _, item := range group.apis {
		if item.api == nil {
			continue
		}
		for _, model := range item.api.CreateTableModels {
			if model == nil {
				continue
			}
			modelType := reflect.TypeOf(model)
			if _, exists := seenTypes[modelType]; exists {
				continue
			}

			seenTypes[modelType] = struct{}{}
			models = append(models, model)
		}
	}
	return models
}

func (a *App) migrateUpdateDatabaseGroup(ctx context.Context, group updateDatabaseMigrationGroup, dbCapability *dto.AppDBCapability) error {
	startedAt := time.Now()
	logger.Debugf(ctx, "[onAppUpdate] Step 2: opening MySQL app DB for package: %s (apis=%d, unique_models=%d)", group.packagePath, len(group.apis), len(group.models))
	dbStartedAt := time.Now()
	db, err := getOrInitMySQLMigrationDB(group.packagePath, dbCapability)
	dbDuration := time.Since(dbStartedAt)
	if err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 2 FAILED: get MySQL app DB for package=%s duration_ms=%d error=%v", group.packagePath, dbDuration.Milliseconds(), err)
		return fmt.Errorf("Failed to get DB: %v", err)
	}

	migrateStartedAt := time.Now()
	for _, model := range group.models {
		modelStartedAt := time.Now()
		if err := db.AutoMigrate(model); err != nil {
			logger.Errorf(ctx, "[onAppUpdate] Step 2 FAILED: AutoMigrate package=%s model=%T duration_ms=%d error=%v",
				group.packagePath, model, time.Since(modelStartedAt).Milliseconds(), err)
			return fmt.Errorf("Failed to migrate table: %v", err)
		}
		modelDuration := time.Since(modelStartedAt)
		if modelDuration >= time.Second {
			logger.Warnf(ctx, "[onAppUpdate][timing] phase=auto_migrate package=%s model=%T duration_ms=%d",
				group.packagePath, model, modelDuration.Milliseconds())
		} else {
			logger.Debugf(ctx, "[onAppUpdate][timing] phase=auto_migrate package=%s model=%T duration_ms=%d",
				group.packagePath, model, modelDuration.Milliseconds())
		}
	}
	migrateDuration := time.Since(migrateStartedAt)
	logger.Infof(ctx,
		"[onAppUpdate][timing] phase=database_package status=ok package=%s apis=%d unique_models=%d db_open_ms=%d migrate_ms=%d duration_ms=%d",
		group.packagePath,
		len(group.apis),
		len(group.models),
		dbDuration.Milliseconds(),
		migrateDuration.Milliseconds(),
		time.Since(startedAt).Milliseconds(),
	)

	return nil
}

func runUpdateDatabaseMigrationGroups(
	groups []updateDatabaseMigrationGroup,
	concurrency int,
	migrate func(updateDatabaseMigrationGroup) error,
) error {
	if len(groups) == 0 {
		return nil
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(groups) {
		concurrency = len(groups)
	}

	var (
		workers  sync.WaitGroup
		mu       sync.Mutex
		next     int
		firstErr error
	)
	workers.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer workers.Done()
			for {
				mu.Lock()
				if firstErr != nil || next >= len(groups) {
					mu.Unlock()
					return
				}
				group := groups[next]
				next++
				mu.Unlock()

				if err := migrate(group); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
			}
		}()
	}
	workers.Wait()
	return firstErr
}

func (a *App) persistCurrentUpdateVersion(ctx context.Context, currentApis []*ApiInfo) error {
	startedAt := time.Now()
	logger.Infof(ctx, "[onAppUpdate] Step 3: Saving current version...")
	if err := a.saveCurrentVersion(currentApis); err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 3 FAILED: duration_ms=%d error=%v", time.Since(startedAt).Milliseconds(), err)
		return fmt.Errorf("Failed to save current version: %v", err)
	}
	logger.Infof(ctx, "[onAppUpdate] Step 3 OK: version saved duration_ms=%d", time.Since(startedAt).Milliseconds())
	return nil
}

func (a *App) buildUpdateDiffData(ctx context.Context, currentApis []*ApiInfo) (*DiffData, error) {
	startedAt := time.Now()
	logger.Infof(ctx, "[onAppUpdate] Step 4: Diffing APIs...")
	diffStartedAt := time.Now()
	add, update, del, err := a.diffApiWithCurrentApis(currentApis)
	diffDuration := time.Since(diffStartedAt)
	if err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 4 FAILED: duration_ms=%d error=%v", diffDuration.Milliseconds(), err)
		return nil, fmt.Errorf("Failed to diff APIs: %v", err)
	}
	logger.Infof(ctx, "[onAppUpdate] Step 4 OK: add=%d, update=%d, delete=%d duration_ms=%d", len(add), len(update), len(del), diffDuration.Milliseconds())

	packageStartedAt := time.Now()
	packages, err := a.collectPackageInfos()
	packageDuration := time.Since(packageStartedAt)
	if err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 5 FAILED: duration_ms=%d error=%v", packageDuration.Milliseconds(), err)
		return nil, fmt.Errorf("Failed to collect packages: %v", err)
	}
	logger.Infof(ctx, "[onAppUpdate] Step 5: collected %d packages for reconciliation duration_ms=%d total_diff_build_ms=%d",
		len(packages), packageDuration.Milliseconds(), time.Since(startedAt).Milliseconds())

	return &DiffData{
		Add:      add,
		Update:   update,
		Delete:   del,
		Packages: packages,
	}, nil
}

func (a *App) runOnAPICreateCallbacks(ctx context.Context, addedApis []*ApiInfo, dbCapability *dto.AppDBCapability) error {
	startedAt := time.Now()
	logger.Infof(ctx, "[onAppUpdate] Step 5: Running OnApiCreate callbacks...")
	for _, aa := range addedApis {
		if err := a.runOnAPICreateCallback(ctx, aa, dbCapability); err != nil {
			logger.Errorf(ctx, "[onAppUpdate][timing] phase=on_api_create status=error api_count=%d duration_ms=%d error=%v",
				len(addedApis), time.Since(startedAt).Milliseconds(), err)
			return err
		}
	}
	logger.Infof(ctx, "[onAppUpdate] Step 5 OK: callbacks done api_count=%d duration_ms=%d", len(addedApis), time.Since(startedAt).Milliseconds())
	return nil
}

func (a *App) runOnAPICreateCallback(ctx context.Context, api *ApiInfo, dbCapability *dto.AppDBCapability) error {
	router, err := a.getRoute(api.Router)
	if err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 5 FAILED: getRoute(%s): %v", api.Router, err)
		return fmt.Errorf("Failed to get router: %v", err)
	}

	create := router.Template.GetBaseConfig().OnApiCreate
	if create == nil {
		return nil
	}

	var req callback.OnApiCreateReq
	if _, err := create(newCallbackContext(router, dbCapability), &req); err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 5 FAILED: OnApiCreate(%s): %v", api.Router, err)
		return fmt.Errorf("Failed to create api: %v", err)
	}

	return nil
}

func appDBCapabilityFromControlMessage(msg *nats.Msg) *dto.AppDBCapability {
	if msg == nil || len(msg.Data) == 0 {
		return nil
	}
	var message subjects.Message
	if err := json.Unmarshal(msg.Data, &message); err != nil {
		return nil
	}
	data, err := json.Marshal(message.Data)
	if err != nil {
		return nil
	}
	var payload struct {
		DBCapability *dto.AppDBCapability `json:"db_capability"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return cloneDBCapability(payload.DBCapability)
}

func (a *App) respondUpdateSuccess(ctx context.Context, msg *nats.Msg, diffData *DiffData) error {
	startedAt := time.Now()
	logger.Infof(ctx, "[onAppUpdate] Step 6: Sending success response...")
	if err := a.transport.RespondUpdateSuccess(msg, diffData); err != nil {
		logger.Errorf(ctx, "[onAppUpdate] Step 6 FAILED: duration_ms=%d error=%v", time.Since(startedAt).Milliseconds(), err)
		return err
	}
	logger.Infof(ctx, "[onAppUpdate] Step 6 OK: response sent duration_ms=%d", time.Since(startedAt).Milliseconds())
	return nil
}

func (a *App) respondUpdateError(msg *nats.Msg, message string) {
	logger.Errorf(context.Background(), "[respondUpdateError] Sending error: %s", message)
	if err := a.transport.RespondUpdateError(msg, message); err != nil {
		logger.Errorf(context.Background(), "[respondUpdateError] Failed: %v", err)
		return
	}
	logger.Infof(context.Background(), "[respondUpdateError] Error response sent successfully")
}
