//go:build tools

package tools

import (
	_ "golang.org/x/tools/go/analysis/passes/nilness"
	_ "honnef.co/go/tools/staticcheck"
)
