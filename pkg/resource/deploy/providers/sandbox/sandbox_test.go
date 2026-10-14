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
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestPackageEnv(t *testing.T) {
	t.Parallel()

	mode, err := New(Options{Endpoints: map[string]string{"aws": "http://127.0.0.1:9999"}})
	require.NoError(t, err)

	for _, pkg := range []string{"aws", "command", "pulumi-nodejs"} {
		p, err := mode.Package(t.Context(), tokens.Package(pkg), nil)
		require.NoError(t, err, pkg)
		endpoint, ok := p.Env().Raw("AWS_ENDPOINT_URL")
		assert.True(t, ok, pkg)
		assert.Equal(t, "http://127.0.0.1:9999", endpoint, pkg)
		profile, ok := p.Env().Raw("AWS_PROFILE")
		assert.True(t, ok, "%s: AWS_PROFILE must be cleared, not left to the ambient environment", pkg)
		assert.Empty(t, profile, pkg)
	}

	random, err := mode.Package(t.Context(), "random", nil)
	require.NoError(t, err)
	_, ok := random.Env().Raw("AWS_ENDPOINT_URL")
	assert.False(t, ok)
}

func TestUnknownEmulator(t *testing.T) {
	t.Parallel()

	_, err := New(Options{Endpoints: map[string]string{"nope": "http://localhost:1"}})
	require.ErrorContains(t, err, `unknown local emulator "nope"`)
}

func TestGCPPackage(t *testing.T) {
	t.Parallel()

	mode, err := New(Options{Endpoints: map[string]string{"gcp": "http://127.0.0.1:9999"}})
	require.NoError(t, err)
	p, err := mode.Package(t.Context(), "gcp", nil)
	require.NoError(t, err)

	token, _ := p.Env().Raw("GOOGLE_OAUTH_ACCESS_TOKEN")
	assert.Equal(t, "floci-local-token", token)
	storageHost, _ := p.Env().Raw("STORAGE_EMULATOR_HOST")
	assert.Equal(t, "http://127.0.0.1:9999", storageHost)
	creds, ok := p.Env().Raw("GOOGLE_APPLICATION_CREDENTIALS")
	assert.True(t, ok)
	assert.Empty(t, creds)

	got := p.ApplyConfig(property.NewMap(map[string]property.Value{
		"credentials": property.New(`{"type": "service_account"}`),
		"project":     property.New("my-project"),
	}))
	assert.Equal(t, "http://127.0.0.1:9999/storage/v1/", got.Get("storageCustomEndpoint").AsString())
	assert.Equal(t, "http://127.0.0.1:9999/compute/v1/", got.Get("computeCustomEndpoint").AsString())
	assert.Equal(t, "floci-local-token", got.Get("accessToken").AsString())
	assert.Equal(t, "my-project", got.Get("project").AsString(), "an explicit project is kept")
	assert.Equal(t, "us-central1", got.Get("region").AsString())
	_, ok = got.GetOk("credentials")
	assert.False(t, ok)

	got = p.ApplyConfig(property.Map{})
	assert.Equal(t, "floci-local", got.Get("project").AsString(), "the project defaults to floci's")
}

func TestAzureIsNotSupportedYet(t *testing.T) {
	t.Parallel()

	mode, err := New(Options{})
	require.NoError(t, err)
	for _, pkg := range []tokens.Package{"azure-native", "azure"} {
		_, err := mode.Package(t.Context(), pkg, nil)
		require.ErrorContains(t, err, "Azure isn't supported yet", pkg)
	}
}

// Emulators are started once, the first time a provider needs them, however many providers load at the same time.
func TestEnsureStartsEmulatorsOnDemand(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	starts := map[string]int{}
	lookups := map[string]int{}
	mode, err := New(Options{
		Ensure: func(_ context.Context, name string, start bool, _ diag.Sink) (Emulator, error) {
			mu.Lock()
			defer mu.Unlock()
			if start {
				starts[name]++
			} else {
				lookups[name]++
			}
			if start && name == "gcp" {
				return Emulator{}, errors.New("floci-gcp is not running")
			}
			return Emulator{Endpoint: "http://localhost:14566"}, nil
		},
	})
	require.NoError(t, err)

	_, err = mode.Package(t.Context(), "random", nil)
	require.NoError(t, err)
	_, err = mode.Package(t.Context(), "command", nil)
	require.NoError(t, err)
	assert.Empty(t, starts, "packages that don't need an emulator never start one")

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			p, err := mode.Package(t.Context(), "aws", nil)
			if assert.Nil(t, err) {
				endpoint, _ := p.Env().Raw("AWS_ENDPOINT_URL")
				assert.Equal(t, "http://localhost:14566", endpoint)
			}
		}()
		go func() {
			defer wg.Done()
			_, err := mode.Package(t.Context(), "gcp", nil)
			assert.ErrorContains(t, err, "floci-gcp is not running")
		}()
	}
	wg.Wait()
	assert.Equal(t, map[string]int{"aws": 1, "gcp": 1}, starts)

	// Once aws is running, command gets its endpoint without another lookup.
	lookupsBefore := lookups["aws"]
	p, err := mode.Package(t.Context(), "command", nil)
	require.NoError(t, err)
	endpoint, _ := p.Env().Raw("AWS_ENDPOINT_URL")
	assert.Equal(t, "http://localhost:14566", endpoint)
	assert.Equal(t, lookupsBefore, lookups["aws"])
}

