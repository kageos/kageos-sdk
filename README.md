# Kageos SDK

Kageos SDK is the public Go module used by Kageos workspace apps.

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

Kageos workspace apps should pin a SDK version in their own `go.mod`.
