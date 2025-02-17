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
	"context"
	"errors"
	"fmt"

	"github.com/natefinch/atomic"

	"github.com/blang/semver"
	"github.com/segmentio/encoding/json"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type ParameterizationDescriptor struct {
	Name    string         // the name of the package.
	Version semver.Version // the version of the package.
	Value   []byte         // the parameter value of the package.
}

// PackageDescriptor is a descriptor for a package, this is similar to a plugin spec but also contains parameterization
// info.
type PackageDescriptor struct {
	Name             string                      // the simple name of the plugin.
	Version          *semver.Version             // the plugin's semantic version, if present.
	DownloadURL      string                      // an optional server to use when downloading this plugin.
	Parameterization *ParameterizationDescriptor // the optional parameterization of the package.
}

type Loader interface {
	// deprecated: use LoadPackageV2
	LoadPackage(pkg string, version *semver.Version) (*Package, error)

	LoadPackageV2(ctx context.Context, descriptor *PackageDescriptor) (*Package, error)
}

type ReferenceLoader interface {
	Loader

	// deprecated: use LoadPackageReferenceV2
	LoadPackageReference(pkg string, version *semver.Version) (PackageReference, error)

	LoadPackageReferenceV2(ctx context.Context, descriptor *PackageDescriptor) (PackageReference, error)
}

type pluginLoader struct {
	host plugin.Host

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
	return newPluginLoaderWithOptions(host, pluginLoaderCacheOptions{})
}

func newPluginLoaderWithOptions(host plugin.Host, cacheOptions pluginLoaderCacheOptions) ReferenceLoader {
	var l ReferenceLoader
	l = &pluginLoader{
		host: host,

		cacheOptions: cacheOptions,
	}
	if !cacheOptions.disableEntryCache {
		l = NewCachedLoader(l)
	}
	return l
}

func (l *pluginLoader) LoadPackage(pkg string, version *semver.Version) (*Package, error) {
	ref, err := l.LoadPackageReference(pkg, version)
	if err != nil {
		return nil, err
	}
	return ref.Definition()
}

func (l *pluginLoader) LoadPackageV2(ctx context.Context, descriptor *PackageDescriptor) (*Package, error) {
	ref, err := l.LoadPackageReferenceV2(ctx, descriptor)
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
	return l.LoadPackageReferenceV2(
		context.TODO(),
		&PackageDescriptor{
			Name:    pkg,
			Version: version,
		})
}

func (l *pluginLoader) LoadPackageReferenceV2(
	ctx context.Context, descriptor *PackageDescriptor,
) (PackageReference, error) {
	if descriptor.Name == "pulumi" {
		return DefaultPulumiPackage.Reference(), nil
	}

	schemaBytes, version, err := l.loadSchemaBytes(ctx, descriptor)
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
	return p, nil
}

// deprecated: use LoadPackageReferenceV2
func LoadPackageReference(loader Loader, pkg string, version *semver.Version) (PackageReference, error) {
	return LoadPackageReferenceV2(
		context.TODO(),
		loader,
		&PackageDescriptor{
			Name:    pkg,
			Version: version,
		})
}

// LoadPackageReferenceV2 loads a package reference for the given descriptor using the given loader. When a reference is
// loaded, the name and version of the reference are compared to the requested name and version. If the name or version
// do not match, a PackageReferenceNameMismatchError or PackageReferenceVersionMismatchError is returned, respectively.
//
// In the event that a mismatch error is returned, the reference is still returned. This is to allow for the caller to
// decide whether or not the mismatch impacts their use of the reference.
func LoadPackageReferenceV2(
	ctx context.Context, loader Loader, descriptor *PackageDescriptor,
) (PackageReference, error) {
	var ref PackageReference
	var err error
	if refLoader, ok := loader.(ReferenceLoader); ok {
		ref, err = refLoader.LoadPackageReferenceV2(ctx, descriptor)
	} else {
		p, pErr := loader.LoadPackageV2(ctx, descriptor)
		err = pErr
		if err == nil {
			ref = p.Reference()
		}
	}

	if err != nil {
		return nil, err
	}

	name := descriptor.Name
	if descriptor.Parameterization != nil {
		name = descriptor.Parameterization.Name
	}
	version := descriptor.Version
	if descriptor.Parameterization != nil {
		version = &descriptor.Parameterization.Version
	}

	if name != ref.Name() {
		return ref, &PackageReferenceNameMismatchError{
			RequestedName:    name,
			RequestedVersion: version,
			LoadedName:       ref.Name(),
			LoadedVersion:    ref.Version(),
		}
	}

	if version != nil && ref.Version() != nil && !ref.Version().Equals(*version) {
		err := &PackageReferenceVersionMismatchError{
			RequestedName:    name,
			RequestedVersion: version,
			LoadedName:       ref.Name(),
			LoadedVersion:    ref.Version(),
		}
		if l, ok := loader.(*cachedLoader); ok {
			err.Message = fmt.Sprintf("entries: %v", l.entries)
		}

		return ref, err
	}

	return ref, nil
}