func TestUnsupportedPackages(t *testing.T) {
	t.Parallel()

	mode, err := New(Options{AllowedPackages: []string{" my-component ", ""}})
	require.NoError(t, err)

	_, err = mode.Package(t.Context(), "kubernetes", nil)
	require.ErrorContains(t, err, `package "kubernetes" is not supported for sandbox stacks`)
	require.ErrorContains(t, err, "my-component")

	p, err := mode.Package(t.Context(), "my-component", nil)
	require.NoError(t, err)
	inner := &recordingProvider{}
	assert.Same(t, inner, p.Wrap(inner), "allowed packages run unmodified")
}

func TestApplyConfig(t *testing.T) {
	t.Parallel()

	mode, err := New(Options{})
	require.NoError(t, err)
	p, err := mode.Package(t.Context(), "aws", nil)
	require.NoError(t, err)

	got := p.ApplyConfig(property.NewMap(map[string]property.Value{
		"profile":   property.New("production"),
		"accessKey": property.New("AKIAREAL"),
		"endpoints": property.New([]property.Value{}),
	}))
	assert.Equal(t, "test", got.Get("accessKey").AsString())
	assert.Equal(t, "us-east-1", got.Get("region").AsString(), "the region defaults when unset")
	assert.True(t, got.Get("s3UsePathStyle").AsBool())
	for _, k := range []string{"profile", "endpoints"} {
		_, ok := got.GetOk(k)
		assert.False(t, ok, k)
	}

	got = p.ApplyConfig(property.NewMap(map[string]property.Value{"region": property.New("eu-west-1")}))
	assert.Equal(t, "eu-west-1", got.Get("region").AsString(), "an explicit region is kept")
}

func TestWrapRewritesProviderConfig(t *testing.T) {
	t.Parallel()

	mode, err := New(Options{})
	require.NoError(t, err)
	p, err := mode.Package(t.Context(), "aws", nil)
	require.NoError(t, err)

	inner := &recordingProvider{}
	wrapped := p.Wrap(inner)
	news := property.NewMap(map[string]property.Value{"profile": property.New("production")})

	resp, err := wrapped.CheckConfig(t.Context(), plugin.CheckConfigRequest{News: news})
	require.NoError(t, err)
	_, ok := inner.checked.GetOk("profile")
	assert.False(t, ok, "the provider validates the overlaid config")
	assert.Equal(t, "test", inner.checked.Get("accessKey").AsString())
	assert.Equal(t, news, resp.Properties, "checked properties are restored to the declared config")

	_, err = wrapped.Configure(t.Context(), plugin.ConfigureRequest{Inputs: news})
	require.NoError(t, err)
	_, ok = inner.configured.GetOk("profile")
	assert.False(t, ok)
	assert.Equal(t, "test", inner.configured.Get("accessKey").AsString())
}

type recordingProvider struct {
	plugin.UnimplementedProvider

	checked    property.Map
	configured property.Map
	checks     int
	createErr  error
}

func (p *recordingProvider) Check(context.Context, plugin.CheckRequest) (plugin.CheckResponse, error) {
	p.checks++
	return plugin.CheckResponse{}, nil
}

func (p *recordingProvider) Create(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
	return plugin.CreateResponse{}, p.createErr
}

func (p *recordingProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	p.checked = req.News
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *recordingProvider) Configure(
	_ context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	p.configured = req.Inputs
	return plugin.ConfigureResponse{}, nil
}

func newGCPPackage(t *testing.T, services map[string]bool, sink diag.Sink) *Package {
	mode, err := New(Options{
		Ensure: func(context.Context, string, bool, diag.Sink) (Emulator, error) {
			return Emulator{Endpoint: "http://localhost:4588", DisplayName: "floci-gcp 0.9.0", Services: services}, nil
		},
	})
	require.NoError(t, err)
	p, err := mode.Package(t.Context(), "gcp", sink)
	require.NoError(t, err)
	return p
}

