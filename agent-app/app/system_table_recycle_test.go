package app

import (
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"
)

type recycleTestRow struct {
	ID        int64          `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	DeletedBy string         `json:"-"`
}

func TestSystemRowsToVisibleMapsIncludesDeletionMetadata(t *testing.T) {
	deletedAt := time.Now().Add(-time.Hour)
	input := []recycleTestRow{{
		ID:        1,
		Name:      "deleted",
		DeletedAt: gorm.DeletedAt{Time: deletedAt, Valid: true},
		DeletedBy: "alice",
	}}
	rows, err := systemRowsToVisibleMaps(reflect.ValueOf(input))
	if err != nil {
		t.Fatalf("systemRowsToVisibleMaps: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %#v, want one row", rows)
	}
	if rows[0]["name"] != "deleted" || rows[0]["deleted_by"] != "alice" || rows[0]["deleted_at"] == nil {
		t.Fatalf("deleted row snapshot = %#v", rows[0])
	}
}

func TestNormalizeSystemDeletedRowsPage(t *testing.T) {
	tests := []struct {
		page, size         int
		wantPage, wantSize int
	}{
		{page: 0, size: 0, wantPage: 1, wantSize: 20},
		{page: 2, size: 50, wantPage: 2, wantSize: 50},
		{page: 1, size: 1000, wantPage: 1, wantSize: 100},
	}
	for _, test := range tests {
		page, size := normalizeSystemDeletedRowsPage(test.page, test.size)
		if page != test.wantPage || size != test.wantSize {
			t.Fatalf("normalize(%d, %d) = (%d, %d), want (%d, %d)", test.page, test.size, page, size, test.wantPage, test.wantSize)
		}
	}
}

func TestSystemTablePrivateRecycleCallbacksAreNotExported(t *testing.T) {
	testApp := newCompileTestApp("/demo/list.table", &TableTemplate{
		BaseConfig:    BaseConfig{Request: compileTestTableReq{}},
		AutoCrudTable: &compileTestTableModel{},
	})
	apis, _, err := testApp.getApis()
	if err != nil {
		t.Fatalf("getApis: %v", err)
	}
	for _, callbackType := range apis[0].Schema.Callbacks {
		if callbackType == CallbackTypeSystemTableGetDeletedRows || callbackType == CallbackTypeSystemTableRestoreRows {
			t.Fatalf("schema exported private recycle callback %q", callbackType)
		}
	}
}
