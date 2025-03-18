// Copyright 2016-2018, Pulumi Corporation.
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

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// The package name for the NodeJS dynamic provider.
const nodejsDynamicProviderPackage = "pulumi-nodejs"

// The `Type()` for the NodeJS dynamic provider.  Logically, this is the same as calling
// providers.MakeProviderType(tokens.Package("pulumi-nodejs")), but does not depend on the providers package
// (a direct dependency would cause a cyclic import issue.
//
// This is needed because we have to handle some buggy behavior that previous versions of this provider implemented.
const nodejsDynamicProviderType = "pulumi:providers:" + nodejsDynamicProviderPackage

// The `Type()` for the Kubernetes provider.  Logically, this is the same as calling
// providers.MakeProviderType(tokens.Package("kubernetes")), but does not depend on the providers package
// (a direct dependency would cause a cyclic import issue.
//
// This is needed because we have to handle some buggy behavior that previous versions of this provider implemented.
const kubernetesProviderType = "pulumi:providers:kubernetes"

// provider reflects a resource plugin, loaded dynamically for a single package.
type provider struct {
	NotForwardCompatibleProvider

	ctx                    *Context                         // a plugin context for caching, etc.
	pkg                    tokens.Package                   // the Pulumi package containing this provider's resources.
	plug                   *plugin                          // the actual plugin process wrapper.
	clientRaw              pulumirpc.ResourceProviderClient // the raw provider client; usually unsafe to use directly.
	disableProviderPreview bool                             // true if previews for Create and Update are disabled.
	legacyPreview          bool                             // enables legacy behavior for unconfigured provider previews.

	// Protocol information for the provider.
	protocol *pluginProtocol

	// The source for the provider's configuration. You should *not* access this field directly when checking
	// configuration, since some of its fields have been deprecated in favour of handshake. Instead, use the
	// getPluginConfig method, which will return both the handshake and a pluginConfig consistent with the handshake.
	configSource *promise.CompletionSource[pluginConfig] // the source for the provider's configuration.
}

type pluginProtocol struct {
	// True if the provider accepts strongly-typed secrets.
	acceptSecrets bool

	// True if the provider accepts strongly-typed resource references.
	acceptResources bool

	// True if this plugin accepts output values.
	acceptOutputs bool

	// True if this plugin supports previews for Create and Update.
	supportsPreview bool

	// True if this plugin supports custom autonaming configuration.
	supportsAutonamingConfiguration bool
}

// pluginConfig holds the configuration of the provider
// as specified by the Configure call.
type pluginConfig struct {
	known bool // true if all configuration values are known.
}

func (p *provider) getPluginConfig(ctx context.Context) (pluginProtocol, pluginConfig, error) {
	pcfg, err := p.configSource.Promise().Result(ctx)
	if err != nil {
		return pluginProtocol{}, pluginConfig{}, err
	}

	if p.protocol == nil {
		return pluginProtocol{}, pluginConfig{}, errors.New(
			"Protocol must be configured by the time plugin configuration has been resolved",
		)
	}

	return *p.protocol, pcfg, nil
}

// Checks PULUMI_DEBUG_PROVIDERS environment variable for any overrides for the provider identified
// by pkg. If the user has requested to attach to a live provider, returns the port number from the
// env var. For example, `PULUMI_DEBUG_PROVIDERS=aws:12345,gcp:678` will result in 12345 for aws.
func GetProviderAttachPort(pkg tokens.Package) (*int, error) {
	var optAttach string

	if providersEnvVar, has := os.LookupEnv("PULUMI_DEBUG_PROVIDERS"); has {
		for _, provider := range strings.Split(providersEnvVar, ",") {
			parts := strings.SplitN(provider, ":", 2)

			if parts[0] == pkg.String() {
				optAttach = parts[1]
				break
			}
		}
	}

	if optAttach == "" {
		return nil, nil
	}

	port, err := strconv.Atoi(optAttach)
	if err != nil {
		return nil, fmt.Errorf("Expected a numeric port, got %s in PULUMI_DEBUG_PROVIDERS: %w",
			optAttach, err)
	}
	return &port, nil
}

// NewProvider attempts to bind to a given package's resource plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewProvider(host Host, ctx *Context, spec workspace.PluginSpec,
	options map[string]interface{}, disableProviderPreview bool, jsonConfig string,
	projectName tokens.PackageName,
) (Provider, error) {
	// See if this is a provider we just want to attach to
	var plug *plugin
	var handshakeRes *ProviderHandshakeResponse

	pkg := tokens.Package(spec.Name)

	attachPort, err := GetProviderAttachPort(pkg)
	if err != nil {
		return nil, err
	}

	prefix := fmt.Sprintf("%v (resource)", pkg)

	if attachPort != nil {
		port := *attachPort

		handshake := func(
			ctx context.Context, bin string, prefix string, conn *grpc.ClientConn,
		) (*ProviderHandshakeResponse, error) {
			req := &ProviderHandshakeRequest{
				EngineAddress: host.ServerAddr(),
				// If we're attaching then we don't know the root or program directory.
				RootDirectory:    nil,
				ProgramDirectory: nil,
				ConfigureWithUrn: true,
			}
			return handshake(ctx, bin, prefix, conn, req)
		}

		var conn *grpc.ClientConn
		conn, handshakeRes, err = dialPlugin(port, pkg.String(), prefix,
			handshake, providerPluginDialOptions(ctx, pkg, ""))
		if err != nil {
			return nil, err
		}

		// Done; store the connection and return the plugin info.
		plug = &plugin{
			Conn: conn,
			// Nothing to kill
			Kill: func() error { return nil },
		}
	} else {
		// Load the plugin's path by using the standard workspace logic.
		path, err := workspace.GetPluginPath(ctx.Diag, spec, host.GetProjectPlugins())
		if err != nil {
			return nil, err
		}

		contract.Assertf(path != "", "unexpected empty path for plugin %s", pkg)

		// Runtime options are passed as environment variables to the provider, this is _currently_ used by
		// dynamic providers to do things like lookup the virtual environment to use.
		env := os.Environ()
		for k, v := range options {
			env = append(env, fmt.Sprintf("PULUMI_RUNTIME_%s=%v", strings.ToUpper(k), v))
		}
		if projectName != "" {
			if pkg == tokens.Package(nodejsDynamicProviderPackage) {
				// The Node.js SDK uses PULUMI_NODEJS_PROJECT to set the project name.
				// Eventually, we should standardize on PULUMI_PROJECT for all SDKs.
				// Also see `constructEnv` in sdk/go/common/resource/plugin/analyzer_plugin.go
				env = append(env, fmt.Sprintf("PULUMI_NODEJS_PROJECT=%s", projectName))
			}
			env = append(env, fmt.Sprintf("PULUMI_PROJECT=%s", projectName))
		}
		if jsonConfig != "" {
			env = append(env, "PULUMI_CONFIG="+jsonConfig)
		}

		handshake := func(
			ctx context.Context, bin string, prefix string, conn *grpc.ClientConn,
		) (*ProviderHandshakeResponse, error) {
			dir := filepath.Dir(bin)
			req := &ProviderHandshakeRequest{
				EngineAddress:    host.ServerAddr(),
				RootDirectory:    &dir,
				ProgramDirectory: &dir,
				ConfigureWithUrn: true,
			}
			return handshake(ctx, bin, prefix, conn, req)
		}

		plug, handshakeRes, err = newPlugin(ctx, ctx.Pwd, path, prefix,
			apitype.ResourcePlugin, []string{host.ServerAddr()}, env,
			handshake, providerPluginDialOptions(ctx, pkg, ""))
		if err != nil {
			return nil, err
		}
	}

	contract.Assertf(plug != nil, "unexpected nil resource plugin for %s", pkg)

	legacyPreview := cmdutil.IsTruthy(os.Getenv("PULUMI_LEGACY_PROVIDER_PREVIEW"))

	p := &provider{
		ctx:                    ctx,
		pkg:                    pkg,
		plug:                   plug,
		clientRaw:              pulumirpc.NewResourceProviderClient(plug.Conn),
		disableProviderPreview: disableProviderPreview,
		legacyPreview:          legacyPreview,
		configSource:           &promise.CompletionSource[pluginConfig]{},
	}

	if handshakeRes != nil {
		p.protocol = &pluginProtocol{
			acceptSecrets:                   handshakeRes.AcceptSecrets,
			acceptResources:                 handshakeRes.AcceptResources,
			supportsPreview:                 true,
			acceptOutputs:                   handshakeRes.AcceptOutputs,
			supportsAutonamingConfiguration: handshakeRes.SupportsAutonamingConfiguration,
		}
	}

	// If we just attached (i.e. plugin bin is nil) we need to call attach
	if plug.Bin == "" {
		err := p.Attach(host.ServerAddr())
		if err != nil {
			return nil, err
		}
	}

	return p, nil
}

func handshake(
	ctx context.Context,
	bin string,
	prefix string,
	conn *grpc.ClientConn,
	req *ProviderHandshakeRequest,
) (*ProviderHandshakeResponse, error) {
	client := pulumirpc.NewResourceProviderClient(conn)
	res, err := client.Handshake(ctx, &pulumirpc.ProviderHandshakeRequest{
		EngineAddress:    req.EngineAddress,
		RootDirectory:    req.RootDirectory,
		ProgramDirectory: req.ProgramDirectory,
		ConfigureWithUrn: req.ConfigureWithUrn,
	})
	if err != nil {
		status, ok := status.FromError(err)
		if ok && status.Code() == codes.Unimplemented {
			// If the provider doesn't implement Handshake, that's fine -- we'll fall back to existing behaviour.
			logging.V(7).Infof("Handshake: not supported by '%v'", bin)
			return nil, nil
		}
	}

	logging.V(7).Infof("Handshake: success [%v]", bin)
	return &ProviderHandshakeResponse{
		AcceptSecrets:                   res.GetAcceptSecrets(),
		AcceptResources:                 res.GetAcceptResources(),
		AcceptOutputs:                   res.GetAcceptOutputs(),
		SupportsAutonamingConfiguration: res.GetSupportsAutonamingConfiguration(),
	}, nil
}

