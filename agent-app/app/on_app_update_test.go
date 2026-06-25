package app

import (
	"testing"

	"github.com/kageos/kageos-sdk/agent-app/env"
)

func TestCollectPackageInfosSortsByDepthThenPath(t *testing.T) {
	oldUser, oldApp := env.User, env.App
	env.User, env.App = "alice", "demo"
	defer func() {
		env.User, env.App = oldUser, oldApp
	}()

	app := &App{
		routerInfo: map[string]*routerInfo{
			"/zeta/run": {
				Options: &RegisterOptions{PackagePath: "zeta"},
			},
			"/alpha/beta/run": {
				Options: &RegisterOptions{PackagePath: "alpha/beta"},
			},
			"/alpha/gamma/run": {
				Options: &RegisterOptions{PackagePath: "alpha/gamma"},
			},
		},
		packageContexts: map[string]*PackageContext{
			"alpha": {Name: "Alpha", Desc: `alpha package`},
		},
	}

	got, err := app.collectPackageInfos()
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{
		"/alice/demo/alpha",
		"/alice/demo/zeta",
		"/alice/demo/alpha/beta",
		"/alice/demo/alpha/gamma",
	}
	if len(got) != len(wantPaths) {
		t.Fatalf("expected %d packages, got %d: %#v", len(wantPaths), len(got), got)
	}

	for i, want := range wantPaths {
		if got[i].FullPath != want {
			t.Fatalf("package %d: want %s, got %s", i, want, got[i].FullPath)
		}
	}
	if got[0].Name != "Alpha" || got[0].Desc != "alpha package" {
		t.Fatalf("expected package context metadata to be applied, got %+v", got[0])
	}
}

func TestCollectPackageInfosIncludesAgentTasksFromPackageContext(t *testing.T) {
	oldUser, oldApp := env.User, env.App
	env.User, env.App = "alice", "demo"
	defer func() {
		env.User, env.App = oldUser, oldApp
	}()

	app := &App{
		routerInfo: map[string]*routerInfo{
			"/gold_watch/sweep.form": {
				Options: &RegisterOptions{PackagePath: "gold_watch"},
			},
		},
		packageContexts: map[string]*PackageContext{
			"gold_watch": {
				Name: "黄金盯盘助手",
				AgentTasks: []AgentTask{
					{
						Code:         "daily_report",
						Title:        "黄金观察日报",
						Message:      "读取观察清单、行情快照和提醒记录，生成日报。",
						CronExpr:     "0 8 * * *",
						Timezone:     "Asia/Shanghai",
						Enabled:      false,
						ModeCode:     "operator",
						LLMConfigID:  7,
						EverySeconds: 0,
					},
				},
			},
		},
	}

	got, err := app.collectPackageInfos()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one package, got %#v", got)
	}
	if got[0].FullPath != "/alice/demo/gold_watch" || len(got[0].AgentTasks) != 1 {
		t.Fatalf("package agent tasks not collected: %#v", got[0])
	}
	task := got[0].AgentTasks[0]
	if task.Code != "daily_report" || task.Message == "" || task.CronExpr != "0 8 * * *" || task.Enabled {
		t.Fatalf("unexpected agent task: %#v", task)
	}
}

func TestCollectPackageInfosRejectsInvalidAgentTask(t *testing.T) {
	oldUser, oldApp := env.User, env.App
	env.User, env.App = "alice", "demo"
	defer func() {
		env.User, env.App = oldUser, oldApp
	}()

	app := &App{
		packageContexts: map[string]*PackageContext{
			"gold_watch": {
				AgentTasks: []AgentTask{
					{
						Code:         "bad",
						Message:      "bad schedule",
						CronExpr:     "0 8 * * *",
						EverySeconds: 3600,
					},
				},
			},
		},
	}

	if _, err := app.collectPackageInfos(); err == nil {
		t.Fatal("expected invalid agent task error")
	}
}

func TestSortApiInfosByKey(t *testing.T) {
	apis := []*ApiInfo{
		{Method: "POST", Router: "/ticket/create"},
		{Method: "GET", Router: "/ticket/list"},
		{Method: "DELETE", Router: "/ticket/delete"},
	}

	sortApiInfosByKey(apis)

	want := []string{
		"DELETE:/ticket/delete",
		"GET:/ticket/list",
		"POST:/ticket/create",
	}
	for i, api := range apis {
		got := api.Method + ":" + api.Router
		if got != want[i] {
			t.Fatalf("api %d: want %s, got %s", i, want[i], got)
		}
	}
}
