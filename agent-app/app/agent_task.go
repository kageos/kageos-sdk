package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/kageos/kageos-sdk/pkg/logger"
)

// AgentTask describes a default scheduled workspace agent session owned by a package.
// It is declarative metadata only; timer-scheduler owns runtime state.
type AgentTask struct {
	Code               string `json:"code"`
	Title              string `json:"title,omitempty"`
	Description        string `json:"description,omitempty"`
	Message            string `json:"message"`
	Enabled            bool   `json:"enabled,omitempty"`
	EverySeconds       int64  `json:"every_seconds,omitempty"`
	CronExpr           string `json:"cron_expr,omitempty"`
	Timezone           string `json:"timezone,omitempty"`
	MaxRuns            int    `json:"max_runs,omitempty"`
	ModeCode           string `json:"mode_code,omitempty"`
	Files              string `json:"files,omitempty"`
	LLMConfigID        int64  `json:"llm_config_id,omitempty"`
	MaxDurationSeconds int64  `json:"max_duration_seconds,omitempty"`
}

type CompiledAgentTask struct {
	Code               string `json:"code"`
	Title              string `json:"title,omitempty"`
	Description        string `json:"description,omitempty"`
	Message            string `json:"message"`
	Enabled            bool   `json:"enabled,omitempty"`
	EverySeconds       int64  `json:"every_seconds,omitempty"`
	CronExpr           string `json:"cron_expr,omitempty"`
	Timezone           string `json:"timezone,omitempty"`
	MaxRuns            int    `json:"max_runs,omitempty"`
	ModeCode           string `json:"mode_code,omitempty"`
	Files              string `json:"files,omitempty"`
	LLMConfigID        int64  `json:"llm_config_id,omitempty"`
	MaxDurationSeconds int64  `json:"max_duration_seconds,omitempty"`
}

func (p *PackageContext) AddAgentTask(task AgentTask) {
	if p == nil {
		panic("PackageContext.AddAgentTask called on nil PackageContext")
	}
	if app == nil {
		initApp()
	}
	if app == nil {
		logger.Errorf(context.Background(), "Cannot add agent task %s: app initialization failed", task.Code)
		return
	}
	packagePath := strings.Trim(p.RouterGroup, "/")
	if packagePath == "" {
		panic("PackageContext.AddAgentTask requires RouterGroup")
	}
	p.AgentTasks = append(p.AgentTasks, task)
	app.packageContexts[packagePath] = p
}

func compileAgentTasks(routerGroup string, tasks []AgentTask) ([]CompiledAgentTask, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	out := make([]CompiledAgentTask, 0, len(tasks))
	seen := map[string]struct{}{}
	for i, task := range tasks {
		compiled, err := compileAgentTask(routerGroup, i, task)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[compiled.Code]; exists {
			return nil, fmt.Errorf("%s agent task code %q is duplicated", routerGroup, compiled.Code)
		}
		seen[compiled.Code] = struct{}{}
		out = append(out, compiled)
	}
	return out, nil
}

func compileAgentTask(routerGroup string, index int, task AgentTask) (CompiledAgentTask, error) {
	code := strings.TrimSpace(task.Code)
	if code == "" {
		return CompiledAgentTask{}, fmt.Errorf("%s agent task #%d code is required", routerGroup, index+1)
	}
	message := strings.TrimSpace(task.Message)
	if message == "" {
		return CompiledAgentTask{}, fmt.Errorf("%s agent task %q message is required", routerGroup, code)
	}
	cronExpr := strings.TrimSpace(task.CronExpr)
	hasCron := cronExpr != ""
	hasEvery := task.EverySeconds > 0
	if hasCron == hasEvery {
		return CompiledAgentTask{}, fmt.Errorf("%s agent task %q must set exactly one of cron_expr or every_seconds", routerGroup, code)
	}
	return CompiledAgentTask{
		Code:               code,
		Title:              strings.TrimSpace(task.Title),
		Description:        strings.TrimSpace(task.Description),
		Message:            message,
		Enabled:            task.Enabled,
		EverySeconds:       task.EverySeconds,
		CronExpr:           cronExpr,
		Timezone:           strings.TrimSpace(task.Timezone),
		MaxRuns:            task.MaxRuns,
		ModeCode:           strings.TrimSpace(task.ModeCode),
		Files:              strings.TrimSpace(task.Files),
		LLMConfigID:        task.LLMConfigID,
		MaxDurationSeconds: task.MaxDurationSeconds,
	}, nil
}