// PackageReferenceNameMismatchError is the type of errors returned by LoadPackageReferenceV2 when the name of the
// loaded reference does not match the requested name.
type PackageReferenceNameMismatchError struct {
	// The requested . name
	RequestedName string
	// The requested version.
	RequestedVersion *semver.Version
	// The loaded name.
	LoadedName string
	// The loaded version.
	LoadedVersion *semver.Version
	// An optional message to be appended to the error's string representation.
	Message string
}

func (e *PackageReferenceNameMismatchError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf(
			"loader returned %s@%v; requested %s@%v",
			e.LoadedName, e.LoadedVersion,
			e.RequestedName, e.RequestedVersion,
		)
	}

	return fmt.Sprintf(
		"loader returned %s@%v; requested %s@%v (%s)",
		e.LoadedName, e.LoadedVersion,
		e.RequestedName, e.RequestedVersion,
		e.Message,
	)
}

// PackageReferenceVersionMismatchError is the type of errors returned by LoadPackageReferenceV2 when the version of the
// loaded reference does not match the requested version.
type PackageReferenceVersionMismatchError struct {
	// The requested name.
	RequestedName string
	// The requested version.
	RequestedVersion *semver.Version
	// The loaded name.
	LoadedName string
	// The loaded version.
	LoadedVersion *semver.Version
	// An optional message to be appended to the error's string representation.
	Message string
}

func (e *PackageReferenceVersionMismatchError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf(
			"loader returned %s@%v; requested %s@%v",
			e.LoadedName, e.LoadedVersion,
			e.RequestedName, e.RequestedVersion,
		)
	}

	return fmt.Sprintf(
		"loader returned %s@%v; requested %s@%v (%s)",
		e.LoadedName, e.LoadedVersion,
		e.RequestedName, e.RequestedVersion,
		e.Message,
	)
}

