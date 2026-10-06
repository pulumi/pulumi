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

// Package sandbox redirects the providers of a sandbox stack at local cloud emulators. Each supported package gets
// a set of environment variables for its plugin process and, optionally, a configuration overlay applied when the
// provider is configured. Packages without a rule are refused, so a sandbox stack never reaches a real cloud through a
// provider we don't know how to redirect.
package sandbox

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	envutil "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// StackTag is the stack tag that marks a sandbox stack. Every deployment of a tagged stack must run in sandbox mode.
const StackTag = "pulumi:sandbox"

//go:embed rules.json
var rulesJSON []byte

const (
	endpointPlaceholder = "{endpoint}"
	hostPlaceholder     = "{host}"
)

type rules struct {
	Emulators map[string]emulatorRule `json:"emulators"`
	Packages  map[string]packageRule  `json:"packages"`
}

type emulatorRule struct {
	DefaultEndpoint string            `json:"defaultEndpoint"`
	Env             map[string]string `json:"env"`
}

type packageRule struct {
	// Emulators the package's plugin needs; they are started if they aren't running. The first one also fills the
	// placeholders in the package's config.
	Emulators []string `json:"emulators"`
	// EnvOnly lists emulators whose environment variables the plugin gets without starting them, for packages such as
	// command that may or may not talk to a cloud.
	EnvOnly        []string       `json:"envOnly"`
	Config         map[string]any `json:"config"`
	ConfigDefaults map[string]any `json:"configDefaults"`
	RemoveConfig   []string       `json:"removeConfig"`
	// Services maps a resource module (the first segment of a type's module, e.g. "compute" for
	// gcp:compute/address:Address) to the name the package's first emulator reports for the service implementing it.
	// Resources whose module isn't listed are never reported as unsupported.
	Services map[string]string `json:"services"`
}

// Emulator describes an emulator a provider is redirected at.
type Emulator struct {
	// Endpoint is the URL the emulator is served at.
	Endpoint string
	// DisplayName names the emulator in messages, e.g. "floci-gcp 0.9.0". It may be empty.
	DisplayName string
	// Services is the set of services the emulator offers, or nil when that isn't known.
	Services map[string]bool
}

// EnsureFunc describes the named emulator. When start is set, it first makes sure the emulator is running, starting
// it if needed; otherwise only the endpoint needs to be filled in.
type EnsureFunc func(ctx context.Context, emulator string, start bool, sink diag.Sink) (Emulator, error)

// Options configures sandbox mode.
type Options struct {
	// Endpoints maps an emulator name (e.g. "aws") to the URL it is served at. Listed emulators are assumed to be
	// running; Ensure is not called for them.
	Endpoints map[string]string
	// Ensure resolves emulators that aren't listed in Endpoints. It is called with start set at most once per emulator,
	// the first time a provider needs it. If Ensure is nil, emulators are assumed to be running at their default
	// endpoint.
	Ensure EnsureFunc
	// AllowedPackages lists extra packages that may run unmodified, such as component providers.
	AllowedPackages []string
}

// Mode holds the resolved sandbox mode rules for a deployment.
type Mode struct {
	rules   rules
	ensure  EnsureFunc
	allowed map[string]bool

	mu        sync.Mutex
	emulators map[string]*resolvedEmulator

	// warned records the resource types already reported as unsupported, so each is reported once per deployment.
	warned sync.Map
}

// resolvedEmulator resolves an emulator once, however many providers need it at the same time.
type resolvedEmulator struct {
	once     sync.Once
	emulator Emulator
	err      error
}

// New resolves the built-in sandbox mode rules against the given options.
func New(opts Options) (*Mode, error) {
	var r rules
	if err := json.Unmarshal(rulesJSON, &r); err != nil {
		return nil, fmt.Errorf("parsing sandbox mode rules: %w", err)
	}

	m := &Mode{rules: r, ensure: opts.Ensure, allowed: map[string]bool{}, emulators: map[string]*resolvedEmulator{}}
	for name, url := range opts.Endpoints {
		if _, ok := r.Emulators[name]; !ok {
			return nil, fmt.Errorf("unknown local emulator %q", name)
		}
		e := &resolvedEmulator{emulator: Emulator{Endpoint: url}}
		e.once.Do(func() {})
		m.emulators[name] = e
	}
	for _, pkg := range opts.AllowedPackages {
		if pkg = strings.TrimSpace(pkg); pkg != "" {
			m.allowed[pkg] = true
		}
	}
	return m, nil
}

// Emulators returns the names of the emulators sandbox mode knows about.
func (m *Mode) Emulators() []string {
	names := slices.Collect(maps.Keys(m.rules.Emulators))
	slices.Sort(names)
	return names
}

// DefaultEndpoint returns the URL the named emulator is served at by default.
func (m *Mode) DefaultEndpoint(emulator string) string {
	return m.rules.Emulators[emulator].DefaultEndpoint
}

