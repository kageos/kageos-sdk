package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/kageos/kageos-sdk/pkg/logger"
)

const (
	DocCreateIfMissing = "create_if_missing"
)

// DocManifest describes package-owned seed docs created during app update.
// It is declarative metadata only; Service Tree docs remain the runtime source of truth.
type DocManifest struct {
	Code        string `json:"code"`
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
	code := strings.Trim(strings.TrimSpace(doc.Code), "/")
	code = strings.TrimSuffix(code, ".docs")
	if code == "" {
		return CompiledDocManifest{}, fmt.Errorf("%s docs #%d code is required", routerGroup, index+1)
	}
	if strings.Contains(code, "/") {
		return CompiledDocManifest{}, fmt.Errorf("%s docs %q code must be a single path segment", routerGroup, code)
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
		name = code
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
