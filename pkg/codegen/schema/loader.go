package schema

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/blang/semver"
	jsoniter "github.com/json-iterator/go"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Loader interface {
	LoadPackage(pkg string, version *semver.Version) (*Package, error)
}

type pluginLoader struct {
	m sync.RWMutex

	host    plugin.Host
	entries map[string]*Package
}

func NewPluginLoader(host plugin.Host) Loader {
	return &pluginLoader{
		host:    host,
		entries: map[string]*Package{},
	}
}

func (l *pluginLoader) getPackage(key string) (*Package, bool) {
	l.m.RLock()
	defer l.m.RUnlock()

	p, ok := l.entries[key]
	return p, ok
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
		if err := pkgPlugin.Install(reader); err != nil {
			return fmt.Errorf("failed to install plugin %s: %w", pkgPlugin, err)
		}
	}

	return nil
}

func (l *pluginLoader) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	key := pkg + "@"
	if version != nil {
		key += version.String()
	}

	if p, ok := l.getPackage(key); ok {
		return p, nil
	}

	if err := l.ensurePlugin(pkg, version); err != nil {
		return nil, err
	}

	provider, err := l.host.Provider(tokens.Package(pkg), version)
	if err != nil {
		return nil, err
	}

	schemaFormatVersion := 0
	schemaBytes, err := provider.GetSchema(schemaFormatVersion)
	if err != nil {
		return nil, err
	}

	var spec PackageSpec
	if err := jsoniter.Unmarshal(schemaBytes, &spec); err != nil {
		return nil, err
	}

	p, diags, err := bindSpec(spec, nil, l, false)
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}

	l.m.Lock()
	defer l.m.Unlock()

	if p, ok := l.entries[pkg]; ok {
		return p, nil
	}
	l.entries[key] = p

	return p, nil
}
