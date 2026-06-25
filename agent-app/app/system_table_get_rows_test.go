package app

import (
	"reflect"
	"strings"
	"testing"

	"github.com/kageos/kageos-sdk/agent-app/callback"
)

func TestSystemTableGetRowsCallbackRequiresAutoCrudTable(t *testing.T) {
	t.Parallel()

	resp, err := handleSystemTableGetRows(&Context{}, &TableTemplate{}, &callback.TableGetRowsReq{IDs: []int64{1}})
	if err == nil {
		t.Fatal("handleSystemTableGetRows() error = nil, want AutoCrudTable error")
	}
	if resp != nil {
		t.Fatalf("handleSystemTableGetRows() resp = %#v, want nil", resp)
	}
	if !strings.Contains(err.Error(), "AutoCrudTable") {
		t.Fatalf("handleSystemTableGetRows() error = %v, want AutoCrudTable error", err)
	}
}

func TestSystemTableGetRowsRowsSlicePtrUsesModelStruct(t *testing.T) {
	t.Parallel()

	rowsPtr, err := newRowsSlicePtr(&compileTestTableModel{})
	if err != nil {
		t.Fatalf("newRowsSlicePtr() error = %v, want nil", err)
	}
	if rowsPtr.Kind().String() != "ptr" || rowsPtr.Elem().Kind().String() != "slice" {
		t.Fatalf("newRowsSlicePtr() = %s -> %s, want pointer to slice", rowsPtr.Kind(), rowsPtr.Elem().Kind())
	}
	if rowsPtr.Elem().Type().Elem() != reflect.TypeOf(compileTestTableModel{}) {
		t.Fatalf("rows element type = %s, want compileTestTableModel", rowsPtr.Elem().Type().Elem())
	}
}

func TestSystemTableGetRowsCallbackIsNotExportedInSchema(t *testing.T) {
	t.Parallel()

	testApp := newCompileTestApp("/demo/list.table", &TableTemplate{
		BaseConfig: BaseConfig{
			Request: compileTestTableReq{},
		},
		AutoCrudTable: &compileTestTableModel{},
	})
	apis, _, err := testApp.getApis()
	if err != nil {
		t.Fatalf("getApis() error = %v, want nil", err)
	}
	if len(apis) != 1 || apis[0].Schema == nil {
		t.Fatalf("getApis() = %#v, want one API with schema", apis)
	}
	for _, callbackType := range apis[0].Schema.Callbacks {
		if callbackType == CallbackTypeSystemTableGetRows {
			t.Fatalf("schema callbacks exported private callback %q: %#v", CallbackTypeSystemTableGetRows, apis[0].Schema.Callbacks)
		}
	}
}
