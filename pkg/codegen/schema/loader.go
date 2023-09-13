// Copyright 2016-2022, Pulumi Corporation.
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

package schema

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/edsrzf/mmap-go"
	"github.com/natefinch/atomic"

	"github.com/blang/semver"
	"github.com/segmentio/encoding/json"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Loader interface {
	LoadPackage(pkg string, version *semver.Version) (*Package, error)
}

type ReferenceLoader interface {
	Loader

	LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error)
}

type pluginLoader struct {
	m sync.RWMutex

	host    plugin.Host
	entries map[string]PackageReference

	cacheOptions pluginLoaderCacheOptions
}

// Caching options intended for benchmarking or debugging:
type pluginLoaderCacheOptions struct {
	// useEntriesCache enables in-memory re-use of packages
	disableEntryCache bool
	// useFileCache enables skipping plugin loading when possible and caching JSON schemas to files
	disableFileCache bool
	// useMmap enables the use of memory mapped IO to avoid copying the JSON schema
	disableMmap bool
}

func NewPluginLoader(host plugin.Host) ReferenceLoader {
	return &pluginLoader{
		host:    host,
		entries: map[string]PackageReference{},
	}
}

func newPluginLoaderWithOptions(host plugin.Host, cacheOptions pluginLoaderCacheOptions) ReferenceLoader {
	return &pluginLoader{
		host:    host,
		entries: map[string]PackageReference{},

		cacheOptions: cacheOptions,
	}
}

func (l *pluginLoader) getPackage(key string) (PackageReference, bool) {
	if l.cacheOptions.disableEntryCache {
		return nil, false
	}
	p, ok := l.entries[key]
	return p, ok
}

func (l *pluginLoader) setPackage(key string, p PackageReference) PackageReference {
	if l.cacheOptions.disableEntryCache {
		return p
	}

	if p, ok := l.entries[key]; ok {
		return p
	}

	l.entries[key] = p
	return p
}

func (l *pluginLoader) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

var ErrGetSchemaNotImplemented = getSchemaNotImplemented{}

type getSchemaNotImplemented struct{}

func (f getSchemaNotImplemented) Error() string {
	return "it looks like GetSchema is not implemented"
}

func schemaIsEmpty(schemaBytes []byte) bool {
	// A non-empty schema is any that contains non-whitespace, non brace characters.
	//
	// Some providers implemented GetSchema initially by returning text matching the regular
	// expression: "\s*\{\s*\}\s*". This handles those cases while not strictly checking that braces
	// match or reading the whole document.
	for _, v := range schemaBytes {
		if v != ' ' && v != '\t' && v != '\r' && v != '\n' && v != '{' && v != '}' {
			return false
		}
	}

	return true
}

func (l *pluginLoader) LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error) {
	if pkg == "pulumi" {
		return DefaultPulumiPackage.Reference(), nil
	}

	l.m.Lock()
	defer l.m.Unlock()

	key := packageIdentity(pkg, version)
	if p, ok := l.getPackage(key); ok && version == nil {
		return p, nil
	}

	schemaBytes, version, err := l.loadSchemaBytes(pkg, version)
	if err != nil {
		return nil, err
	}
	if schemaIsEmpty(schemaBytes) {
		return nil, getSchemaNotImplemented{}
	}

	var spec PartialPackageSpec
	if _, err := json.Parse(schemaBytes, &spec, json.ZeroCopy); err != nil {
		return nil, err
	}

	// Insert a version into the spec if the package does not provide one or if the
	// existing version is less than the provided one
	if version != nil {
		setVersion := true
		if spec.PackageInfoSpec.Version != "" {
			vSemver, err := semver.Make(spec.PackageInfoSpec.Version)
			if err == nil {
				if vSemver.Compare(*version) == 1 {
					setVersion = false
				}
			}
		}
		if setVersion {
			spec.PackageInfoSpec.Version = version.String()
		}
	}

	p, err := ImportPartialSpec(spec, nil, l)
	if err != nil {
		return nil, err
	}
	return l.setPackage(key, p), nil
}

func LoadPackageReference(loader Loader, pkg string, version *semver.Version) (PackageReference, error) {
	var ref PackageReference
	var err error
	if refLoader, ok := loader.(ReferenceLoader); ok {
		ref, err = refLoader.LoadPackageReference(pkg, version)
	} else {
		p, pErr := loader.LoadPackage(pkg, version)
		err = pErr
		if err == nil {
			ref = p.Reference()
		}
	}

	if err != nil {
		return nil, err
	}

	if pkg != ref.Name() || version != nil && ref.Version() != nil && !ref.Version().Equals(*version) {
		if l, ok := loader.(*pluginLoader); ok {
			return nil, fmt.Errorf("req: %s@%v: entries: %v (returned %s@%v)", pkg, version,
				l.entries, ref.Name(), ref.Version())
		}
		return nil, fmt.Errorf("loader returned %s@%v: expected %s@%v", ref.Name(), ref.Version(), pkg, version)
	}

	return ref, nil
}

func (l *pluginLoader) loadSchemaBytes(pkg string, version *semver.Version) ([]byte, *semver.Version, error) {
	pluginInfo, err := l.host.ResolvePlugin(workspace.ResourcePlugin, pkg, version)
	if err != nil {
		var missingError *workspace.MissingError
		if errors.As(err, &missingError) {
			spec := workspace.PluginSpec{
				Kind:    workspace.ResourcePlugin,
				Name:    pkg,
				Version: version,
			}

			log := func(sev diag.Severity, msg string) {
				l.host.Log(sev, "", msg, 0)
			}

			_, err = pkgWorkspace.InstallPlugin(spec, log)
			if err != nil {
				return nil, nil, err
			}

			pluginInfo, err = l.host.ResolvePlugin(workspace.ResourcePlugin, pkg, version)
			if err != nil {
				return nil, version, err
			}
		} else {
			return nil, nil, err
		}
	}
	contract.Assertf(pluginInfo != nil, "loading pkg %q: pluginInfo was unexpectedly nil", pkg)

	if version == nil {
		version = pluginInfo.Version
	}

	if pluginInfo.SchemaPath != "" && version != nil {
		schemaBytes, ok := l.loadCachedSchemaBytes(pkg, pluginInfo.SchemaPath, pluginInfo.SchemaTime)
		if ok {
			return schemaBytes, nil, nil
		}
	}

	schemaBytes, provider, err := l.loadPluginSchemaBytes(pkg, version)
	if err != nil {
		return nil, nil, fmt.Errorf("Error loading schema from plugin: %w", err)
	}

	if pluginInfo.SchemaPath != "" {
		err = atomic.WriteFile(pluginInfo.SchemaPath, bytes.NewReader(schemaBytes))
		if err != nil {
			return nil, nil, fmt.Errorf("Error writing schema from plugin to cache: %w", err)
		}
	}

	if version == nil {
		info, _ := provider.GetPluginInfo() // nonfatal error
		version = info.Version
	}

	return schemaBytes, version, nil
}

func (l *pluginLoader) loadPluginSchemaBytes(pkg string, version *semver.Version) ([]byte, plugin.Provider, error) {
	provider, err := l.host.Provider(tokens.Package(pkg), version)
	if err != nil {
		return nil, nil, err
	}
	contract.Assertf(provider != nil, "unexpected nil provider for %s@%v", pkg, version)

	schemaFormatVersion := 0
	schemaBytes, err := provider.GetSchema(schemaFormatVersion)
	if err != nil {
		return nil, nil, err
	}

	return schemaBytes, provider, nil
}

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
