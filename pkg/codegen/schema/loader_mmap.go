//go:build !js
// +build !js

package schema

import (
	"io"
	"os"
	"time"

	"github.com/edsrzf/mmap-go"
)

var mmapedFiles = make(map[string]mmap.MMap)

func (l *pluginLoader) loadCachedSchemaBytes(pkg string, path string, schemaTime time.Time) ([]byte, bool) {
	if l.cacheOptions.disableFileCache {
		return nil, false
	}

	if schemaMmap, ok := mmapedFiles[path]; ok {
		return schemaMmap, true
	}

	success := false
	schemaFile, err := os.OpenFile(path, os.O_RDONLY, 0o644)
	defer func() {
		if !success {
			schemaFile.Close()
		}
	}()
	if err != nil {
		return nil, success
	}

	stat, err := schemaFile.Stat()
	if err != nil {
		return nil, success
	}
	cachedAt := stat.ModTime()

	if schemaTime.After(cachedAt) {
		return nil, success
	}

	if l.cacheOptions.disableMmap {
		data, err := io.ReadAll(schemaFile)
		if err != nil {
			return nil, success
		}
		success = true
		return data, success
	}

	schemaMmap, err := mmap.Map(schemaFile, mmap.RDONLY, 0)
	if err != nil {
		return nil, success
	}
	success = true

	return schemaMmap, success
}
