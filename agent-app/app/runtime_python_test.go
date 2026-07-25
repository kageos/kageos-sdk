package app

import (
	"testing"

	pythonRuntime "github.com/kageos/kageos-sdk/agent-app/runtime/python"
)

func TestRuntimePythonResponseFilesPreservesDeclaredNames(t *testing.T) {
	files, err := runtimePythonResponseFiles(
		[]string{"/output/generated-12345.xlsx"},
		[]pythonRuntime.PythonArtifact{{
			Path: "/output/generated-12345.xlsx",
			Name: "商品导入模板_已填充.xlsx",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected one response file, got %d", len(files))
	}
	if files[0].Path != "/output/generated-12345.xlsx" || files[0].Name != "商品导入模板_已填充.xlsx" {
		t.Fatalf("unexpected response file: %#v", files[0])
	}
}

func TestRuntimePythonResponseFilesRejectsExtensionMismatch(t *testing.T) {
	_, err := runtimePythonResponseFiles(
		[]string{"/output/report.xlsx"},
		[]pythonRuntime.PythonArtifact{{Path: "/output/report.xlsx", Name: "report.csv"}},
	)
	if err == nil {
		t.Fatal("expected extension mismatch to fail")
	}
}

func TestRuntimeValidateUploadedFileCount(t *testing.T) {
	if err := runtimeValidateUploadedFileCount("kageos/a.xlsx,kageos/b.xlsx", 2); err != nil {
		t.Fatal(err)
	}
	if err := runtimeValidateUploadedFileCount("kageos/a.xlsx", 2); err == nil {
		t.Fatal("expected partial upload to fail")
	}
}
