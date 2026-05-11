// Copyright 2016, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !js

package schema

import (
	"io"
	"os"
	"time"

	"github.com/edsrzf/mmap-go"
)

var mmapedFiles = make(map[string]mmap.MMap)

// loadCachedSchemaBytes returns the cached schema at path if the cache file is newer than
// pluginInstallTime (i.e. the schema was written after the plugin was last installed).
func (l *pluginLoader) loadCachedSchemaBytes(path string, pluginInstallTime time.Time) ([]byte, bool) {
	if l.cacheOptions.disableFileCache {
		return nil, false
	}

	if schemaMmap, ok := mmapedFiles[path]; ok {
		return schemaMmap, true
	}

	success := false
	schemaFile, err := os.OpenFile(path, os.O_RDONLY, 0o644)
	defer func() {
		// Close on failure, or on a plain read where the data has already been
		// copied. For mmap, the file must remain open while the mapping is active
		// (required on Windows), so we leave it open in that case.
		if !success || l.cacheOptions.disableMmap {
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

	if pluginInstallTime.After(stat.ModTime()) {
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