func (m *Mode) emulator(ctx context.Context, name string, start bool, sink diag.Sink) (Emulator, error) {
	if !start {
		m.mu.Lock()
		e, ok := m.emulators[name]
		m.mu.Unlock()
		if ok {
			// Wait for any start in progress, then use whatever it resolved.
			e.once.Do(func() {})
			if e.err == nil {
				return e.emulator, nil
			}
		}
		if m.ensure != nil {
			return m.ensure(ctx, name, false, sink)
		}
		return Emulator{Endpoint: m.DefaultEndpoint(name)}, nil
	}

	m.mu.Lock()
	e, ok := m.emulators[name]
	if !ok {
		e = &resolvedEmulator{}
		m.emulators[name] = e
	}
	m.mu.Unlock()

	e.once.Do(func() {
		if m.ensure == nil {
			e.emulator = Emulator{Endpoint: m.DefaultEndpoint(name)}
			return
		}
		e.emulator, e.err = m.ensure(ctx, name, true, sink)
	})
	return e.emulator, e.err
}

// EmulatorEnv returns the environment variables that point clients at the named emulator served at endpoint. An empty
// value means the variable must be unset, so credentials for the real cloud can't take effect.
func (m *Mode) EmulatorEnv(name, endpoint string) (map[string]string, error) {
	emulator, ok := m.rules.Emulators[name]
	if !ok {
		return nil, fmt.Errorf("unknown local emulator %q", name)
	}
	placeholders := newPlaceholders(endpoint)
	vars := make(map[string]string, len(emulator.Env))
	for k, v := range emulator.Env {
		vars[k] = placeholders.Replace(v)
	}
	return vars, nil
}

// CheckPackages returns an error listing every package that a sandbox stack can't use, without starting any
// emulators.
func (m *Mode) CheckPackages(pkgs []tokens.Package) error {
	var unsupported []string
	for _, pkg := range pkgs {
		if _, ok := m.rules.Packages[string(pkg)]; !ok && !m.allowed[string(pkg)] {
			unsupported = append(unsupported, string(pkg))
		}
	}
	if len(unsupported) == 0 {
		return nil
	}
	slices.Sort(unsupported)
	unsupported = slices.Compact(unsupported)
	return fmt.Errorf("sandbox stacks can't use %s; supported packages are %s. "+
		"Set PULUMI_SANDBOX_ALLOW_PACKAGES to allow a package that never talks to a cloud",
		quoteList(unsupported), strings.Join(m.supportedPackages(), ", "))
}

func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}

// Package returns the rule for a provider package, making sure the emulators it needs are running. It returns an error
// if the package cannot be used by a sandbox stack.
func (m *Mode) Package(ctx context.Context, pkg tokens.Package, sink diag.Sink) (*Package, error) {
	rule, ok := m.rules.Packages[string(pkg)]
	if !ok {
		if m.allowed[string(pkg)] {
			return &Package{env: envutil.MapStore{}}, nil
		}
		if strings.HasPrefix(string(pkg), "azure") {
			return nil, fmt.Errorf("package %q is not supported for sandbox stacks: Azure isn't supported yet", pkg)
		}
		return nil, fmt.Errorf("package %q is not supported for sandbox stacks; supported packages are %s",
			pkg, strings.Join(m.supportedPackages(), ", "))
	}

	vars := envutil.MapStore{}
	var configPlaceholders *strings.Replacer
	var primary Emulator
	emulators := slices.Concat(rule.Emulators, rule.EnvOnly)
	for i, name := range emulators {
		emulator, ok := m.rules.Emulators[name]
		if !ok {
			return nil, fmt.Errorf("package %q references unknown local emulator %q", pkg, name)
		}
		start := i < len(rule.Emulators)
		resolved, err := m.emulator(ctx, name, start, sink)
		if err != nil {
			return nil, err
		}
		placeholders := newPlaceholders(resolved.Endpoint)
		for k, v := range emulator.Env {
			vars[k] = placeholders.Replace(v)
		}
		if i == 0 && start {
			configPlaceholders = placeholders
			primary = resolved
		}
	}

	overlay, err := toValues(rule.Config, configPlaceholders)
	if err != nil {
		return nil, fmt.Errorf("sandbox mode config for package %q: %w", pkg, err)
	}
	defaults, err := toValues(rule.ConfigDefaults, configPlaceholders)
	if err != nil {
		return nil, fmt.Errorf("sandbox mode config defaults for package %q: %w", pkg, err)
	}

	return &Package{
		mode:     m,
		sink:     sink,
		env:      vars,
		overlay:  overlay,
		defaults: defaults,
		remove:   rule.RemoveConfig,
		emulator: primary,
		services: rule.Services,
	}, nil
}

func newPlaceholders(endpoint string) *strings.Replacer {
	host := endpoint
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		host = u.Host
	}
	return strings.NewReplacer(endpointPlaceholder, endpoint, hostPlaceholder, host)
}

func (m *Mode) supportedPackages() []string {
	pkgs := slices.Collect(maps.Keys(m.rules.Packages))
	for pkg := range m.allowed {
		pkgs = append(pkgs, pkg)
	}
	slices.Sort(pkgs)
	return pkgs
}

func toValues(m map[string]any, placeholders *strings.Replacer) (map[string]property.Value, error) {
	values := make(map[string]property.Value, len(m))
	for k, v := range m {
		if str, ok := v.(string); ok && placeholders != nil {
			v = placeholders.Replace(str)
		}
		pv, err := property.Any(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		values[k] = pv
	}
	return values, nil
}
