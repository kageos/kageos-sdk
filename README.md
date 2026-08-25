# kageos SDK

kageos SDK is the public Go module used by kageos workspace apps.

It contains the app runtime APIs, widget schema helpers, response builders,
callback helpers, lightweight DTOs, and public utility packages that workspace
code imports at build time.

## Module

```go
module github.com/kageos/kageos-sdk
```

Common imports:

```go
import (
	"github.com/kageos/kageos-sdk/agent-app/app"
	"github.com/kageos/kageos-sdk/agent-app/callback"
	"github.com/kageos/kageos-sdk/agent-app/response"
	"github.com/kageos/kageos-sdk/agent-app/types"
	"github.com/kageos/kageos-sdk/pkg/gormx/query"
	"github.com/kageos/kageos-sdk/pkg/logger"
)
```

## Workspace App Example

```go
package main

import "github.com/kageos/kageos-sdk/agent-app/app"

func main() {
	if err := app.Run(); err != nil {
		panic(err)
	}
}
```

## Guides

- [Chart time bucket policy](agent-app/CHART_BUCKET_POLICY.md): choose chart
  aggregation granularity, estimate returned points, and optionally coarsen
  oversized chart responses.

### Rich OnSelectFuzzy items

Dynamic select and multiselect callbacks may attach read-only rich text and
file refs to a candidate. `Value` remains the stable value submitted by the
form; `DisplayInfo` remains short structured data for display and statistics.

```go
item := &callback.SelectFuzzyItem{
	Value:    topic.ID,
	Label:    topic.Title,
	RichText: topic.Content,
	DisplayInfo: map[string]interface{}{
		"status": topic.Status,
	},
}

optionItem := &callback.SelectFuzzyItem{
	Value: option.ID,
	Label: option.Content,
	Files: option.Image,
}
```

`Files` uses the same comma-separated file-ref protocol as the `files` widget.
Do not put rich text, file refs, or composite UI payloads in `Value`.

## Local Development

Run the SDK test suite:

```bash
go test ./...
```

Use a local SDK checkout from a workspace app:

```go
replace github.com/kageos/kageos-sdk => /path/to/kageos-sdk
```

## Versioning

Publish SDK releases with semantic tags, for example:

```bash
git tag v0.1.0
git push origin main --tags
```

kageos workspace apps should pin a SDK version in their own `go.mod`.

## License

kageos SDK is licensed under the [Apache License 2.0](LICENSE). You may use it
in open-source and proprietary applications, including commercial kageos
directories, apps, plugins, and integrations, subject to the license terms.
