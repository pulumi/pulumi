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

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	envutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// Package is the sandbox mode rule for a single provider package.
type Package struct {
	mode     *Mode
	sink     diag.Sink
	env      envutil.MapStore
	overlay  map[string]property.Value
	defaults map[string]property.Value
	remove   []string

	// emulator is the emulator the package's resources are created in, and services maps resource modules to the
	// services it must offer for them.
	emulator Emulator
	services map[string]string
}

// Env returns the environment variables to set on the package's plugin process. They take precedence over the
// ambient environment.
func (p *Package) Env() envutil.Store {
	return p.env
}

// Wrap returns a provider that applies the package's configuration overlay when its configuration is checked and when
// it is configured, and that reports resources the emulator doesn't support. The overlay never reaches the provider
// inputs recorded in state: CheckConfig's result has the overlaid keys restored to the values the program declared.
func (p *Package) Wrap(provider plugin.Provider) plugin.Provider {
	if len(p.overlay) == 0 && len(p.defaults) == 0 && len(p.remove) == 0 && len(p.services) == 0 {
		return provider
	}
	return &sandboxProvider{Provider: provider, pkg: p}
}

// ApplyConfig applies the package's configuration overlay to a provider's configuration inputs.
func (p *Package) ApplyConfig(inputs property.Map) property.Map {
	inputs = inputs.Delete(p.remove...)
	for k, v := range p.defaults {
		if existing, ok := inputs.GetOk(k); !ok || existing.IsNull() {
			inputs = inputs.Set(k, v)
		}
	}
	for k, v := range p.overlay {
		inputs = inputs.Set(k, v)
	}
	return inputs
}

// revertConfig undoes ApplyConfig on checked properties, restoring every key the overlay touched to its value in the
// original inputs.
func (p *Package) revertConfig(checked, original property.Map) property.Map {
	keys := slices.Concat(slices.Collect(maps.Keys(p.overlay)), slices.Collect(maps.Keys(p.defaults)), p.remove)
	for _, k := range keys {
		if v, ok := original.GetOk(k); ok {
			checked = checked.Set(k, v)
		} else {
			checked = checked.Delete(k)
		}
	}
	return checked
}

// missingService returns the service a resource needs that the emulator doesn't offer, if any. Resources whose
// module has no known service, and emulators whose services aren't known, are assumed to be supported.
func (p *Package) missingService(urn resource.URN) (string, bool) {
	if p.emulator.Services == nil {
		return "", false
	}
	service, ok := p.services[resourceModule(urn)]
	if !ok || p.emulator.Services[service] {
		return "", false
	}
	return service, true
}

// resourceModule returns the first segment of a resource type's module, e.g. "compute" for
// gcp:compute/address:Address.
func resourceModule(urn resource.URN) string {
	parts := strings.Split(string(urn.Type()), ":")
	if len(parts) != 3 {
		return ""
	}
	module, _, _ := strings.Cut(parts[1], "/")
	return module
}

func (p *Package) emulatorName() string {
	if p.emulator.DisplayName != "" {
		return p.emulator.DisplayName
	}
	return "the emulator at " + p.emulator.Endpoint
}

// warnUnsupported reports, once per resource type, a resource the emulator doesn't support.
func (p *Package) warnUnsupported(urn resource.URN) {
	service, missing := p.missingService(urn)
	if !missing || p.sink == nil {
		return
	}
	if _, warned := p.mode.warned.LoadOrStore(urn.Type(), true); warned {
		return
	}
	p.sink.Warningf(diag.RawMessage(urn, fmt.Sprintf(
		"%s isn't emulated by %s (it has no %q service), so creating it will likely fail",
		urn.Type(), p.emulatorName(), service)))
}

// emulatorGap matches the errors providers report when an emulator doesn't implement an API.
var emulatorGap = regexp.MustCompile(`(?i)(response code|status ?code:?) *(404|405|501)\b|not implemented`)

// explain adds a hint to an error from an operation on a resource the emulator may not support.
func (p *Package) explain(urn resource.URN, err error) error {
	if _, isInitErr := errors.AsType[*plugin.InitError](err); err == nil || isInitErr {
		// The engine type-asserts InitErrors, so they must be returned as they are.
		return err
	}
	if service, missing := p.missingService(urn); missing {
		return hintError{err, fmt.Sprintf("%s doesn't emulate the %q service that %s needs",
			p.emulatorName(), service, urn.Type())}
	}
	if emulatorGap.MatchString(err.Error()) {
		return hintError{err, fmt.Sprintf("%s returned an unexpected response; it may not support %s",
			p.emulatorName(), urn.Type())}
	}
	return err
}

// hintError adds a hint to a provider error. Provider errors often end in blank lines, which are trimmed so the hint
// follows the error directly.
type hintError struct {
	err  error
	hint string
}

func (e hintError) Error() string {
	return strings.TrimRight(e.err.Error(), " \t\n") + "\n\nhint: " + e.hint
}

func (e hintError) Unwrap() error { return e.err }

type sandboxProvider struct {
	plugin.Provider

	pkg *Package
}

func (p *sandboxProvider) CheckConfig(
	ctx context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	news := req.News
	// Providers such as pulumi-aws validate credentials here, so they must see the overlay.
	req.News = p.pkg.ApplyConfig(news)
	resp, err := p.Provider.CheckConfig(ctx, req)
	if err != nil || len(resp.Failures) != 0 {
		return resp, err
	}
	resp.Properties = p.pkg.revertConfig(resp.Properties, news)
	return resp, nil
}

func (p *sandboxProvider) Configure(
	ctx context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	req.Inputs = p.pkg.ApplyConfig(req.Inputs)
	return p.Provider.Configure(ctx, req)
}

func (p *sandboxProvider) Check(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	p.pkg.warnUnsupported(req.URN)
	return p.Provider.Check(ctx, req)
}

func (p *sandboxProvider) Create(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	resp, err := p.Provider.Create(ctx, req)
	return resp, p.pkg.explain(req.URN, err)
}

func (p *sandboxProvider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	resp, err := p.Provider.Read(ctx, req)
	return resp, p.pkg.explain(req.URN, err)
}

func (p *sandboxProvider) Update(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	resp, err := p.Provider.Update(ctx, req)
	return resp, p.pkg.explain(req.URN, err)
}

func (p *sandboxProvider) Delete(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	resp, err := p.Provider.Delete(ctx, req)
	return resp, p.pkg.explain(req.URN, err)
}