func (l *pluginLoader) loadSchemaBytes(
	ctx context.Context, descriptor *PackageDescriptor,
) ([]byte, *semver.Version, error) {
	attachPort, err := plugin.GetProviderAttachPort(tokens.Package(descriptor.Name))
	if err != nil {
		return nil, nil, err
	}
	version := descriptor.Version

	// If PULUMI_DEBUG_PROVIDERS requested an attach port, skip caching and workspace
	// interaction and load the schema directly from the given port.
	if attachPort != nil {
		schemaBytes, provider, err := l.loadPluginSchemaBytes(ctx, descriptor)
		if err != nil {
			return nil, nil, fmt.Errorf("Error loading schema from plugin: %w", err)
		}

		if version == nil {
			info, err := provider.GetPluginInfo(ctx)
			contract.IgnoreError(err) // nonfatal error
			version = info.Version
		}
		return schemaBytes, version, nil
	}

	pluginInfo, err := l.host.ResolvePlugin(apitype.ResourcePlugin, descriptor.Name, version)
	if err != nil {
		// Try and install the plugin if it was missing and try again, unless auto plugin installs are turned off.
		var missingError *workspace.MissingError
		if !errors.As(err, &missingError) || env.DisableAutomaticPluginAcquisition.Value() {
			return nil, nil, err
		}

		spec := workspace.PluginSpec{
			Kind:              apitype.ResourcePlugin,
			Name:              descriptor.Name,
			Version:           version,
			PluginDownloadURL: descriptor.DownloadURL,
		}

		log := func(sev diag.Severity, msg string) {
			l.host.Log(sev, "", msg, 0)
		}

		_, err = pkgWorkspace.InstallPlugin(ctx, spec, log)
		if err != nil {
			return nil, nil, err
		}

		pluginInfo, err = l.host.ResolvePlugin(apitype.ResourcePlugin, descriptor.Name, version)
		if err != nil {
			return nil, version, err
		}
	}
	contract.Assertf(pluginInfo != nil, "loading pkg %q: pluginInfo was unexpectedly nil", descriptor.Name)

	if version == nil {
		version = pluginInfo.Version
	}

	canCache := pluginInfo.SchemaPath != "" && version != nil && descriptor.Parameterization == nil

	if canCache {
		schemaBytes, ok := l.loadCachedSchemaBytes(descriptor.Name, pluginInfo.SchemaPath, pluginInfo.SchemaTime)
		if ok {
			return schemaBytes, nil, nil
		}
	}

	schemaBytes, provider, err := l.loadPluginSchemaBytes(ctx, descriptor)
	if err != nil {
		return nil, nil, fmt.Errorf("Error loading schema from plugin: %w", err)
	}

	if canCache {
		err = atomic.WriteFile(pluginInfo.SchemaPath, bytes.NewReader(schemaBytes))
		if err != nil {
			return nil, nil, fmt.Errorf("Error writing schema from plugin to cache: %w", err)
		}
	}

	if version == nil {
		info, _ := provider.GetPluginInfo(ctx) // nonfatal error
		version = info.Version
	}

	return schemaBytes, version, nil
}

func (l *pluginLoader) loadPluginSchemaBytes(
	ctx context.Context, descriptor *PackageDescriptor,
) ([]byte, plugin.Provider, error) {
	wsDescriptor := workspace.PackageDescriptor{
		PluginSpec: workspace.PluginSpec{
			Name:              descriptor.Name,
			Version:           descriptor.Version,
			PluginDownloadURL: descriptor.DownloadURL,
		},
	}
	if descriptor.Parameterization != nil {
		wsDescriptor.Parameterization = &workspace.Parameterization{
			Name:    descriptor.Parameterization.Name,
			Version: descriptor.Parameterization.Version,
			Value:   descriptor.Parameterization.Value,
		}
	}

	provider, err := l.host.Provider(wsDescriptor)
	if err != nil {
		return nil, nil, err
	}
	contract.Assertf(provider != nil, "unexpected nil provider for %s@%v", descriptor.Name, descriptor.Version)

	var schemaFormatVersion int32
	getSchemaRequest := plugin.GetSchemaRequest{
		Version: schemaFormatVersion,
	}

	// If this is a parameterized package, we need to pass the parameter value to the provider.
	if descriptor.Parameterization != nil {
		parameterization := plugin.ParameterizeRequest{
			Parameters: &plugin.ParameterizeValue{
				Name:    descriptor.Parameterization.Name,
				Version: descriptor.Parameterization.Version,
				Value:   descriptor.Parameterization.Value,
			},
		}
		resp, err := provider.Parameterize(ctx, parameterization)
		if err != nil {
			return nil, nil, err
		}
		if resp.Name != descriptor.Parameterization.Name {
			return nil, nil, fmt.Errorf(
				"unexpected parameterization response: %s != %s", resp.Name, descriptor.Parameterization.Name)
		}
		if !resp.Version.EQ(descriptor.Parameterization.Version) {
			return nil, nil, fmt.Errorf(
				"unexpected parameterization response: %s != %s", resp.Version, descriptor.Parameterization.Version)
		}

		getSchemaRequest.SubpackageName = resp.Name
		getSchemaRequest.SubpackageVersion = &resp.Version
	}

	schema, err := provider.GetSchema(ctx, getSchemaRequest)
	if err != nil {
		return nil, nil, err
	}

	return schema.Schema, provider, nil
}
