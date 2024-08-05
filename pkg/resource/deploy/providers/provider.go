// Copyright 2016-2021, Pulumi Corporation.
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

package providers

import (
	"fmt"
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type ProviderParameterization struct {
	// The name of the parametrized package.
	name tokens.Package
	// The version of the parametrized package.
	version semver.Version
	// The value of the parameter.
	value []byte
}

// NewProviderParameterization constructs a new provider parameterization.
func NewProviderParameterization(name tokens.Package, version semver.Version, value []byte,
) *ProviderParameterization {
	return &ProviderParameterization{
		name:    name,
		version: version,
		value:   value,
	}
}

// A ProviderRequest is a tuple of an optional semantic version, download server url, parameter, and a package name.
// Whenever the engine receives a registration for a resource that doesn't explicitly specify a provider, the engine
// creates a ProviderRequest for that resource's provider, using the version passed to the engine as part of
// RegisterResource and the package derived from the resource's token.
//
// The source evaluator (source_eval.go) is responsible for servicing provider requests. It does this by interpreting
// these provider requests and sending resource registrations to the engine for the providers themselves. These are
// called "default providers".
//
// ProviderRequest is useful as a hash key. The engine is free to instantiate any number of provider requests, but it is
// free to cache requests for a provider request that is equal to one that has already been serviced. If you do use
// ProviderRequest as a hash key, you should call String() to get a usable key for string-based hash maps.
// ProviderRequests only hash by their package name, version and download URL. The checksums and parameterization are
// not used in the hash.
type ProviderRequest struct {
	version           *semver.Version
	name              tokens.Package
	pluginDownloadURL string
	pluginChecksums   map[string][]byte
	parameterization  *ProviderParameterization
}

// NewProviderRequest constructs a new provider request from an optional version, optional
// pluginDownloadURL, optional parameter, and package.
func NewProviderRequest(
	name tokens.Package, version *semver.Version,
	pluginDownloadURL string, checksums map[string][]byte,
	parameterization *ProviderParameterization,
) ProviderRequest {
	return ProviderRequest{
		version:           version,
		name:              name,
		pluginDownloadURL: strings.TrimSuffix(pluginDownloadURL, "/"),
		pluginChecksums:   checksums,
		parameterization:  parameterization,
	}
}

// Parameterization returns the parameterization of this provider request. May be nil if no parameterization was
// provided.
func (p ProviderRequest) Parameterization() *ProviderParameterization {
	return p.parameterization
}

// Name returns the this provider plugin name.
func (p ProviderRequest) Name() tokens.Package {
	return p.name
}

// Version returns this provider request's version. May be nil if no version was provided.
func (p ProviderRequest) Version() *semver.Version {
	return p.version
}

// Package returns this provider request's package.
func (p ProviderRequest) Package() tokens.Package {
	if p.parameterization != nil {
		return p.parameterization.name
	}
	return p.name
}

// PluginDownloadURL returns this providers server url. May be "" if no pluginDownloadURL was
// provided.
func (p ProviderRequest) PluginDownloadURL() string {
	return p.pluginDownloadURL
}

// PluginChecksums returns this providers checksums. May be nil if no checksums were provided.
func (p ProviderRequest) PluginChecksums() map[string][]byte {
	return p.pluginChecksums
}

// DefaultName returns a QName that is an appropriate name for a default provider constructed from this provider
// request. The name is intended to be unique; as such, the name is derived from the version associated with this
// request.
//
// If a version is not provided, "default" is returned. Otherwise, Name returns a name starting with "default" and
// followed by a QName-legal representation of the semantic version of the requested provider.
func (p ProviderRequest) DefaultName() string {
	base := "default"

	var v *semver.Version
	if p.parameterization != nil {
		v = &p.parameterization.version
	} else {
		v = p.version
	}

	if v != nil {
		// QNames are forbidden to contain dashes, so we construct a string here using the semantic
		// version's component parts.
		base += fmt.Sprintf("_%d_%d_%d", v.Major, v.Minor, v.Patch)
		for _, pre := range v.Pre {
			base += "_" + pre.String()
		}
		for _, build := range v.Build {
			base += "_" + build
		}
	}

	if url := p.pluginDownloadURL; url != "" {
		base += "_" + tokens.IntoQName(url).String()
	}

	// This thing that we generated must be a QName, the engine doesn't actually care but it probably helps
	// down the line if we keep these names simple.
	contract.Assertf(tokens.IsQName(base), "generated provider name %q is not a QName", base)
	return base
}

// String returns a string representation of this request. This string is suitable for use as a hash key.
func (p ProviderRequest) String() string {
	var version string
	if p.parameterization != nil {
		version = "-" + p.parameterization.version.String()
	} else if p.version != nil {
		version = "-" + p.version.String()
	}
	var url string
	if p.pluginDownloadURL != "" {
		url = "-" + p.pluginDownloadURL
	}
	return p.Package().String() + version + url
}
