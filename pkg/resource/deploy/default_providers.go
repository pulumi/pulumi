// Copyright 2026, Pulumi Corporation.
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

package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// defaultProviders manages the registration of default providers. The default provider for a package is the provider
// resource that will be used to manage resources that do not explicitly reference a provider. Default providers will
// only be registered for packages that are used by resources registered by the user's Pulumi program.
type defaultProviders struct {
	// A map of package identifiers to versions, used to disambiguate which plugin to load if no version is provided
	// by the language host.
	defaultProviderInfo map[tokens.Package]workspace.PackageDescriptor

	// A map of ProviderRequest strings to provider references, used to keep track of the set of default providers that
	// have already been loaded.
	providers map[string]sdkproviders.Reference
	config    plugin.ConfigSource

	requests        chan defaultProviderRequest
	providerRegChan chan<- *registerResourceEvent
	cancel          <-chan bool
}

type defaultProviderResponse struct {
	ref sdkproviders.Reference
	err error
}

type defaultProviderRequest struct {
	req      providers.ProviderRequest
	response chan<- defaultProviderResponse
}

func (d *defaultProviders) normalizeProviderRequest(req providers.ProviderRequest) providers.ProviderRequest {
	// Request that the engine instantiate a specific version of this provider, if one was requested. We'll figure out
	// what version to request by:
	//   1. Providing the Version field of the ProviderRequest verbatim, if it was provided, otherwise
	//   2. Querying the list of default versions provided to us on startup and returning the value associated with
	//      the given package, if one exists, otherwise
	//   3. We give nothing to the engine and let the engine figure it out.
	//
	// As we tighen up our approach to provider versioning, 2 and 3 will go away and be replaced entirely by 1. 3 is
	// especially onerous because the engine selects the "newest" plugin available on the machine, which is generally
	// problematic for a lot of reasons.
	if req.Version() != nil {
		logging.V(5).Infof("normalizeProviderRequest(%s): using version %s from request", req, req.Version())
	} else {
		if version := d.defaultProviderInfo[req.Package()].Version; version != nil {
			logging.V(5).Infof("normalizeProviderRequest(%s): default version hit on version %s", req, version)
			req = providers.NewProviderRequest(
				req.Package(), version, req.PluginDownloadURL(), req.PluginChecksums(), req.Parameterization())
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default provider miss, sending nil version to engine", req)
		}
	}

	if req.PluginDownloadURL() != "" {
		logging.V(5).Infof("normalizeProviderRequest(%s): using pluginDownloadURL %s from request",
			req, req.PluginDownloadURL())
	} else {
		if pluginDownloadURL := d.defaultProviderInfo[req.Package()].PluginDownloadURL; pluginDownloadURL != "" {
			logging.V(5).Infof("normalizeProviderRequest(%s): default pluginDownloadURL hit on %s",
				req, pluginDownloadURL)
			req = providers.NewProviderRequest(
				req.Package(), req.Version(), pluginDownloadURL, req.PluginChecksums(), req.Parameterization())
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default pluginDownloadURL miss, sending empty string to engine", req)
		}
	}

	if req.PluginChecksums() != nil {
		logging.V(5).Infof("normalizeProviderRequest(%s): using pluginChecksums %v from request",
			req, req.PluginChecksums())
	} else {
		if pluginChecksums := d.defaultProviderInfo[req.Package()].Checksums; pluginChecksums != nil {
			logging.V(5).Infof("normalizeProviderRequest(%s): default pluginChecksums hit on %v",
				req, pluginChecksums)
			req = providers.NewProviderRequest(
				req.Package(), req.Version(), req.PluginDownloadURL(), pluginChecksums, req.Parameterization())
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default pluginChecksums miss, sending empty map to engine", req)
		}
	}

	if req.Parameterization() != nil {
		logging.V(5).Infof("normalizeProviderRequest(%s): using parameterization %v from request",
			req, req.Parameterization())
	} else {
		if parameterization := d.defaultProviderInfo[req.Package()].Parameterization; parameterization != nil {
			logging.V(5).Infof("normalizeProviderRequest(%s): default parameterization hit on %v",
				req, parameterization)

			req = providers.NewProviderRequest(
				req.Package(), req.Version(), req.PluginDownloadURL(), req.PluginChecksums(), parameterization)
		} else {
			logging.V(5).Infof(
				"normalizeProviderRequest(%s): default parameterization miss, sending nil to engine", req)
		}
	}

	return req
}

// newRegisterDefaultProviderEvent creates a RegisterResourceEvent and completion channel that can be sent to the
// engine to register a default provider resource for the indicated package.
func (d *defaultProviders) newRegisterDefaultProviderEvent(
	req providers.ProviderRequest,
) (*registerResourceEvent, <-chan *RegisterResult, error) {
	// Attempt to get the config for the package.
	minputs, err := d.config.GetPackageConfig(req.Package())
	if err != nil {
		return nil, nil, err
	}
	inputs := resource.ToResourcePropertyMap(minputs)
	if req.Version() != nil {
		providers.SetProviderVersion(inputs, req.Version())
	}
	if req.PluginDownloadURL() != "" {
		providers.SetProviderURL(inputs, req.PluginDownloadURL())
	}
	if req.PluginChecksums() != nil {
		providers.SetProviderChecksums(inputs, req.PluginChecksums())
	}
	if req.Parameterization() != nil {
		providers.SetProviderName(inputs, req.Name())
		providers.SetProviderParameterization(inputs, req.Parameterization())
	}

	// Create the result channel and the event.
	done := make(chan *RegisterResult)
	event := &registerResourceEvent{
		goal: pkgresource.NewGoal{
			Type:                    sdkproviders.MakeProviderType(req.Package()),
			Name:                    req.DefaultName(),
			Custom:                  true,
			Properties:              resource.FromResourcePropertyMap(inputs),
			Parent:                  "",
			Protect:                 nil,
			Dependencies:            nil,
			Provider:                "",
			InitErrors:              nil,
			PropertyDependencies:    nil,
			DeleteBeforeReplace:     nil,
			IgnoreChanges:           nil,
			AdditionalSecretOutputs: nil,
			Aliases:                 nil,
			ID:                      "",
			CustomTimeouts:          nil,
			ReplaceOnChanges:        nil,
			ReplacementTrigger:      property.New(property.Null),
			RetainOnDelete:          nil,
			HideDiff:                nil,
			DeletedWith:             "",
			ReplaceWith:             nil,
			SourcePosition:          "",
			StackTrace:              nil,
			ResourceHooks:           nil,
		}.Make(),
		done: done,
	}
	return event, done, nil
}

