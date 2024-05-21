//go:build js
// +build js

package schema

import (
	"time"
)

func (l *pluginLoader) loadCachedSchemaBytes(pkg string, path string, schemaTime time.Time) ([]byte, bool) {
	return nil, false
}
