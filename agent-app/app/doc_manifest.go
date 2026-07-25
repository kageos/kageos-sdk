package app

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/kageos/kageos-sdk/pkg/logger"
)

const (
	DocCreateIfMissing = "create_if_missing"
)

// DocManifest describes package-owned seed docs created during app update.
// It is declarative metadata only; Service Tree docs remain the runtime source of truth.
type DocManifest struct {
	Code        string `json:"code"` // package 内相对路径，如 "runbook.docs" 或 "./docs/readme.docs"
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Tags        string `json:"tags,omitempty"`
	Content     string `json:"content"`
	Format      string `json:"format,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Policy      string `json:"policy,omitempty"`
}

type CompiledDocManifest struct {
	Code        string `json:"code"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Tags        string `json:"tags,omitempty"`
	Content     string `json:"content"`
	Format      string `json:"format,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Policy      string `json:"policy,omitempty"`
}

func (p *PackageContext) AddDocs(doc DocManifest) {
	if p == nil {
		panic("PackageContext.AddDocs called on nil PackageContext")
	}
	if app == nil {
		initApp()
	}
	if app == nil {
		logger.Errorf(context.Background(), "Cannot add docs %s: app initialization failed", doc.Code)
		return
	}
	packagePath := strings.Trim(p.RouterGroup, "/")
	if packagePath == "" {
		panic("PackageContext.AddDocs requires RouterGroup")
	}
	p.Docs = append(p.Docs, doc)
	app.packageContexts[packagePath] = p
}

func compileDocManifests(routerGroup string, docs []DocManifest) ([]CompiledDocManifest, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	out := make([]CompiledDocManifest, 0, len(docs))
	seen := map[string]struct{}{}
	for i, doc := range docs {
		compiled, err := compileDocManifest(routerGroup, i, doc)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[compiled.Code]; exists {
			return nil, fmt.Errorf("%s docs code %q is duplicated", routerGroup, compiled.Code)
		}
		seen[compiled.Code] = struct{}{}
		out = append(out, compiled)
	}
	return out, nil
}

func compileDocManifest(routerGroup string, index int, doc DocManifest) (CompiledDocManifest, error) {
	code, err := normalizeDocManifestCode(doc.Code)
	if err != nil {
		return CompiledDocManifest{}, fmt.Errorf("%s docs #%d: %w", routerGroup, index+1, err)
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		return CompiledDocManifest{}, fmt.Errorf("%s docs %q content is required", routerGroup, code)
	}
	policy := strings.TrimSpace(doc.Policy)
	if policy == "" {
		policy = DocCreateIfMissing
	}
	if policy != DocCreateIfMissing {
		return CompiledDocManifest{}, fmt.Errorf("%s docs %q unsupported policy %q", routerGroup, code, policy)
	}
	name := strings.TrimSpace(doc.Name)
	if name == "" {
		name = strings.TrimSuffix(path.Base(code), ".docs")
	}
	format := strings.TrimSpace(doc.Format)
	if format == "" {
		format = "markdown"
	}
	return CompiledDocManifest{
		Code:        code,
		Name:        name,
		Description: strings.TrimSpace(doc.Description),
		Tags:        strings.TrimSpace(doc.Tags),
		Content:     doc.Content,
		Format:      format,
		Summary:     strings.TrimSpace(doc.Summary),
		Policy:      policy,
	}, nil
}

func normalizeDocManifestCode(rawCode string) (string, error) {
	code := strings.TrimSpace(rawCode)
	if code == "" {
		return "", fmt.Errorf("code is required")
	}
	if strings.HasPrefix(code, "/") {
		return "", fmt.Errorf("code %q must be relative to the package", rawCode)
	}
	if strings.Contains(code, `\`) {
		return "", fmt.Errorf("code %q must use forward slashes", rawCode)
	}

	code = strings.TrimPrefix(code, "./")
	parts := strings.Split(code, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("code %q contains an invalid path segment", rawCode)
		}
	}

	last := len(parts) - 1
	parts[last] = strings.TrimSuffix(parts[last], ".docs")
	if parts[last] == "" {
		return "", fmt.Errorf("code %q is missing a document name", rawCode)
	}
	parts[last] += ".docs"
	return strings.Join(parts, "/"), nil
}
