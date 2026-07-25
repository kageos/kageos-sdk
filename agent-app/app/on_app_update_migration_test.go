package app

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type updateMigrationTestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:64;index"`
}

type updateMigrationChangedTestModel struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:128;index"`
}

type updateMigrationSameTableV1 struct {
	ID uint `gorm:"primaryKey"`
}

func (updateMigrationSameTableV1) TableName() string {
	return "update_migration_same_table"
}

type updateMigrationSameTableV2 struct {
	ID      uint   `gorm:"primaryKey"`
	Details string `gorm:"size:255"`
}

func (updateMigrationSameTableV2) TableName() string {
	return "update_migration_same_table"
}

func TestGroupUpdateDatabaseAPIsGroupsByPackage(t *testing.T) {
	api := func(name, packagePath string) *ApiInfo {
		return &ApiInfo{
			Name:              name,
			CreateTableModels: []interface{}{&updateMigrationTestModel{}},
			routerInfo: &routerInfo{
				Options: &RegisterOptions{PackagePath: packagePath},
			},
		}
	}

	groups := groupUpdateDatabaseAPIs(t.Context(), []*ApiInfo{
		api("orders", "/sales/"),
		api("payments", "sales"),
		api("tickets", "support"),
		nil,
	})
	if len(groups) != 2 {
		t.Fatalf("expected 2 package groups, got %d: %#v", len(groups), groups)
	}
	if groups[0].packagePath != "sales" || len(groups[0].apis) != 2 {
		t.Fatalf("unexpected first group: %#v", groups[0])
	}
	if groups[1].packagePath != "support" || len(groups[1].apis) != 1 {
		t.Fatalf("unexpected second group: %#v", groups[1])
	}
}

func TestPrepareUpdateDatabaseMigrationGroupsDeduplicatesModels(t *testing.T) {
	api := func(name string, models ...interface{}) updateDatabaseMigrationAPI {
		return updateDatabaseMigrationAPI{
			api: &ApiInfo{
				Name:              name,
				CreateTableModels: models,
			},
		}
	}
	groups := prepareUpdateDatabaseMigrationGroups([]updateDatabaseMigrationGroup{
		{
			packagePath: "sales",
			apis: []updateDatabaseMigrationAPI{
				api("orders", &updateMigrationTestModel{}, &updateMigrationChangedTestModel{}),
				api("payments", &updateMigrationTestModel{}, &updateMigrationChangedTestModel{}),
			},
		},
	})
	if len(groups) != 1 {
		t.Fatalf("expected one group, got %#v", groups)
	}
	if got := len(groups[0].models); got != 2 {
		t.Fatalf("expected two unique models, got %d", got)
	}
}

func TestPrepareUpdateDatabaseMigrationGroupsKeepsDistinctModelsForSameTable(t *testing.T) {
	groups := prepareUpdateDatabaseMigrationGroups([]updateDatabaseMigrationGroup{
		{
			packagePath: "sales",
			apis: []updateDatabaseMigrationAPI{{
				api: &ApiInfo{
					CreateTableModels: []interface{}{
						&updateMigrationSameTableV1{},
						&updateMigrationSameTableV2{},
					},
				},
			}},
		},
	})
	if len(groups) != 1 {
		t.Fatalf("expected one group, got %#v", groups)
	}
	if got := len(groups[0].models); got != 2 {
		t.Fatalf("expected both schemas for the same table to be retained, got %d", got)
	}
}

func TestPrepareUpdateDatabaseMigrationGroupsSchedulesHeavierPackagesFirst(t *testing.T) {
	group := func(packagePath string, models ...interface{}) updateDatabaseMigrationGroup {
		return updateDatabaseMigrationGroup{
			packagePath: packagePath,
			apis: []updateDatabaseMigrationAPI{{
				api: &ApiInfo{CreateTableModels: models},
			}},
		}
	}

	groups := prepareUpdateDatabaseMigrationGroups([]updateDatabaseMigrationGroup{
		group("light", &updateMigrationTestModel{}),
		group("heavy",
			&updateMigrationTestModel{},
			&updateMigrationChangedTestModel{},
			&updateMigrationSameTableV1{},
		),
		group("medium", &updateMigrationTestModel{}, &updateMigrationChangedTestModel{}),
	})

	if len(groups) != 3 {
		t.Fatalf("expected three groups, got %#v", groups)
	}
	if groups[0].packagePath != "heavy" || groups[1].packagePath != "medium" || groups[2].packagePath != "light" {
		t.Fatalf("unexpected migration order: %s, %s, %s",
			groups[0].packagePath, groups[1].packagePath, groups[2].packagePath)
	}
}

func TestInternalCreateTableIsPreparedWhenPublicAPISchemaIsUnchanged(t *testing.T) {
	previous := &ApiInfo{
		Name:         "orders",
		CreateTables: []string{"orders"},
	}
	current := &ApiInfo{
		Name:              "orders",
		CreateTables:      []string{"orders"},
		CreateTableModels: []interface{}{&updateMigrationTestModel{}, &updateMigrationChangedTestModel{}},
		routerInfo: &routerInfo{
			Options: &RegisterOptions{PackagePath: "sales"},
		},
	}
	if !previous.IsEqual(current) {
		t.Fatal("expected the public API schema to remain unchanged")
	}

	for update := 1; update <= 2; update++ {
		groups := prepareUpdateDatabaseMigrationGroups(groupUpdateDatabaseAPIs(t.Context(), []*ApiInfo{current}))
		if len(groups) != 1 {
			t.Fatalf("update %d: expected one migration group, got %#v", update, groups)
		}
		if got := len(groups[0].models); got != 2 {
			t.Fatalf("update %d: expected both CreateTables models to be migrated, got %d", update, got)
		}
	}
}

func TestRunUpdateDatabaseMigrationGroupsBoundsConcurrency(t *testing.T) {
	groups := make([]updateDatabaseMigrationGroup, 12)
	for i := range groups {
		groups[i].packagePath = string(rune('a' + i))
	}

	var active, maxActive, completed atomic.Int32
	err := runUpdateDatabaseMigrationGroups(groups, 4, func(updateDatabaseMigrationGroup) error {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			old := maxActive.Load()
			if current <= old || maxActive.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		completed.Add(1)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Load() != int32(len(groups)) {
		t.Fatalf("expected %d completed groups, got %d", len(groups), completed.Load())
	}
	if got := maxActive.Load(); got < 2 || got > 4 {
		t.Fatalf("expected concurrency in [2,4], got %d", got)
	}
}

func TestRunUpdateDatabaseMigrationGroupsReturnsFirstError(t *testing.T) {
	wantErr := errors.New("migration failed")
	groups := []updateDatabaseMigrationGroup{{packagePath: "a"}, {packagePath: "b"}}
	err := runUpdateDatabaseMigrationGroups(groups, 1, func(group updateDatabaseMigrationGroup) error {
		if group.packagePath == "a" {
			return wantErr
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}
