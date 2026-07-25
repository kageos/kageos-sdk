package app

import "testing"

func TestNormalizeDocManifestCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "root doc", code: "runbook", want: "runbook.docs"},
		{name: "root doc suffix", code: "runbook.docs", want: "runbook.docs"},
		{name: "nested relative doc", code: "./docs/readme", want: "docs/readme.docs"},
		{name: "nested relative doc suffix", code: "./docs/readme.docs", want: "docs/readme.docs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeDocManifestCode(tt.code)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestNormalizeDocManifestCodeRejectsUnsafePaths(t *testing.T) {
	for _, code := range []string{
		"/docs/readme",
		"../readme",
		"docs/../readme",
		"docs/./readme",
		"docs//readme",
		`docs\readme`,
	} {
		t.Run(code, func(t *testing.T) {
			if _, err := normalizeDocManifestCode(code); err == nil {
				t.Fatalf("expected %q to be rejected", code)
			}
		})
	}
}

func TestCompileDocManifestsRejectsDuplicateNormalizedPaths(t *testing.T) {
	_, err := compileDocManifests("/mail", []DocManifest{
		{Code: "./docs/readme", Content: "first"},
		{Code: "docs/readme.docs", Content: "second"},
	})
	if err == nil {
		t.Fatal("expected duplicate normalized docs path error")
	}
}

func TestCompileNestedDocManifestDefaultsNameToDocumentCode(t *testing.T) {
	got, err := compileDocManifest("/mail", 0, DocManifest{
		Code:    "./docs/readme",
		Content: "# Readme\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "readme" {
		t.Fatalf("want default name %q, got %q", "readme", got.Name)
	}
}