// handleRequest services a single default provider request. If the request is for a default provider that we have
// already loaded, we will return its reference. If the request is for a default provider that has not yet been
// loaded, we will send a register resource request to the engine, wait for it to complete, and then cache and return
// the reference of the loaded provider.
//
// Note that this function must not be called from two goroutines concurrently; it is the responsibility of d.serve()
// to ensure this.
func (d *defaultProviders) handleRequest(req providers.ProviderRequest) (sdkproviders.Reference, error) {
	logging.V(5).Infof("handling default provider request for package %s", req)

	req = d.normalizeProviderRequest(req)

	denyCreation, err := d.shouldDenyRequest(req)
	if err != nil {
		return sdkproviders.Reference{}, err
	}
	if denyCreation {
		logging.V(5).Infof("denied default provider request for package %s", req)
		return sdkproviders.NewDenyDefaultProvider(string(req.Package().Name())), nil
	}

	// Have we loaded this provider before? Use the existing reference, if so.
	//
	// Note that we are using the request's String as the key for the provider map. Go auto-derives hash and equality
	// functions for aggregates, but the one auto-derived for ProviderRequest does not have the semantics we want. The
	// use of a string key here is hacky but gets us the desired semantics - that ProviderRequest is a tuple of
	// optional value-typed Version and a package.
	ref, ok := d.providers[req.String()]
	if ok {
		return ref, nil
	}

	event, done, err := d.newRegisterDefaultProviderEvent(req)
	if err != nil {
		return sdkproviders.Reference{}, err
	}

	select {
	case d.providerRegChan <- event:
	case <-d.cancel:
		return sdkproviders.Reference{}, context.Canceled
	}

	logging.V(5).Infof("waiting for default provider for package %s", req)

	var result *RegisterResult
	select {
	case result = <-done:
	case <-d.cancel:
		return sdkproviders.Reference{}, context.Canceled
	}

	logging.V(5).Infof("registered default provider for package %s: %s", req, result.State.URN)

	id := result.State.ID
	contract.Assertf(id != "", "default provider for package %s has no ID", req)

	ref, err = sdkproviders.NewReference(result.State.URN, id)
	contract.Assertf(err == nil, "could not create provider reference with URN %s and ID %s", result.State.URN, id)
	d.providers[req.String()] = ref

	return ref, nil
}

// If req should be allowed, or if we should prevent the request.
func (d *defaultProviders) shouldDenyRequest(req providers.ProviderRequest) (bool, error) {
	logging.V(9).Infof("checking if %#v should be denied", req)

	if req.Package().Name().String() == "pulumi" {
		logging.V(9).Infof("we always allow %#v through", req)
		return false, nil
	}

	pConfig, err := d.config.GetPackageConfig("pulumi")
	if err != nil {
		return true, err
	}

	denyCreation := false
	if value, ok := pConfig.GetOk("disable-default-providers"); ok {
		array := []any{}
		if !value.IsString() {
			return true, errors.New("Unexpected encoding of pulumi:disable-default-providers")
		}
		if value.AsString() == "" {
			// If the list is provided but empty, we don't encode a empty json
			// list, we just encode the empty string. Check to ensure we don't
			// get parse errors.
			return false, nil
		}
		if err := json.Unmarshal([]byte(value.AsString()), &array); err != nil {
			return true, fmt.Errorf("Failed to parse %s: %w", value.AsString(), err)
		}
		for i, v := range array {
			s, ok := v.(string)
			if !ok {
				return true, fmt.Errorf("pulumi:disable-default-providers[%d] must be a string", i)
			}
			barred := strings.TrimSpace(s)
			if barred == "*" || barred == req.Package().Name().String() {
				logging.V(7).Infof("denying %s (star=%t)", req, barred == "*")
				denyCreation = true
				break
			}
		}
	} else {
		logging.V(9).Infof("Did not find a config for 'pulumi'")
	}

	return denyCreation, nil
}

// serve is the primary loop responsible for handling default provider requests.
func (d *defaultProviders) serve() {
	for {
		select {
		case req := <-d.requests:
			// Note that we do not need to handle cancellation when sending the response: every message we receive is
			// guaranteed to have something waiting on the other end of the response channel.
			ref, err := d.handleRequest(req.req)
			req.response <- defaultProviderResponse{ref: ref, err: err}
		case <-d.cancel:
			return
		}
	}
}

// getDefaultProviderRef fetches the provider reference for the default provider for a particular package.
func (d *defaultProviders) getDefaultProviderRef(req providers.ProviderRequest) (sdkproviders.Reference, error) {
	response := make(chan defaultProviderResponse)
	select {
	case d.requests <- defaultProviderRequest{req: req, response: response}:
	case <-d.cancel:
		return sdkproviders.Reference{}, context.Canceled
	}
	res := <-response
	return res.ref, res.err
}