func providerPluginDialOptions(ctx *Context, pkg tokens.Package, path string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.OpenTracingInterceptorDialOptions(otgrpc.SpanDecorator(decorateProviderSpans)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]interface{}{
			"mode": "client",
			"kind": "resource",
		}
		if pkg != "" {
			metadata["name"] = pkg.String()
		}
		if path != "" {
			metadata["path"] = path
		}
		dialOpts = append(dialOpts, ctx.DialOptions(metadata)...)
	}

	return dialOpts
}

// NewProviderFromPath creates a new provider by loading the plugin binary located at `path`.
func NewProviderFromPath(host Host, ctx *Context, path string) (Provider, error) {
	env := os.Environ()

	handshake := func(
		ctx context.Context, bin string, prefix string, conn *grpc.ClientConn,
	) (*ProviderHandshakeResponse, error) {
		dir := filepath.Dir(bin)
		req := &ProviderHandshakeRequest{
			EngineAddress:    host.ServerAddr(),
			RootDirectory:    &dir,
			ProgramDirectory: &dir,
			ConfigureWithUrn: true,
		}
		return handshake(ctx, bin, prefix, conn, req)
	}

	plug, handshakeRes, err := newPlugin(ctx, ctx.Pwd, path, "",
		apitype.ResourcePlugin, []string{host.ServerAddr()}, env,
		handshake, providerPluginDialOptions(ctx, "", path))
	if err != nil {
		return nil, err
	}
	contract.Assertf(plug != nil, "unexpected nil resource plugin at %q", path)

	legacyPreview := cmdutil.IsTruthy(os.Getenv("PULUMI_LEGACY_PROVIDER_PREVIEW"))

	p := &provider{
		ctx:           ctx,
		plug:          plug,
		clientRaw:     pulumirpc.NewResourceProviderClient(plug.Conn),
		legacyPreview: legacyPreview,
		configSource:  &promise.CompletionSource[pluginConfig]{},
	}

	if handshakeRes != nil {
		p.protocol = &pluginProtocol{
			acceptSecrets:                   handshakeRes.AcceptSecrets,
			acceptResources:                 handshakeRes.AcceptResources,
			supportsPreview:                 true,
			acceptOutputs:                   handshakeRes.AcceptOutputs,
			supportsAutonamingConfiguration: handshakeRes.SupportsAutonamingConfiguration,
		}
	}

	// If we just attached (i.e. plugin bin is nil) we need to call attach
	if plug.Bin == "" {
		err := p.Attach(host.ServerAddr())
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

func NewProviderWithClient(ctx *Context, pkg tokens.Package, client pulumirpc.ResourceProviderClient,
	disableProviderPreview bool,
) Provider {
	return &provider{
		ctx:                    ctx,
		pkg:                    pkg,
		clientRaw:              client,
		disableProviderPreview: disableProviderPreview,
		configSource:           &promise.CompletionSource[pluginConfig]{},
	}
}

func (p *provider) Pkg() tokens.Package { return p.pkg }

// label returns a base label for tracing functions.
func (p *provider) label() string {
	return fmt.Sprintf("Provider[%s, %p]", p.pkg, p)
}

func (p *provider) requestContext() context.Context {
	if p.ctx == nil {
		return context.Background()
	}
	return p.ctx.Request()
}

// isDiffCheckConfigLogicallyUnimplemented returns true when an rpcerror.Error should be treated as if it was an error
// due to a rpc being unimplemented. Due to past mistakes, different providers returned "Unimplemented" in a variaity of
// different ways that don't always result in an Uimplemented error code.
func isDiffCheckConfigLogicallyUnimplemented(err *rpcerror.Error, providerType tokens.Type) bool {
	switch string(providerType) {
	// The NodeJS dynamic provider implementation incorrectly returned an empty message instead of properly implementing
	// Diff/CheckConfig.  This gets turned into a error with type: "Internal".
	case nodejsDynamicProviderType:
		if err.Code() == codes.Internal {
			logging.V(8).Infof("treating error %s as unimplemented error", err)
			return true
		}

	// The Kubernetes provider returned an "Unimplmeneted" message, but it did so by returning a status from a different
	// package that the provider was expected. That caused the error to be wrapped with an "Unknown" error.
	case kubernetesProviderType:
		if err.Code() == codes.Unknown && strings.Contains(err.Message(), "Unimplemented") {
			logging.V(8).Infof("treating error %s as unimplemented error", err)
			return true
		}
	}

	return false
}

func (p *provider) Handshake(ctx context.Context, req ProviderHandshakeRequest) (*ProviderHandshakeResponse, error) {
	res, err := p.clientRaw.Handshake(ctx, &pulumirpc.ProviderHandshakeRequest{
		EngineAddress:    req.EngineAddress,
		RootDirectory:    req.RootDirectory,
		ProgramDirectory: req.ProgramDirectory,
		ConfigureWithUrn: req.ConfigureWithUrn,
	})
	if err != nil {
		return nil, err
	}

	return &ProviderHandshakeResponse{
		AcceptSecrets:                   res.GetAcceptSecrets(),
		AcceptResources:                 res.GetAcceptResources(),
		AcceptOutputs:                   res.GetAcceptOutputs(),
		SupportsAutonamingConfiguration: res.GetSupportsAutonamingConfiguration(),
	}, nil
}

func (p *provider) Parameterize(ctx context.Context, request ParameterizeRequest) (ParameterizeResponse, error) {
	var params pulumirpc.ParameterizeRequest
	switch p := request.Parameters.(type) {
	case *ParameterizeArgs:
		params.Parameters = &pulumirpc.ParameterizeRequest_Args{
			Args: &pulumirpc.ParameterizeRequest_ParametersArgs{
				Args: p.Args,
			},
		}
	case *ParameterizeValue:
		params.Parameters = &pulumirpc.ParameterizeRequest_Value{
			Value: &pulumirpc.ParameterizeRequest_ParametersValue{
				Name:    p.Name,
				Version: p.Version.String(),
				Value:   p.Value,
			},
		}
	case nil:
		// No args present. That should be Ok.
	default:
		panic(fmt.Sprintf("Impossible - type is constrained to ParameterizeArgs or ParameterizeValue, found %T", p))
	}
	resp, err := p.clientRaw.Parameterize(p.requestContext(), &params)
	if err != nil {
		return ParameterizeResponse{}, err
	}
	version, err := semver.Parse(resp.Version)
	if err != nil {
		return ParameterizeResponse{}, err
	}
	return ParameterizeResponse{Name: resp.Name, Version: version}, err
}

// GetSchema fetches the schema for this resource provider, if any.
func (p *provider) GetSchema(ctx context.Context, req GetSchemaRequest) (GetSchemaResponse, error) {
	var subpackageVersion string
	if req.SubpackageVersion != nil {
		subpackageVersion = req.SubpackageVersion.String()
	}

	resp, err := p.clientRaw.GetSchema(p.requestContext(), &pulumirpc.GetSchemaRequest{
		Version:           req.Version,
		SubpackageName:    req.SubpackageName,
		SubpackageVersion: subpackageVersion,
	})
	if err != nil {
		return GetSchemaResponse{}, err
	}
	return GetSchemaResponse{[]byte(resp.GetSchema())}, nil
}

// CheckConfig validates the configuration for this resource provider.
func (p *provider) CheckConfig(ctx context.Context, req CheckConfigRequest) (CheckConfigResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	label := fmt.Sprintf("%s.CheckConfig(%s)", p.label(), req.URN)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d)", label, len(req.Olds), len(req.News))

	molds, err := MarshalProperties(req.Olds, MarshalOptions{
		Label:        label + ".olds",
		KeepUnknowns: req.AllowUnknowns,
	})
	if err != nil {
		return CheckConfigResponse{}, err
	}

	mnews, err := MarshalProperties(req.News, MarshalOptions{
		Label:        label + ".news",
		KeepUnknowns: req.AllowUnknowns,
	})
	if err != nil {
		return CheckConfigResponse{}, err
	}

	resp, err := p.clientRaw.CheckConfig(p.requestContext(), &pulumirpc.CheckRequest{
		Urn:  string(req.URN),
		Name: req.URN.Name(),
		Type: req.URN.Type().String(),
		Olds: molds,
		News: mnews,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented || isDiffCheckConfigLogicallyUnimplemented(rpcError, req.URN.Type()) {
			// For backwards compatibility, just return the news as if the provider was okay with them.
			logging.V(7).Infof("%s unimplemented rpc: returning news as is", label)
			return CheckConfigResponse{Properties: req.News}, nil
		}
		logging.V(8).Infof("%s provider received rpc error `%s`: `%s`", label, rpcError.Code(),
			rpcError.Message())
		return CheckConfigResponse{}, err
	}

	// Unmarshal the provider inputs.
	var inputs resource.PropertyMap
	if ins := resp.GetInputs(); ins != nil {
		inputs, err = UnmarshalProperties(ins, MarshalOptions{
			Label:          label + ".inputs",
			KeepUnknowns:   req.AllowUnknowns,
			RejectUnknowns: !req.AllowUnknowns,
			KeepSecrets:    true,
			KeepResources:  true,
		})
		if err != nil {
			return CheckConfigResponse{}, err
		}
	}

	// And now any properties that failed verification.
	failures := slice.Prealloc[CheckFailure](len(resp.GetFailures()))
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	// Copy over any secret annotations, since we could not pass any to the provider, and return.
	annotateSecrets(inputs, req.News)
	logging.V(7).Infof("%s success: inputs=#%d failures=#%d", label, len(inputs), len(failures))
	return CheckConfigResponse{Properties: inputs, Failures: failures}, nil
}

func decodeDetailedDiff(resp *pulumirpc.DiffResponse) map[string]PropertyDiff {
	if !resp.GetHasDetailedDiff() {
		return nil
	}

	detailedDiff := make(map[string]PropertyDiff)
	for k, v := range resp.GetDetailedDiff() {
		var d DiffKind
		switch v.GetKind() {
		case pulumirpc.PropertyDiff_ADD:
			d = DiffAdd
		case pulumirpc.PropertyDiff_ADD_REPLACE:
			d = DiffAddReplace
		case pulumirpc.PropertyDiff_DELETE:
			d = DiffDelete
		case pulumirpc.PropertyDiff_DELETE_REPLACE:
			d = DiffDeleteReplace
		case pulumirpc.PropertyDiff_UPDATE:
			d = DiffUpdate
		case pulumirpc.PropertyDiff_UPDATE_REPLACE:
			d = DiffUpdateReplace
		default:
			// Consider unknown diff kinds to be simple updates.
			d = DiffUpdate
		}
		detailedDiff[k] = PropertyDiff{
			Kind:      d,
			InputDiff: v.GetInputDiff(),
		}
	}

	return detailedDiff
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *provider) DiffConfig(ctx context.Context, req DiffConfigRequest) (DiffConfigResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	label := fmt.Sprintf("%s.DiffConfig(%s)", p.label(), req.URN)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(req.OldInputs), len(req.OldOutputs), len(req.NewInputs))

	mOldInputs, err := MarshalProperties(req.OldInputs, MarshalOptions{
		Label:        label + ".oldInputs",
		KeepUnknowns: true,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mOldOutputs, err := MarshalProperties(req.OldOutputs, MarshalOptions{
		Label:        label + ".oldOutputs",
		KeepUnknowns: true,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mNewInputs, err := MarshalProperties(req.NewInputs, MarshalOptions{
		Label:        label + ".newInputs",
		KeepUnknowns: true,
	})
	if err != nil {
		return DiffResult{}, err
	}

	resp, err := p.clientRaw.DiffConfig(p.requestContext(), &pulumirpc.DiffRequest{
		Urn:           string(req.URN),
		Name:          req.URN.Name(),
		Type:          req.URN.Type().String(),
		OldInputs:     mOldInputs,
		Olds:          mOldOutputs,
		News:          mNewInputs,
		IgnoreChanges: req.IgnoreChanges,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented || isDiffCheckConfigLogicallyUnimplemented(rpcError, req.URN.Type()) {
			logging.V(7).Infof("%s unimplemented rpc: returning DiffUnknown with no replaces", label)
			// In this case, the provider plugin did not implement this and we have to provide some answer:
			//
			// There are two interesting scenarios with the present gRPC interface:
			// 1. Configuration differences in which all properties are known
			// 2. Configuration differences in which some new property is unknown.
			//
			// In both cases, we return a diff result that indicates that the provider _should not_ be replaced.
			// Although this decision is not conservative--indeed, the conservative decision would be to always require
			// replacement of a provider if any input has changed--we believe that it results in the best possible user
			// experience for providers that do not implement DiffConfig functionality. If we took the conservative
			// route here, any change to a provider's configuration (no matter how inconsequential) would cause all of
			// its resources to be replaced. This is clearly a bad experience, and differs from how things worked prior
			// to first-class providers.
			return DiffResult{Changes: DiffUnknown, ReplaceKeys: nil}, nil
		}
		logging.V(8).Infof("%s provider received rpc error `%s`: `%s`", label, rpcError.Code(),
			rpcError.Message())
		// https://github.com/pulumi/pulumi/issues/14529: Old versions of kubernetes would error on this
		// call if "kubeconfig" was set to a file. This didn't cause issues later when the same config was
		// passed to Configure, and for many years silently "worked".
		// https://github.com/pulumi/pulumi/pull/14436 fixed this method to start returning errors which
		// exposed this issue with the kubernetes provider, new versions will be fixed to not error on
		// this (https://github.com/pulumi/pulumi-kubernetes/issues/2663) but so that the CLI continues to
		// work for old versions we have an explicit ignore for this one error here.
		if p.pkg == "kubernetes" &&
			strings.Contains(rpcError.Error(), "cannot unmarshal string into Go value of type struct") {
			logging.V(8).Infof("%s ignoring error from kubernetes provider", label)
			return DiffResult{Changes: DiffUnknown}, nil
		}

		return DiffResult{}, err
	}

	replaces := slice.Prealloc[resource.PropertyKey](len(resp.GetReplaces()))
	for _, replace := range resp.GetReplaces() {
		replaces = append(replaces, resource.PropertyKey(replace))
	}
	stables := slice.Prealloc[resource.PropertyKey](len(resp.GetStables()))
	for _, stable := range resp.GetStables() {
		stables = append(stables, resource.PropertyKey(stable))
	}
	diffs := slice.Prealloc[resource.PropertyKey](len(resp.GetDiffs()))
	for _, diff := range resp.GetDiffs() {
		diffs = append(diffs, resource.PropertyKey(diff))
	}

	changes := resp.GetChanges()
	deleteBeforeReplace := resp.GetDeleteBeforeReplace()
	logging.V(7).Infof("%s success: changes=%d #replaces=%v #stables=%v delbefrepl=%v, diffs=#%v",
		label, changes, replaces, stables, deleteBeforeReplace, diffs)

	return DiffResult{
		Changes:             DiffChanges(changes),
		ReplaceKeys:         replaces,
		StableKeys:          stables,
		ChangedKeys:         diffs,
		DetailedDiff:        decodeDetailedDiff(resp),
		DeleteBeforeReplace: deleteBeforeReplace,
	}, nil
}

// annotateSecrets copies the "secretness" from the ins to the outs. If there are values with the same keys for the
// outs and the ins, if they are both objects, they are transformed recursively. Otherwise, if the value in the ins
// contains a secret, the entire out value is marked as a secret.  This is very close to how we project secrets
// in the programming model, with one small difference, which is how we treat the case where both are objects. In the
// programming model, we would say the entire output object is a secret. Here, we actually recur in. We do this because
// we don't want a single secret value in a rich structure to taint the entire object. Doing so would mean things like
// the entire value in the deployment would be encrypted instead of a small chunk. It also means the entire property
// would be displayed as `[secret]` in the CLI instead of a small part.
//
// NOTE: This means that for an array, if any value in the input version is a secret, the entire output array is
// marked as a secret. This is actually a very nice result, because often arrays are treated like sets by providers
// and the order may not be preserved across an operation. This means we do end up encrypting the entire array
// but that's better than accidentally leaking a value which just moved to a different location.
func annotateSecrets(outs, ins resource.PropertyMap) {
	if outs == nil || ins == nil {
		return
	}

	for key, inValue := range ins {
		outValue, has := outs[key]
		if !has {
			continue
		}
		if outValue.IsObject() && inValue.IsObject() {
			annotateSecrets(outValue.ObjectValue(), inValue.ObjectValue())
		} else if !outValue.IsSecret() && inValue.ContainsSecrets() {
			outs[key] = resource.MakeSecret(outValue)
		}
	}
}

func removeSecrets(v resource.PropertyValue) interface{} {
	switch {
	case v.IsNull():
		return nil
	case v.IsBool():
		return v.BoolValue()
	case v.IsNumber():
		return v.NumberValue()
	case v.IsString():
		return v.StringValue()
	case v.IsArray():
		arr := []interface{}{}
		for _, v := range v.ArrayValue() {
			arr = append(arr, removeSecrets(v))
		}
		return arr
	case v.IsAsset():
		return v.AssetValue()
	case v.IsArchive():
		return v.ArchiveValue()
	case v.IsComputed():
		return v.Input()
	case v.IsOutput():
		return v.OutputValue()
	case v.IsSecret():
		return removeSecrets(v.SecretValue().Element)
	default:
		contract.Assertf(v.IsObject(), "v is not Object '%v' instead", v.TypeString())
		obj := map[string]interface{}{}
		for k, v := range v.ObjectValue() {
			obj[string(k)] = removeSecrets(v)
		}
		return obj
	}
}

func traverseProperty(element resource.PropertyValue, f func(resource.PropertyValue)) {
	f(element)
	if element.IsSecret() {
		traverseSecret(element.SecretValue(), f)
	} else if element.IsObject() {
		traverseMap(element.ObjectValue(), f)
	} else if element.IsArray() {
		traverseArray(element.ArrayValue(), f)
	}
}

func traverseArray(elements []resource.PropertyValue, f func(resource.PropertyValue)) {
	for _, element := range elements {
		traverseProperty(element, f)
	}
}

func traverseSecret(v *resource.Secret, f func(resource.PropertyValue)) {
	traverseProperty(v.Element, f)
}

func traverseMap(m resource.PropertyMap, f func(resource.PropertyValue)) {
	for _, value := range m {
		traverseProperty(value, f)
	}
}

// restoreElidedAssetContents is used to restore contents of assets inside resource property maps after
// we have skipped serializing contents of assets in order to avoid sending them over the wire to resource
// providers. Mainly used in `Read` operations after we receive the live inputs from the resource provider plugin.
// Those inputs may echo back the input assets and the engine writes them out to the state. We need to make sure that
// we don't write out empty assets to the state, so we restore the asset contents from the original inputs.
func restoreElidedAssetContents(original resource.PropertyMap, transformed resource.PropertyMap) {
	isEmptyAsset := func(v *asset.Asset) bool {
		return v.Text == "" && v.Path == "" && v.URI == ""
	}

	isEmptyArchive := func(v *archive.Archive) bool {
		return v.Path == "" && v.URI == "" && v.Assets == nil
	}

	originalAssets := map[string]*asset.Asset{}
	originalArchives := map[string]*archive.Archive{}

	traverseMap(original, func(value resource.PropertyValue) {
		if value.IsAsset() {
			originalAsset := value.AssetValue()
			originalAssets[originalAsset.Hash] = originalAsset
		}

		if value.IsArchive() {
			originalArchive := value.ArchiveValue()
			originalArchives[originalArchive.Hash] = originalArchive
		}
	})

	traverseMap(transformed, func(value resource.PropertyValue) {
		if value.IsAsset() {
			transformedAsset := value.AssetValue()
			originalAsset, has := originalAssets[transformedAsset.Hash]
			if has && isEmptyAsset(transformedAsset) {
				transformedAsset.Sig = originalAsset.Sig
				transformedAsset.Text = originalAsset.Text
				transformedAsset.Path = originalAsset.Path
				transformedAsset.URI = originalAsset.URI
			}
		}

		if value.IsArchive() {
			transformedArchive := value.ArchiveValue()
			originalArchive, has := originalArchives[transformedArchive.Hash]
			if has && isEmptyArchive(transformedArchive) {
				transformedArchive.Sig = originalArchive.Sig
				transformedArchive.URI = originalArchive.URI
				transformedArchive.Path = originalArchive.Path
				transformedArchive.Assets = originalArchive.Assets
			}
		}
	})
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *provider) Configure(ctx context.Context, req ConfigureRequest) (ConfigureResponse, error) {
	label := p.label() + ".Configure()"
	logging.V(7).Infof("%s executing (#vars=%d)", label, len(req.Inputs))

	// Convert the inputs to a config map. If any are unknown, do not configure the underlying plugin: instead, leave
	// the cfgknown bit unset and carry on.
	config := make(map[string]string)
	for k, v := range req.Inputs {
		if k == "version" {
			continue
		}

		if v.ContainsUnknowns() {
			if p.protocol == nil {
				p.protocol = &pluginProtocol{}
			}

			p.configSource.MustFulfill(pluginConfig{
				known: false,
			})
			return ConfigureResponse{}, nil
		}

		mapped := removeSecrets(v)
		if _, isString := mapped.(string); !isString {
			marshalled, err := json.Marshal(mapped)
			if err != nil {
				err := fmt.Errorf("marshaling configuration property '%v': %w", k, err)
				p.configSource.MustReject(err)
				return ConfigureResponse{}, err
			}
			mapped = string(marshalled)
		}

		// Pass the older spelling of a configuration key across the RPC interface, for now, to support
		// providers which are on the older plan.
		config[string(p.Pkg())+":config:"+string(k)] = mapped.(string)
	}

	minputs, err := MarshalProperties(req.Inputs, MarshalOptions{
		Label:         label + ".inputs",
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		err := fmt.Errorf("marshaling provider inputs: %w", err)
		p.configSource.MustReject(err)
		return ConfigureResponse{}, err
	}

	// Spawn the configure to happen in parallel.  This ensures that we remain responsive elsewhere that might
	// want to make forward progress, even as the configure call is happening.
	go func() {
		var urn, typ, id *string
		if req.URN != nil {
			urnVal := string(*req.URN)
			urn = &urnVal
		}
		if req.ID != nil {
			idVal := string(*req.ID)
			id = &idVal
		}
		if req.Type != nil {
			typVal := string(*req.Type)
			typ = &typVal
		}

		resp, err := p.clientRaw.Configure(p.requestContext(), &pulumirpc.ConfigureRequest{
			Urn:                    urn,
			Name:                   req.Name,
			Type:                   typ,
			Id:                     id,
			AcceptSecrets:          true,
			AcceptResources:        true,
			SendsOldInputs:         true,
			SendsOldInputsToDelete: true,
			Variables:              config,
			Args:                   minputs,
		})
		if err != nil {
			rpcError := rpcerror.Convert(err)
			logging.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
			err = createConfigureError(rpcError)
			p.configSource.MustReject(err)
			return
		}

		if p.protocol == nil {
			p.protocol = &pluginProtocol{
				acceptSecrets:                   resp.GetAcceptSecrets(),
				acceptResources:                 resp.GetAcceptResources(),
				supportsPreview:                 resp.GetSupportsPreview(),
				acceptOutputs:                   resp.GetAcceptOutputs(),
				supportsAutonamingConfiguration: resp.GetSupportsAutonamingConfiguration(),
			}
		}

		p.configSource.MustFulfill(pluginConfig{
			known: true,
		})
	}()

	return ConfigureResponse{}, nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *provider) Check(ctx context.Context, req CheckRequest) (CheckResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	label := fmt.Sprintf("%s.Check(%s)", p.label(), req.URN)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d)", label, len(req.Olds), len(req.News))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return CheckResponse{}, err
	}

	// If the configuration for this provider was not fully known--e.g. if we are doing a preview and some input
	// property was sourced from another resource's output properties--don't call into the underlying provider.
	if !pcfg.known {
		return CheckResponse{Properties: req.News}, nil
	}

	molds, err := MarshalProperties(req.Olds, MarshalOptions{
		Label:         label + ".olds",
		KeepUnknowns:  req.AllowUnknowns,
		KeepSecrets:   protocol.acceptSecrets,
		KeepResources: protocol.acceptResources,
	})
	if err != nil {
		return CheckResponse{}, err
	}
	mnews, err := MarshalProperties(req.News, MarshalOptions{
		Label:         label + ".news",
		KeepUnknowns:  req.AllowUnknowns,
		KeepSecrets:   protocol.acceptSecrets,
		KeepResources: protocol.acceptResources,
	})
	if err != nil {
		return CheckResponse{}, err
	}

	var autonaming *pulumirpc.CheckRequest_AutonamingOptions
	if req.Autonaming != nil {
		if protocol.supportsAutonamingConfiguration {
			autonaming = &pulumirpc.CheckRequest_AutonamingOptions{
				ProposedName: req.Autonaming.ProposedName,
				Mode:         pulumirpc.CheckRequest_AutonamingOptions_Mode(req.Autonaming.Mode),
			}
		} else if req.Autonaming.WarnIfNoSupport {
			p.ctx.Diag.Warningf(diag.Message(req.URN,
				"%s resource has a custom autonaming setting but the provider does not support "+
					"autonaming configuration, consider upgrading to a newer version"),
				req.URN)
		}
	}

	resp, err := client.Check(p.requestContext(), &pulumirpc.CheckRequest{
		Urn:        string(req.URN),
		Name:       req.URN.Name(),
		Type:       req.URN.Type().String(),
		Olds:       molds,
		News:       mnews,
		RandomSeed: req.RandomSeed,
		Autonaming: autonaming,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
		return CheckResponse{}, rpcError
	}

	// Unmarshal the provider inputs.
	var inputs resource.PropertyMap
	if ins := resp.GetInputs(); ins != nil {
		inputs, err = UnmarshalProperties(ins, MarshalOptions{
			Label:          label + ".inputs",
			KeepUnknowns:   req.AllowUnknowns,
			RejectUnknowns: !req.AllowUnknowns,
			KeepSecrets:    true,
			KeepResources:  true,
		})
		if err != nil {
			return CheckResponse{}, err
		}
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !protocol.acceptSecrets {
		annotateSecrets(inputs, req.News)
	}

	// And now any properties that failed verification.
	failures := slice.Prealloc[CheckFailure](len(resp.GetFailures()))
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	logging.V(7).Infof("%s success: inputs=#%d failures=#%d", label, len(inputs), len(failures))
	return CheckResponse{Properties: inputs, Failures: failures}, nil
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *provider) Diff(ctx context.Context, req DiffRequest) (DiffResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	contract.Assertf(req.URN != "", "Diff requires a URN")
	contract.Assertf(req.ID != "", "Diff requires an ID")
	contract.Assertf(req.OldInputs != nil, "Diff requires old input properties")
	contract.Assertf(req.NewInputs != nil, "Diff requires new input properties")
	contract.Assertf(req.OldOutputs != nil, "Diff requires old output properties")

	label := fmt.Sprintf("%s.Diff(%s,%s)", p.label(), req.URN, req.ID)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(req.OldInputs), len(req.OldOutputs), len(req.NewInputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return DiffResult{}, err
	}

	// If the configuration for this provider was not fully known--e.g. if we are doing a preview and some input
	// property was sourced from another resource's output properties--don't call into the underlying provider.
	// Instead, indicate that the diff is unavailable and write a message
	if !pcfg.known {
		logging.V(7).Infof("%s: cannot diff due to unknown config", label)
		const message = "The provider for this resource has inputs that are not known during preview.\n" +
			"This preview may not correctly represent the changes that will be applied during an update."
		return DiffResult{}, DiffUnavailable(message)
	}

	mOldInputs, err := MarshalProperties(req.OldInputs, MarshalOptions{
		Label:              label + ".oldInputs",
		ElideAssetContents: true,
		KeepUnknowns:       req.AllowUnknowns,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mOldOutputs, err := MarshalProperties(req.OldOutputs, MarshalOptions{
		Label:              label + ".oldOutputs",
		ElideAssetContents: true,
		KeepUnknowns:       req.AllowUnknowns,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mNewInputs, err := MarshalProperties(req.NewInputs, MarshalOptions{
		Label:              label + ".newInputs",
		ElideAssetContents: true,
		KeepUnknowns:       req.AllowUnknowns,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return DiffResult{}, err
	}

	resp, err := client.Diff(p.requestContext(), &pulumirpc.DiffRequest{
		Id:            string(req.ID),
		Urn:           string(req.URN),
		Name:          req.URN.Name(),
		Type:          req.URN.Type().String(),
		OldInputs:     mOldInputs,
		Olds:          mOldOutputs,
		News:          mNewInputs,
		IgnoreChanges: req.IgnoreChanges,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return DiffResult{}, rpcError
	}

	// nil is semantically important to a lot of the pulumi system so we only pre-allocate if we have non-zero length.
	replaces := slice.Prealloc[resource.PropertyKey](len(resp.GetReplaces()))
	for _, replace := range resp.GetReplaces() {
		replaces = append(replaces, resource.PropertyKey(replace))
	}
	stables := slice.Prealloc[resource.PropertyKey](len(resp.GetStables()))
	for _, stable := range resp.GetStables() {
		stables = append(stables, resource.PropertyKey(stable))
	}
	diffs := slice.Prealloc[resource.PropertyKey](len(resp.GetDiffs()))
	for _, diff := range resp.GetDiffs() {
		diffs = append(diffs, resource.PropertyKey(diff))
	}

	changes := resp.GetChanges()
	deleteBeforeReplace := resp.GetDeleteBeforeReplace()
	logging.V(7).Infof("%s success: changes=%d #replaces=%v #stables=%v delbefrepl=%v, diffs=#%v, detaileddiff=%v",
		label, changes, replaces, stables, deleteBeforeReplace, diffs, resp.GetDetailedDiff())

	return DiffResult{
		Changes:             DiffChanges(changes),
		ReplaceKeys:         replaces,
		StableKeys:          stables,
		ChangedKeys:         diffs,
		DetailedDiff:        decodeDetailedDiff(resp),
		DeleteBeforeReplace: deleteBeforeReplace,
	}, nil
}

// Create allocates a new instance of the provided resource and assigns its unique resource.ID and outputs afterwards.
func (p *provider) Create(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	contract.Assertf(req.URN != "", "Create requires a URN")
	contract.Assertf(req.Properties != nil, "Create requires properties")

	label := fmt.Sprintf("%s.Create(%s)", p.label(), req.URN)
	logging.V(7).Infof("%s executing (#props=%v)", label, len(req.Properties))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return CreateResponse{}, err
	}

	// If this is a preview and the plugin does not support provider previews, or if the configuration for the provider
	// is not fully known, hand back an empty property map. This will force the language SDK will to treat all properties
	// as unknown, which is conservatively correct.
	//
	// If the provider does not support previews, return the inputs as the state. Note that this can cause problems for
	// the language SDKs if there are input and state properties that share a name but expect differently-shaped values.
	if req.Preview {
		// TODO: it would be great to swap the order of these if statements. This would prevent a behavioral change for
		// providers that do not support provider previews, which will always return the inputs as state regardless of
		// whether or not the config is known. Unfortunately, we can't, since the `supportsPreview` bit depends on the
		// result of `Configure`, which we won't call if the `cfgknown` is false. It may be worth fixing this catch-22
		// by extending the provider gRPC interface with a `SupportsFeature` API similar to the language monitor.
		if !pcfg.known {
			if p.legacyPreview {
				return CreateResponse{Properties: req.Properties}, nil
			}
			return CreateResponse{}, nil
		}
		if !protocol.supportsPreview || p.disableProviderPreview {
			return CreateResponse{Properties: req.Properties}, nil
		}
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assertf(pcfg.known, "Create cannot be called if the configuration is unknown")

	mprops, err := MarshalProperties(req.Properties, MarshalOptions{
		Label:         label + ".inputs",
		KeepUnknowns:  req.Preview,
		KeepSecrets:   protocol.acceptSecrets,
		KeepResources: protocol.acceptResources,
	})
	if err != nil {
		return CreateResponse{}, err
	}

	var id resource.ID
	var liveObject *structpb.Struct
	var resourceError error
	resourceStatus := resource.StatusOK
	resp, err := client.Create(p.requestContext(), &pulumirpc.CreateRequest{
		Urn:        string(req.URN),
		Name:       req.URN.Name(),
		Type:       req.URN.Type().String(),
		Properties: mprops,
		Timeout:    req.Timeout,
		Preview:    req.Preview,
	})
	if err != nil {
		resourceStatus, id, liveObject, _, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, resourceError)

		if resourceStatus != resource.StatusPartialFailure {
			return CreateResponse{}, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		id = resource.ID(resp.GetId())
		liveObject = resp.GetProperties()
	}

	if id == "" && !req.Preview {
		return CreateResponse{Status: resource.StatusUnknown},
			fmt.Errorf("plugin for package '%v' returned empty resource.ID from create '%v'", p.pkg, req.URN)
	}

	outs, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label:          label + ".outputs",
		RejectUnknowns: !req.Preview,
		KeepUnknowns:   req.Preview,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return CreateResponse{Status: resourceStatus}, err
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !protocol.acceptSecrets {
		annotateSecrets(outs, req.Properties)
	}

	logging.V(7).Infof("%s success: id=%s; #outs=%d", label, id, len(outs))
	return CreateResponse{
		ID:         id,
		Properties: outs,
		Status:     resourceStatus,
	}, resourceError
}

// read the current live state associated with a resource.  enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource id, but may also include some properties.
func (p *provider) Read(ctx context.Context, req ReadRequest) (ReadResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	contract.Assertf(req.URN != "", "Read URN was empty")
	contract.Assertf(req.ID != "", "Read ID was empty")

	label := fmt.Sprintf("%s.Read(%s,%s)", p.label(), req.ID, req.URN)
	logging.V(7).Infof("%s executing (#inputs=%v, #state=%v)", label, len(req.Inputs), len(req.State))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return ReadResponse{Status: resource.StatusUnknown}, err
	}

	// If the provider is not fully configured, return an empty bag.
	if !pcfg.known {
		return ReadResponse{ReadResult{
			Outputs: resource.PropertyMap{},
			Inputs:  resource.PropertyMap{},
		}, resource.StatusUnknown}, nil
	}

	// Marshal the resource inputs and state so we can perform the RPC.
	var minputs *structpb.Struct
	if req.Inputs != nil {
		m, err := MarshalProperties(req.Inputs, MarshalOptions{
			Label:              label,
			ElideAssetContents: true,
			KeepSecrets:        protocol.acceptSecrets,
			KeepResources:      protocol.acceptResources,
		})
		if err != nil {
			return ReadResponse{Status: resource.StatusUnknown}, err
		}
		minputs = m
	}
	mstate, err := MarshalProperties(req.State, MarshalOptions{
		Label:              label,
		ElideAssetContents: true,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return ReadResponse{Status: resource.StatusUnknown}, err
	}

	// Now issue the read request over RPC, blocking until it finished.
	var readID resource.ID
	var liveObject *structpb.Struct
	var liveInputs *structpb.Struct
	var resourceError error
	resourceStatus := resource.StatusOK
	resp, err := client.Read(p.requestContext(), &pulumirpc.ReadRequest{
		Id:         string(req.ID),
		Urn:        string(req.URN),
		Name:       req.URN.Name(),
		Type:       req.URN.Type().String(),
		Properties: mstate,
		Inputs:     minputs,
	})
	if err != nil {
		resourceStatus, readID, liveObject, liveInputs, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, err)

		if resourceStatus != resource.StatusPartialFailure {
			return ReadResponse{Status: resourceStatus}, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		readID = resource.ID(resp.GetId())
		liveObject = resp.GetProperties()
		liveInputs = resp.GetInputs()
	}

	// If the resource was missing, simply return a nil property map.
	if string(readID) == "" {
		return ReadResponse{Status: resourceStatus}, nil
	}

	// Finally, unmarshal the resulting state properties and return them.
	newState, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label:          label + ".outputs",
		RejectUnknowns: true,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return ReadResponse{Status: resourceStatus}, err
	}

	var newInputs resource.PropertyMap
	if liveInputs != nil {
		newInputs, err = UnmarshalProperties(liveInputs, MarshalOptions{
			Label:          label + ".inputs",
			RejectUnknowns: true,
			KeepSecrets:    true,
			KeepResources:  true,
		})
		if err != nil {
			return ReadResponse{Status: resourceStatus}, err
		}
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !protocol.acceptSecrets {
		annotateSecrets(newInputs, req.Inputs)
		annotateSecrets(newState, req.State)
	}

	// make sure any echoed properties restore their original asset contents if they have not changed
	restoreElidedAssetContents(req.Inputs, newInputs)
	restoreElidedAssetContents(req.Inputs, newState)

	logging.V(7).Infof("%s success; #outs=%d, #inputs=%d", label, len(newState), len(newInputs))
	return ReadResponse{ReadResult{
		ID:      readID,
		Outputs: newState,
		Inputs:  newInputs,
	}, resourceStatus}, resourceError
}

// Update updates an existing resource with new values.
func (p *provider) Update(ctx context.Context, req UpdateRequest) (UpdateResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	contract.Assertf(req.URN != "", "Update requires a URN")
	contract.Assertf(req.ID != "", "Update requires an ID")
	contract.Assertf(req.OldInputs != nil, "Update requires old inputs")
	contract.Assertf(req.OldOutputs != nil, "Update requires old outputs")
	contract.Assertf(req.NewInputs != nil, "Update requires new properties")

	label := fmt.Sprintf("%s.Update(%s,%s)", p.label(), req.ID, req.URN)
	logging.V(7).Infof("%s executing (#oldInputs=%v,#oldOutputs=%v,#newInputs=%v)",
		label, len(req.OldInputs), len(req.OldOutputs), len(req.NewInputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, err
	}

	// If this is a preview and the plugin does not support provider previews, or if the configuration for the provider
	// is not fully known, hand back an empty property map. This will force the language SDK to treat all properties
	// as unknown, which is conservatively correct.
	//
	// If the provider does not support previews, return the inputs as the state. Note that this can cause problems for
	// the language SDKs if there are input and state properties that share a name but expect differently-shaped values.
	if req.Preview {
		// TODO: it would be great to swap the order of these if statements. This would prevent a behavioral change for
		// providers that do not support provider previews, which will always return the inputs as state regardless of
		// whether or not the config is known. Unfortunately, we can't, since the `supportsPreview` bit depends on the
		// result of `Configure`, which we won't call if the `cfgknown` is false. It may be worth fixing this catch-22
		// by extending the provider gRPC interface with a `SupportsFeature` API similar to the language monitor.
		if !pcfg.known {
			if p.legacyPreview {
				return UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
			}
			return UpdateResponse{Properties: resource.PropertyMap{}, Status: resource.StatusOK}, nil
		}
		if !protocol.supportsPreview || p.disableProviderPreview {
			return UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
		}
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assertf(pcfg.known, "Update cannot be called if the configuration is unknown")

	mOldInputs, err := MarshalProperties(req.OldInputs, MarshalOptions{
		Label:              label + ".oldInputs",
		ElideAssetContents: true,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return UpdateResponse{Status: resource.StatusOK}, err
	}
	mOldOutputs, err := MarshalProperties(req.OldOutputs, MarshalOptions{
		Label:              label + ".oldOutputs",
		ElideAssetContents: true,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return UpdateResponse{Status: resource.StatusOK}, err
	}
	mNewInputs, err := MarshalProperties(req.NewInputs, MarshalOptions{
		Label:         label + ".newInputs",
		KeepUnknowns:  req.Preview,
		KeepSecrets:   protocol.acceptSecrets,
		KeepResources: protocol.acceptResources,
	})
	if err != nil {
		return UpdateResponse{Status: resource.StatusOK}, err
	}

	var liveObject *structpb.Struct
	var resourceError error
	resourceStatus := resource.StatusOK
	resp, err := client.Update(p.requestContext(), &pulumirpc.UpdateRequest{
		Id:            string(req.ID),
		Urn:           string(req.URN),
		Name:          req.URN.Name(),
		Type:          req.URN.Type().String(),
		Olds:          mOldOutputs,
		News:          mNewInputs,
		Timeout:       req.Timeout,
		IgnoreChanges: req.IgnoreChanges,
		Preview:       req.Preview,
		OldInputs:     mOldInputs,
	})
	if err != nil {
		resourceStatus, _, liveObject, _, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, resourceError)

		if resourceStatus != resource.StatusPartialFailure {
			return UpdateResponse{Status: resourceStatus}, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		liveObject = resp.GetProperties()
	}

	outs, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label:          label + ".outputs",
		RejectUnknowns: !req.Preview,
		KeepUnknowns:   req.Preview,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return UpdateResponse{Status: resourceStatus}, err
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !protocol.acceptSecrets {
		annotateSecrets(outs, req.NewInputs)
	}
	logging.V(7).Infof("%s success; #outs=%d", label, len(outs))

	return UpdateResponse{Properties: outs, Status: resourceStatus}, resourceError
}

// Delete tears down an existing resource.
func (p *provider) Delete(ctx context.Context, req DeleteRequest) (DeleteResponse, error) {
	// We either leave Name&Type empty and fill them in from the URN, or they must match the URN.
	contract.Assertf(req.Name == "" || req.Name == req.URN.Name(),
		"req.Name (%s) != req.URN.Name() (%s)", req.Name, req.URN.Name())
	contract.Assertf(req.Type == "" || req.Type == req.URN.Type(),
		"req.Type (%s) != req.URN.Type() (%s)", req.Type, req.URN.Type())

	contract.Assertf(req.URN != "", "Delete requires a URN")
	contract.Assertf(req.ID != "", "Delete requires an ID")

	label := fmt.Sprintf("%s.Delete(%s,%s)", p.label(), req.URN, req.ID)
	logging.V(7).Infof("%s executing (#inputs=%d, #outputs=%d)", label, len(req.Inputs), len(req.Outputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return DeleteResponse{}, err
	}

	// We should never call delete at preview time, so we should never see unknowns here
	contract.Assertf(pcfg.known, "Delete cannot be called if the configuration is unknown")

	minputs, err := MarshalProperties(req.Inputs, MarshalOptions{
		Label:              label,
		ElideAssetContents: true,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return DeleteResponse{}, err
	}

	moutputs, err := MarshalProperties(req.Outputs, MarshalOptions{
		Label:              label,
		ElideAssetContents: true,
		KeepSecrets:        protocol.acceptSecrets,
		KeepResources:      protocol.acceptResources,
	})
	if err != nil {
		return DeleteResponse{}, err
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assertf(pcfg.known, "Delete cannot be called if the configuration is unknown")

	if _, err := client.Delete(p.requestContext(), &pulumirpc.DeleteRequest{
		Id:         string(req.ID),
		Urn:        string(req.URN),
		Name:       req.URN.Name(),
		Type:       req.URN.Type().String(),
		Properties: moutputs,
		Timeout:    req.Timeout,
		OldInputs:  minputs,
	}); err != nil {
		resourceStatus, rpcErr := resourceStateAndError(err)
		logging.V(7).Infof("%s failed: %v", label, rpcErr)
		return DeleteResponse{Status: resourceStatus}, rpcErr
	}

	logging.V(7).Infof("%s success", label)
	return DeleteResponse{Status: resource.StatusOK}, err
}

// Construct creates a new component resource from the given type, name, parent, options, and inputs, and returns
// its URN and outputs.
func (p *provider) Construct(ctx context.Context, req ConstructRequest) (ConstructResponse, error) {
	contract.Assertf(req.Type != "", "Construct requires a type")
	contract.Assertf(req.Name != "", "Construct requires a name")
	contract.Assertf(req.Inputs != nil, "Construct requires input properties")

	label := fmt.Sprintf("%s.Construct(%s, %s, %s)", p.label(), req.Type, req.Name, req.Parent)
	logging.V(7).Infof("%s executing (#inputs=%v)", label, len(req.Inputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(ctx)
	if err != nil {
		return ConstructResult{}, err
	}

	// If the provider is not fully configured.  Pretend we are the provider and call RegisterResource to get the URN.
	if !pcfg.known {
		// Connect to the resource monitor and create an appropriate client.
		conn, err := grpc.NewClient(
			req.Info.MonitorAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return ConstructResult{}, fmt.Errorf("could not connect to resource monitor: %w", err)
		}
		resmon := pulumirpc.NewResourceMonitorClient(conn)
		resp, err := resmon.RegisterResource(ctx, &pulumirpc.RegisterResourceRequest{
			Type:   string(req.Type),
			Name:   req.Name,
			Parent: string(req.Parent),
		})
		if err != nil {
			rpcError := rpcerror.Convert(err)
			logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
			return ConstructResult{}, rpcError
		}
		return ConstructResult{
			URN: resource.URN(resp.GetUrn()),
		}, nil
	}

	if !protocol.acceptSecrets {
		return ConstructResult{}, errors.New("plugins that can construct components must support secrets")
	}

	// Marshal the input properties.
	minputs, err := MarshalProperties(req.Inputs, MarshalOptions{
		Label:         label + ".inputs",
		KeepUnknowns:  true,
		KeepSecrets:   protocol.acceptSecrets,
		KeepResources: protocol.acceptResources,
		// To initially scope the use of this new feature, we only keep output values for
		// Construct and Call (when the client accepts them).
		KeepOutputValues: protocol.acceptOutputs,
	})
	if err != nil {
		return ConstructResult{}, err
	}

	// Marshal the aliases.
	aliasURNs := make([]string, len(req.Options.Aliases))
	for i, alias := range req.Options.Aliases {
		aliasURNs[i] = string(alias.URN)
	}

	// Marshal the dependencies.
	dependencies := make([]string, len(req.Options.Dependencies))
	for i, dep := range req.Options.Dependencies {
		dependencies[i] = string(dep)
	}

	// Marshal the property dependencies.
	inputDependencies := map[string]*pulumirpc.ConstructRequest_PropertyDependencies{}
	for name, dependencies := range req.Options.PropertyDependencies {
		urns := make([]string, len(dependencies))
		for i, urn := range dependencies {
			urns[i] = string(urn)
		}
		inputDependencies[string(name)] = &pulumirpc.ConstructRequest_PropertyDependencies{Urns: urns}
	}

	// Marshal the config.
	config := map[string]string{}
	for k, v := range req.Info.Config {
		config[k.String()] = v
	}
	configSecretKeys := []string{}
	for _, k := range req.Info.ConfigSecretKeys {
		configSecretKeys = append(configSecretKeys, k.String())
	}

	rpcReq := &pulumirpc.ConstructRequest{
		Project:                 req.Info.Project,
		Stack:                   req.Info.Stack,
		Config:                  config,
		ConfigSecretKeys:        configSecretKeys,
		DryRun:                  req.Info.DryRun,
		Parallel:                req.Info.Parallel,
		MonitorEndpoint:         req.Info.MonitorAddress,
		Type:                    string(req.Type),
		Name:                    req.Name,
		Parent:                  string(req.Parent),
		Inputs:                  minputs,
		Protect:                 req.Options.Protect,
		Providers:               req.Options.Providers,
		InputDependencies:       inputDependencies,
		Aliases:                 aliasURNs,
		Dependencies:            dependencies,
		AdditionalSecretOutputs: req.Options.AdditionalSecretOutputs,
		DeletedWith:             string(req.Options.DeletedWith),
		DeleteBeforeReplace:     req.Options.DeleteBeforeReplace,
		IgnoreChanges:           req.Options.IgnoreChanges,
		ReplaceOnChanges:        req.Options.ReplaceOnChanges,
		RetainOnDelete:          req.Options.RetainOnDelete,
		AcceptsOutputValues:     true,
	}
	if ct := req.Options.CustomTimeouts; ct != nil {
		rpcReq.CustomTimeouts = &pulumirpc.ConstructRequest_CustomTimeouts{
			Create: ct.Create,
			Update: ct.Update,
			Delete: ct.Delete,
		}
	}

	resp, err := client.Construct(p.requestContext(), rpcReq)
	if err != nil {
		return ConstructResult{}, err
	}

	outputs, err := UnmarshalProperties(resp.GetState(), MarshalOptions{
		Label:            label + ".outputs",
		KeepUnknowns:     req.Info.DryRun,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
	if err != nil {
		return ConstructResult{}, err
	}

	outputDependencies := map[resource.PropertyKey][]resource.URN{}
	for k, rpcDeps := range resp.GetStateDependencies() {
		urns := make([]resource.URN, len(rpcDeps.Urns))
		for i, d := range rpcDeps.Urns {
			urns[i] = resource.URN(d)
		}
		outputDependencies[resource.PropertyKey(k)] = urns
	}

	logging.V(7).Infof("%s success: #outputs=%d", label, len(outputs))
	return ConstructResponse{
		URN:                resource.URN(resp.GetUrn()),
		Outputs:            outputs,
		OutputDependencies: outputDependencies,
	}, nil
}

// Invoke dynamically executes a built-in function in the provider.
func (p *provider) Invoke(ctx context.Context, req InvokeRequest) (InvokeResponse, error) {
	contract.Assertf(req.Tok != "", "Invoke requires a token")

	label := fmt.Sprintf("%s.Invoke(%s)", p.label(), req.Tok)
	logging.V(7).Infof("%s executing (#args=%d)", label, len(req.Args))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(ctx)
	if err != nil {
		return InvokeResponse{}, err
	}

	// If the provider is not fully configured, return an empty property map.
	if !pcfg.known {
		return InvokeResponse{Properties: resource.PropertyMap{}}, nil
	}

	margs, err := MarshalProperties(req.Args, MarshalOptions{
		Label:         label + ".args",
		KeepSecrets:   protocol.acceptSecrets,
		KeepResources: protocol.acceptResources,
	})
	if err != nil {
		return InvokeResponse{}, err
	}

	resp, err := client.Invoke(p.requestContext(), &pulumirpc.InvokeRequest{
		Tok:  string(req.Tok),
		Args: margs,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return InvokeResponse{}, rpcError
	}

	// Unmarshal any return values.
	ret, err := UnmarshalProperties(resp.GetReturn(), MarshalOptions{
		Label:          label + ".returns",
		RejectUnknowns: true,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return InvokeResponse{}, err
	}

	// And now any properties that failed verification.
	failures := slice.Prealloc[CheckFailure](len(resp.GetFailures()))
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	logging.V(7).Infof("%s success (#ret=%d,#failures=%d) success", label, len(ret), len(failures))
	return InvokeResponse{
		Properties: ret,
		Failures:   failures,
	}, nil
}

// StreamInvoke dynamically executes a built-in function in the provider, which returns a stream of
// responses.
func (p *provider) StreamInvoke(ctx context.Context, req StreamInvokeRequest) (StreamInvokeResponse, error) {
	contract.Assertf(req.Tok != "", "StreamInvoke requires a token")

	label := fmt.Sprintf("%s.StreamInvoke(%s)", p.label(), req.Tok)
	logging.V(7).Infof("%s executing (#args=%d)", label, len(req.Args))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return StreamInvokeResponse{}, err
	}

	// If the provider is not fully configured, return an empty property map.
	if !pcfg.known {
		return StreamInvokeResponse{}, req.OnNext(resource.PropertyMap{})
	}

	margs, err := MarshalProperties(req.Args, MarshalOptions{
		Label:         label + ".args",
		KeepSecrets:   protocol.acceptSecrets,
		KeepResources: protocol.acceptResources,
	})
	if err != nil {
		return StreamInvokeResponse{}, err
	}

	streamClient, err := client.StreamInvoke(p.requestContext(), &pulumirpc.InvokeRequest{
		Tok:  string(req.Tok),
		Args: margs,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return StreamInvokeResponse{}, rpcError
	}

	for {
		in, err := streamClient.Recv()
		if err == io.EOF {
			return StreamInvokeResponse{}, nil
		}
		if err != nil {
			return StreamInvokeResponse{}, err
		}

		// Unmarshal response.
		ret, err := UnmarshalProperties(in.GetReturn(), MarshalOptions{
			Label:          label + ".returns",
			RejectUnknowns: true,
			KeepSecrets:    true,
			KeepResources:  true,
		})
		if err != nil {
			return StreamInvokeResponse{}, err
		}

		// Check properties that failed verification.
		var failures []CheckFailure
		for _, failure := range in.GetFailures() {
			failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
		}

		if len(failures) > 0 {
			return StreamInvokeResponse{Failures: failures}, nil
		}

		// Send stream message back to whoever is consuming the stream.
		if err := req.OnNext(ret); err != nil {
			return StreamInvokeResponse{}, err
		}
	}
}

// Call dynamically executes a method in the provider associated with a component resource.
func (p *provider) Call(_ context.Context, req CallRequest) (CallResponse, error) {
	contract.Assertf(req.Tok != "", "Call requires a token")

	label := fmt.Sprintf("%s.Call(%s)", p.label(), req.Tok)
	logging.V(7).Infof("%s executing (#args=%d)", label, len(req.Args))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	protocol, pcfg, err := p.getPluginConfig(context.Background())
	if err != nil {
		return CallResult{}, err
	}

	// If the provider is not fully configured, return an empty property map.
	if !pcfg.known {
		return CallResult{}, nil
	}

	margs, err := MarshalProperties(req.Args, MarshalOptions{
		Label:         label + ".args",
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
		// To initially scope the use of this new feature, we only keep output values for
		// Construct and Call (when the client accepts them).
		KeepOutputValues: protocol.acceptOutputs,
	})
	if err != nil {
		return CallResult{}, err
	}

	// Marshal the arg dependencies.
	argDependencies := map[string]*pulumirpc.CallRequest_ArgumentDependencies{}
	for name, dependencies := range req.Options.ArgDependencies {
		urns := make([]string, len(dependencies))
		for i, urn := range dependencies {
			urns[i] = string(urn)
		}
		argDependencies[string(name)] = &pulumirpc.CallRequest_ArgumentDependencies{Urns: urns}
	}

	// Marshal the config.
	config := map[string]string{}
	for k, v := range req.Info.Config {
		config[k.String()] = v
	}

	resp, err := client.Call(p.requestContext(), &pulumirpc.CallRequest{
		Tok:                 string(req.Tok),
		Args:                margs,
		ArgDependencies:     argDependencies,
		Project:             req.Info.Project,
		Stack:               req.Info.Stack,
		Config:              config,
		DryRun:              req.Info.DryRun,
		Parallel:            req.Info.Parallel,
		MonitorEndpoint:     req.Info.MonitorAddress,
		AcceptsOutputValues: true,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return CallResult{}, rpcError
	}

	// Unmarshal any return values.
	ret, err := UnmarshalProperties(resp.GetReturn(), MarshalOptions{
		Label:            label + ".returns",
		KeepUnknowns:     req.Info.DryRun,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
	if err != nil {
		return CallResult{}, err
	}

	returnDependencies := map[resource.PropertyKey][]resource.URN{}
	for k, rpcDeps := range resp.GetReturnDependencies() {
		urns := make([]resource.URN, len(rpcDeps.Urns))
		for i, d := range rpcDeps.Urns {
			urns[i] = resource.URN(d)
		}
		returnDependencies[resource.PropertyKey(k)] = urns
	}

	// And now any properties that failed verification.
	failures := slice.Prealloc[CheckFailure](len(resp.GetFailures()))
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	logging.V(7).Infof("%s success (#ret=%d,#failures=%d) success", label, len(ret), len(failures))
	return CallResult{Return: ret, ReturnDependencies: returnDependencies, Failures: failures}, nil
}

// GetPluginInfo returns this plugin's information.
func (p *provider) GetPluginInfo(ctx context.Context) (workspace.PluginInfo, error) {
	label := p.label() + ".GetPluginInfo()"
	logging.V(7).Infof("%s executing", label)

	// Calling GetPluginInfo happens immediately after loading, and does not require configuration to proceed.
	// Thus, we access the clientRaw property, rather than calling getClient.
	resp, err := p.clientRaw.GetPluginInfo(p.requestContext(), &emptypb.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
		return workspace.PluginInfo{}, rpcError
	}

	var version *semver.Version
	if v := resp.Version; v != "" {
		sv, err := semver.ParseTolerant(v)
		if err != nil {
			return workspace.PluginInfo{}, err
		}
		version = &sv
	}

	path := ""
	if p.plug != nil {
		path = p.plug.Bin
	}

	logging.V(7).Infof("%s success (#version=%v) success", label, version)
	return workspace.PluginInfo{
		Name:    string(p.pkg),
		Path:    path,
		Kind:    apitype.ResourcePlugin,
		Version: version,
	}, nil
}

// Attach attaches this plugin to the engine
func (p *provider) Attach(address string) error {
	label := p.label() + ".Attach()"
	logging.V(7).Infof("%s executing", label)

	// Calling Attach happens immediately after loading, and does not require configuration to proceed.
	// Thus, we access the clientRaw property, rather than calling getClient.
	_, err := p.clientRaw.Attach(p.requestContext(), &pulumirpc.PluginAttach{Address: address})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
		return rpcError
	}

	return nil
}

func (p *provider) SignalCancellation(ctx context.Context) error {
	_, err := p.clientRaw.Cancel(p.requestContext(), &emptypb.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(8).Infof("provider received rpc error `%s`: `%s`", rpcError.Code(),
			rpcError.Message())
		if rpcError.Code() == codes.Unimplemented {
			// For backwards compatibility, do nothing if it's not implemented.
			return nil
		}
	}

	return err
}

// Close tears down the underlying plugin RPC connection and process.
func (p *provider) Close() error {
	if p.plug == nil {
		return nil
	}
	return p.plug.Close()
}

// createConfigureError creates a nice error message from an RPC error that
// originated from `Configure`.
//
// If we requested that a resource configure itself but omitted required configuration
// variables, resource providers will respond with a list of missing variables and their descriptions.
// If that is what occurred, we'll use that information here to construct a nice error message.
func createConfigureError(rpcerr *rpcerror.Error) error {
	var err error
	for _, detail := range rpcerr.Details() {
		if missingKeys, ok := detail.(*pulumirpc.ConfigureErrorMissingKeys); ok {
			for _, missingKey := range missingKeys.MissingKeys {
				singleError := fmt.Errorf("missing required configuration key \"%s\": %s\n"+
					"Set a value using the command `pulumi config set %s <value>`.",
					missingKey.Name, missingKey.Description, missingKey.Name)
				err = multierror.Append(err, singleError)
			}
		}
	}

	if err != nil {
		return err
	}

	return rpcerr
}

// resourceStateAndError interprets an error obtained from a gRPC endpoint.
//
// gRPC gives us a `status.Status` structure as an `error` whenever our
// gRPC servers serve up an error. Each `status.Status` contains a code
// and a message. Based on the error code given to us, we can understand
// the state of our system and if our resource status is truly unknown.
//
// In general, our resource state is only really unknown if the server
// had an internal error, in which case it will serve one of `codes.Internal`,
// `codes.DataLoss`, or `codes.Unknown` to us.
func resourceStateAndError(err error) (resource.Status, *rpcerror.Error) {
	rpcError := rpcerror.Convert(err)
	logging.V(8).Infof("provider received rpc error `%s`: `%s`", rpcError.Code(), rpcError.Message())
	//nolint:exhaustive // We want to handle only some error codes specially
	switch rpcError.Code() {
	case codes.Internal, codes.DataLoss, codes.Unknown:
		logging.V(8).Infof("rpc error kind `%s` may not be recoverable", rpcError.Code())
		return resource.StatusUnknown, rpcError
	}

	logging.V(8).Infof("rpc error kind `%s` is well-understood and recoverable", rpcError.Code())
	return resource.StatusOK, rpcError
}

// parseError parses a gRPC error into a set of values that represent the state of a resource. They
// are: (1) the `resourceStatus`, indicating the last known state (e.g., `StatusOK`, representing
// success, `StatusUnknown`, representing internal failure); (2) the `*rpcerror.Error`, our internal
// representation for RPC errors; and optionally (3) `liveObject`, containing the last known live
// version of the object that has successfully created but failed to initialize (e.g., because the
// object was created, but app code is continually crashing and the resource never achieves
// liveness).
func parseError(err error) (
	resourceStatus resource.Status, id resource.ID, liveInputs, liveObject *structpb.Struct, resourceErr error,
) {
	var responseErr *rpcerror.Error
	resourceStatus, responseErr = resourceStateAndError(err)
	contract.Assertf(responseErr != nil, "resourceStateAndError must never return a nil error")

	// If resource was successfully created but failed to initialize, the error will be packed
	// with the live properties of the object.
	resourceErr = responseErr
	for _, detail := range responseErr.Details() {
		if initErr, ok := detail.(*pulumirpc.ErrorResourceInitFailed); ok {
			id = resource.ID(initErr.GetId())
			liveObject = initErr.GetProperties()
			liveInputs = initErr.GetInputs()
			resourceStatus = resource.StatusPartialFailure
			resourceErr = &InitError{Reasons: initErr.Reasons}
			break
		}
	}

	return resourceStatus, id, liveObject, liveInputs, resourceErr
}

// InitError represents a failure to initialize a resource, i.e., the resource has been successfully
// created, but it has failed to initialize.
type InitError struct {
	Reasons []string
}

var _ error = (*InitError)(nil)

func (ie *InitError) Error() string {
	var err error
	for _, reason := range ie.Reasons {
		err = multierror.Append(err, errors.New(reason))
	}
	if err == nil {
		return "resource init failed"
	}
	return err.Error()
}

func decorateSpanWithType(span opentracing.Span, urn string) {
	if urn := resource.URN(urn); urn.IsValid() {
		span.SetTag("pulumi-decorator", urn.Type())
	}
}

func decorateProviderSpans(span opentracing.Span, method string, req, resp interface{}, grpcError error) {
	if req == nil {
		return
	}

	switch method {
	case "/pulumirpc.ResourceProvider/Check", "/pulumirpc.ResourceProvider/CheckConfig":
		decorateSpanWithType(span, req.(*pulumirpc.CheckRequest).Urn)
	case "/pulumirpc.ResourceProvider/Diff", "/pulumirpc.ResourceProvider/DiffConfig":
		decorateSpanWithType(span, req.(*pulumirpc.DiffRequest).Urn)
	case "/pulumirpc.ResourceProvider/Create":
		decorateSpanWithType(span, req.(*pulumirpc.CreateRequest).Urn)
	case "/pulumirpc.ResourceProvider/Update":
		decorateSpanWithType(span, req.(*pulumirpc.UpdateRequest).Urn)
	case "/pulumirpc.ResourceProvider/Delete":
		decorateSpanWithType(span, req.(*pulumirpc.DeleteRequest).Urn)
	case "/pulumirpc.ResourceProvider/Invoke":
		span.SetTag("pulumi-decorator", req.(*pulumirpc.InvokeRequest).Tok)
	}
}

// GetMapping fetches the conversion mapping (if any) for this resource provider.
func (p *provider) GetMapping(ctx context.Context, req GetMappingRequest) (GetMappingResponse, error) {
	label := p.label() + ".GetMapping"
	logging.V(7).Infof("%s executing: key=%s, provider=%s", label, req.Key, req.Provider)

	resp, err := p.clientRaw.GetMapping(p.requestContext(), &pulumirpc.GetMappingRequest{
		Key:      req.Key,
		Provider: req.Provider,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented {
			// For backwards compatibility, just return nothing as if the provider didn't have a mapping for
			// the given key
			logging.V(7).Infof("%s unimplemented", label)
			return GetMappingResponse{}, nil
		}
		logging.V(7).Infof("%s failed: %v", label, rpcError)
		return GetMappingResponse{}, err
	}

	logging.V(7).Infof("%s success: data=#%d provider=%s", label, len(resp.Data), resp.Provider)
	return GetMappingResponse{
		Data:     resp.Data,
		Provider: resp.Provider,
	}, nil
}

func (p *provider) GetMappings(ctx context.Context, req GetMappingsRequest) (GetMappingsResponse, error) {
	label := p.label() + ".GetMappings"
	logging.V(7).Infof("%s executing: key=%s", label, req.Key)

	resp, err := p.clientRaw.GetMappings(p.requestContext(), &pulumirpc.GetMappingsRequest{
		Key: req.Key,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented {
			// For backwards compatibility just return nil to indicate unimplemented.
			logging.V(7).Infof("%s unimplemented", label)
			return GetMappingsResponse{}, nil
		}
		logging.V(7).Infof("%s failed: %v", label, rpcError)
		return GetMappingsResponse{}, err
	}

	logging.V(7).Infof("%s success: providers=%v", label, resp.Providers)
	// Ensure we don't return nil here because we use it as an "unimplemented" flag elsewhere in the system
	if resp.Providers == nil {
		resp.Providers = []string{}
	}
	return GetMappingsResponse{resp.Providers}, nil
}
