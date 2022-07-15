package schema

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/edsrzf/mmap-go"
	"github.com/natefinch/atomic"

	"github.com/blang/semver"
	"github.com/segmentio/encoding/json"

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

// ensurePlugin downloads and installs the specified plugin if it does not already exist.
func (l *pluginLoader) ensurePlugin(pkg string, version *semver.Version) error {
	// TODO: schema and provider versions
	// hack: Some of the hcl2 code isn't yet handling versions, so bail out if the version is nil to avoid failing
	// 		 the download. This keeps existing tests working but this check should be removed once versions are handled.
	if version == nil {
		return nil
	}

	pkgPlugin := workspace.PluginInfo{
		Kind:    workspace.ResourcePlugin,
		Name:    pkg,
		Version: version,
	}

	tryDownload := func(dst io.WriteCloser) error {
		defer dst.Close()
		tarball, expectedByteCount, err := pkgPlugin.Download()
		if err != nil {
			return err
		}
		defer tarball.Close()
		copiedByteCount, err := io.Copy(dst, tarball)
		if err != nil {
			return err
		}
		if copiedByteCount != expectedByteCount {
			return fmt.Errorf("Expected %d bytes but copied %d when downloading plugin %s",
				expectedByteCount, copiedByteCount, pkgPlugin)
		}
		return nil
	}

	tryDownloadToFile := func() (string, error) {
		file, err := ioutil.TempFile("" /* default temp dir */, "pulumi-plugin-tar")
		if err != nil {
			return "", err
		}
		err = tryDownload(file)
		if err != nil {
			err2 := os.Remove(file.Name())
			if err2 != nil {
				return "", fmt.Errorf("Error while removing tempfile: %v. Context: %w", err2, err)
			}
			return "", err
		}
		return file.Name(), nil
	}

	downloadToFileWithRetry := func() (string, error) {
		delay := 80 * time.Millisecond
		for attempt := 0; ; attempt++ {
			tempFile, err := tryDownloadToFile()
			if err == nil {
				return tempFile, nil
			}

			if err != nil && attempt >= 5 {
				return tempFile, err
			}
			time.Sleep(delay)
			delay = delay * 2
		}
	}

	if !workspace.HasPlugin(pkgPlugin) {
		tarball, err := downloadToFileWithRetry()
		if err != nil {
			return fmt.Errorf("failed to download plugin: %s: %w", pkgPlugin, err)
		}
		defer os.Remove(tarball)
		reader, err := os.Open(tarball)
		if err != nil {
			return fmt.Errorf("failed to open downloaded plugin: %s: %w", pkgPlugin, err)
		}
		if err := pkgPlugin.InstallWithContext(context.Background(), workspace.TarPlugin(reader), false); err != nil {
			return fmt.Errorf("failed to install plugin %s: %w", pkgPlugin, err)
		}
	}

	return nil
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
	return fmt.Sprintf("it looks like GetSchema is not implemented")
}

var schemaIsEmptyRE = regexp.MustCompile(`\s*\{\s*\}\s*$`)

func schemaIsEmpty(schemaBytes []byte) bool {
	// We assume that GetSchema isn't implemented it something of the form "{[\t\n ]*}" is
	// returned. That is what we did in the past when we chose not to implement GetSchema.
	return schemaIsEmptyRE.Match(schemaBytes)
}

func (l *pluginLoader) LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error) {
	l.m.Lock()
	defer l.m.Unlock()

	key := packageIdentity(pkg, version)
	if p, ok := l.getPackage(key); ok {
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

	// Insert a version into the spec if the package does not provide one
	if version != nil && spec.PackageInfoSpec.Version == "" {
		spec.PackageInfoSpec.Version = version.String()
	}

	p, err := importPartialSpec(spec, nil, l)
	if err != nil {
		return nil, err
	}

	return l.setPackage(key, p), nil
}

func LoadPackageReference(loader Loader, pkg string, version *semver.Version) (PackageReference, error) {
	if refLoader, ok := loader.(ReferenceLoader); ok {
		return refLoader.LoadPackageReference(pkg, version)
	}
	p, err := loader.LoadPackage(pkg, version)
	if err != nil {
		return nil, err
	}
	return p.Reference(), nil
}

func (l *pluginLoader) loadSchemaBytes(pkg string, version *semver.Version) ([]byte, *semver.Version, error) {
	if err := l.ensurePlugin(pkg, version); err != nil {
		return nil, nil, err
	}

	pluginInfo, err := l.host.ResolvePlugin(workspace.ResourcePlugin, pkg, nil)
	if err != nil {
		return nil, nil, err
	}

	if version == nil {
		version = pluginInfo.Version
	}

	if pluginInfo.SchemaPath != "" {
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
		info, err := provider.GetPluginInfo()
		if err != nil {
			// Nonfatal
		}
		version = info.Version
	}

	return schemaBytes, version, nil
}

func (l *pluginLoader) loadPluginSchemaBytes(pkg string, version *semver.Version) ([]byte, plugin.Provider, error) {
	provider, err := l.host.Provider(tokens.Package(pkg), version)
	if err != nil {
		return nil, nil, err
	}
	contract.Assert(provider != nil)

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
	schemaFile, err := os.OpenFile(path, os.O_RDONLY, 0644)
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
