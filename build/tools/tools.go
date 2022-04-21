//go:build generate

//go:generate sh -c "go list -f '{{.ImportPath}}@{{.Module.Version}}' $(sed -n 's/.*_ \"\\(.*\\)\".*/\\1/p' <$GOFILE) | GOBIN=$PWD/../../.gobincache xargs -n 1 go install"

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/mattn/goveralls"
)