// Resources the emulator doesn't offer a service for are reported once per type, and still sent to the provider.
func TestUnsupportedResourceWarning(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	sink := diag.DefaultSink(io.Discard, &out, diag.FormatOptions{Color: colors.Never})
	p := newGCPPackage(t, map[string]bool{"gcs": true}, sink)
	inner := &recordingProvider{}
	wrapped := p.Wrap(inner)

	for _, urn := range []resource.URN{
		"urn:pulumi:dev::proj::gcp:compute/address:Address::ip",
		"urn:pulumi:dev::proj::gcp:compute/address:Address::ip2",
		"urn:pulumi:dev::proj::gcp:storage/bucket:Bucket::bucket",
		"urn:pulumi:dev::proj::gcp:dns/managedZone:ManagedZone::zone",
	} {
		_, err := wrapped.Check(t.Context(), plugin.CheckRequest{URN: urn})
		require.NoError(t, err)
	}

	assert.Equal(t, 4, inner.checks, "every resource still reaches the provider")
	assert.Equal(t, 1, strings.Count(out.String(), "isn't emulated by floci-gcp 0.9.0"), out.String())
	assert.Contains(t, out.String(),
		`gcp:compute/address:Address isn't emulated by floci-gcp 0.9.0 (it has no "compute" service)`)
	assert.NotContains(t, out.String(), "storage", "supported services aren't reported")
	assert.NotContains(t, out.String(), "dns", "unmapped modules aren't reported")
}

func TestUnknownServicesAreNotReported(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	sink := diag.DefaultSink(io.Discard, &out, diag.FormatOptions{Color: colors.Never})
	wrapped := newGCPPackage(t, nil, sink).Wrap(&recordingProvider{})
	_, err := wrapped.Check(t.Context(), plugin.CheckRequest{URN: "urn:pulumi:dev::proj::gcp:compute/address:Address::ip"})
	require.NoError(t, err)
	assert.Empty(t, out.String())
}

func TestFailureHints(t *testing.T) {
	t.Parallel()

	const (
		address = resource.URN("urn:pulumi:dev::proj::gcp:compute/address:Address::ip")
		zone    = resource.URN("urn:pulumi:dev::proj::gcp:dns/managedZone:ManagedZone::z")
		bucket  = resource.URN("urn:pulumi:dev::proj::gcp:storage/bucket:Bucket::b")
	)

	p := newGCPPackage(t, map[string]bool{"gcs": true}, nil)
	inner := &recordingProvider{}
	wrapped := p.Wrap(inner)

	inner.createErr = errors.New("Error creating Address: googleapi: got HTTP response code 405 with body: ")
	_, err := wrapped.Create(t.Context(), plugin.CreateRequest{URN: address})
	require.ErrorIs(t, err, inner.createErr)
	assert.Equal(t, "Error creating Address: googleapi: got HTTP response code 405 with body:\n\n"+
		`hint: floci-gcp 0.9.0 doesn't emulate the "compute" service that gcp:compute/address:Address needs`,
		err.Error(), "the hint follows the error without the provider's trailing blank lines")

	inner.createErr = errors.New("Error creating ManagedZone: googleapi: got HTTP response code 404 with body: ")
	_, err = wrapped.Create(t.Context(), plugin.CreateRequest{URN: zone})
	assert.Contains(t, err.Error(),
		"hint: floci-gcp 0.9.0 returned an unexpected response; it may not support gcp:dns/managedZone:ManagedZone")

	inner.createErr = errors.New("bucket name already taken")
	_, err = wrapped.Create(t.Context(), plugin.CreateRequest{URN: bucket})
	assert.Equal(t, "bucket name already taken", err.Error(), "unrelated errors are left alone")

	initErr := &plugin.InitError{Reasons: []string{"response code 405"}}
	inner.createErr = initErr
	_, err = wrapped.Create(t.Context(), plugin.CreateRequest{URN: address})
	assert.Same(t, initErr, err, "the engine type-asserts InitErrors, so they pass through unwrapped")
}

func TestCheckPackages(t *testing.T) {
	t.Parallel()

	mode, err := New(Options{AllowedPackages: []string{"my-component"}})
	require.NoError(t, err)

	require.NoError(t, mode.CheckPackages([]tokens.Package{"aws", "gcp", "random", "my-component"}))

	err = mode.CheckPackages([]tokens.Package{"aws", "kubernetes", "cloudflare", "kubernetes"})
	require.ErrorContains(t, err, `sandbox stacks can't use "cloudflare", "kubernetes";`)
	assert.Contains(t, err.Error(), "PULUMI_SANDBOX_ALLOW_PACKAGES")
}
