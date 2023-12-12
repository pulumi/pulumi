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
	"strconv"
	"strings"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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

// The `Type()` for the NodeJS dynamic provider.  Logically, this is the same as calling
// providers.MakeProviderType(tokens.Package("pulumi-nodejs")), but does not depend on the providers package
// (a direct dependency would cause a cyclic import issue.
//
// This is needed because we have to handle some buggy behavior that previous versions of this provider implemented.
const nodejsDynamicProviderType = "pulumi:providers:pulumi-nodejs"

// The `Type()` for the Kubernetes provider.  Logically, this is the same as calling
// providers.MakeProviderType(tokens.Package("kubernetes")), but does not depend on the providers package
// (a direct dependency would cause a cyclic import issue.
//
// This is needed because we have to handle some buggy behavior that previous versions of this provider implemented.
const kubernetesProviderType = "pulumi:providers:kubernetes"

// provider reflects a resource plugin, loaded dynamically for a single package.
type provider struct {
	ctx                    *Context                         // a plugin context for caching, etc.
	pkg                    tokens.Package                   // the Pulumi package containing this provider's resources.
	plug                   *plugin                          // the actual plugin process wrapper.
	clientRaw              pulumirpc.ResourceProviderClient // the raw provider client; usually unsafe to use directly.
	disableProviderPreview bool                             // true if previews for Create and Update are disabled.
	legacyPreview          bool                             // enables legacy behavior for unconfigured provider previews.

	configSource *promise.CompletionSource[pluginConfig] // the source for the provider's configuration.
}

// pluginConfig holds the configuration of the provider
// as specified by the Configure call.
type pluginConfig struct {
	known bool // true if all configuration values are known.

	acceptSecrets   bool // true if this plugin accepts strongly-typed secrets.
	acceptResources bool // true if this plugin accepts strongly-typed resource refs.
	acceptOutputs   bool // true if this plugin accepts output values.
	supportsPreview bool // true if this plugin supports previews for Create and Update.
}

// NewProvider attempts to bind to a given package's resource plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewProvider(host Host, ctx *Context, pkg tokens.Package, version *semver.Version,
	options map[string]interface{}, disableProviderPreview bool, jsonConfig string,
) (Provider, error) {
	// See if this is a provider we just want to attach to
	var plug *plugin
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

	prefix := fmt.Sprintf("%v (resource)", pkg)

	if optAttach != "" {
		port, err := strconv.Atoi(optAttach)
		if err != nil {
			return nil, fmt.Errorf("Expected a numeric port, got %s in PULUMI_DEBUG_PROVIDERS: %w",
				optAttach, err)
		}

		conn, err := dialPlugin(port, pkg.String(), prefix, providerPluginDialOptions(ctx, pkg, ""))
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
		path, err := workspace.GetPluginPath(ctx.Diag,
			workspace.ResourcePlugin, strings.ReplaceAll(string(pkg), tokens.QNameDelimiter, "_"),
			version, host.GetProjectPlugins())
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
		if jsonConfig != "" {
			env = append(env, "PULUMI_CONFIG="+jsonConfig)
		}
		plug, err = newPlugin(ctx, ctx.Pwd, path, prefix,
			workspace.ResourcePlugin, []string{host.ServerAddr()}, env, providerPluginDialOptions(ctx, pkg, ""))
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

	// If we just attached (i.e. plugin bin is nil) we need to call attach
	if plug.Bin == "" {
		err := p.Attach(host.ServerAddr())
		if err != nil {
			return nil, err
		}
	}

	return p, nil
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

	plug, err := newPlugin(ctx, ctx.Pwd, path, "",
		workspace.ResourcePlugin, []string{host.ServerAddr()}, env, providerPluginDialOptions(ctx, "", path))
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

// GetSchema fetches the schema for this resource provider, if any.
func (p *provider) GetSchema(version int) ([]byte, error) {
	resp, err := p.clientRaw.GetSchema(p.requestContext(), &pulumirpc.GetSchemaRequest{
		Version: int32(version),
	})
	if err != nil {
		return nil, err
	}
	return []byte(resp.GetSchema()), nil
}

// CheckConfig validates the configuration for this resource provider.
func (p *provider) CheckConfig(urn resource.URN, olds,
	news resource.PropertyMap, allowUnknowns bool,
) (resource.PropertyMap, []CheckFailure, error) {
	label := fmt.Sprintf("%s.CheckConfig(%s)", p.label(), urn)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d)", label, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{
		Label:        label + ".olds",
		KeepUnknowns: allowUnknowns,
	})
	if err != nil {
		return nil, nil, err
	}

	mnews, err := MarshalProperties(news, MarshalOptions{
		Label:        label + ".news",
		KeepUnknowns: allowUnknowns,
	})
	if err != nil {
		return nil, nil, err
	}

	resp, err := p.clientRaw.CheckConfig(p.requestContext(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented || isDiffCheckConfigLogicallyUnimplemented(rpcError, urn.Type()) {
			// For backwards compatibility, just return the news as if the provider was okay with them.
			logging.V(7).Infof("%s unimplemented rpc: returning news as is", label)
			return news, nil, nil
		}
		logging.V(8).Infof("%s provider received rpc error `%s`: `%s`", label, rpcError.Code(),
			rpcError.Message())
		return nil, nil, err
	}

	// Unmarshal the provider inputs.
	var inputs resource.PropertyMap
	if ins := resp.GetInputs(); ins != nil {
		inputs, err = UnmarshalProperties(ins, MarshalOptions{
			Label:          label + ".inputs",
			KeepUnknowns:   allowUnknowns,
			RejectUnknowns: !allowUnknowns,
			KeepSecrets:    true,
			KeepResources:  true,
		})
		if err != nil {
			return nil, nil, err
		}
	}

	// And now any properties that failed verification.
	failures := slice.Prealloc[CheckFailure](len(resp.GetFailures()))
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	// Copy over any secret annotations, since we could not pass any to the provider, and return.
	annotateSecrets(inputs, news)
	logging.V(7).Infof("%s success: inputs=#%d failures=#%d", label, len(inputs), len(failures))
	return inputs, failures, nil
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
func (p *provider) DiffConfig(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (DiffResult, error) {
	label := fmt.Sprintf("%s.DiffConfig(%s)", p.label(), urn)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(oldInputs), len(oldOutputs), len(newInputs))

	mOldInputs, err := MarshalProperties(oldInputs, MarshalOptions{
		Label:        label + ".oldInputs",
		KeepUnknowns: true,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mOldOutputs, err := MarshalProperties(oldOutputs, MarshalOptions{
		Label:        label + ".oldOutputs",
		KeepUnknowns: true,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mNewInputs, err := MarshalProperties(newInputs, MarshalOptions{
		Label:        label + ".newInputs",
		KeepUnknowns: true,
	})
	if err != nil {
		return DiffResult{}, err
	}

	resp, err := p.clientRaw.DiffConfig(p.requestContext(), &pulumirpc.DiffRequest{
		Urn:           string(urn),
		OldInputs:     mOldInputs,
		Olds:          mOldOutputs,
		News:          mNewInputs,
		IgnoreChanges: ignoreChanges,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented || isDiffCheckConfigLogicallyUnimplemented(rpcError, urn.Type()) {
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
	isEmptyAsset := func(v *resource.Asset) bool {
		return v.Text == "" && v.Path == "" && v.URI == ""
	}

	isEmptyArchive := func(v *resource.Archive) bool {
		return v.Path == "" && v.URI == "" && v.Assets == nil
	}

	originalAssets := map[string]*resource.Asset{}
	originalArchives := map[string]*resource.Archive{}

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
func (p *provider) Configure(inputs resource.PropertyMap) error {
	label := p.label() + ".Configure()"
	logging.V(7).Infof("%s executing (#vars=%d)", label, len(inputs))

	// Convert the inputs to a config map. If any are unknown, do not configure the underlying plugin: instead, leave
	// the cfgknown bit unset and carry on.
	config := make(map[string]string)
	for k, v := range inputs {
		if k == "version" {
			continue
		}

		if v.ContainsUnknowns() {
			p.configSource.MustFulfill(pluginConfig{
				known:           false,
				acceptSecrets:   false,
				acceptResources: false,
			})
			return nil
		}

		mapped := removeSecrets(v)
		if _, isString := mapped.(string); !isString {
			marshalled, err := json.Marshal(mapped)
			if err != nil {
				err := fmt.Errorf("marshaling configuration property '%v': %w", k, err)
				p.configSource.MustReject(err)
				return err
			}
			mapped = string(marshalled)
		}

		// Pass the older spelling of a configuration key across the RPC interface, for now, to support
		// providers which are on the older plan.
		config[string(p.Pkg())+":config:"+string(k)] = mapped.(string)
	}

	minputs, err := MarshalProperties(inputs, MarshalOptions{
		Label:         label + ".inputs",
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		err := fmt.Errorf("marshaling provider inputs: %w", err)
		p.configSource.MustReject(err)
		return err
	}

	// Spawn the configure to happen in parallel.  This ensures that we remain responsive elsewhere that might
	// want to make forward progress, even as the configure call is happening.
	go func() {
		resp, err := p.clientRaw.Configure(p.requestContext(), &pulumirpc.ConfigureRequest{
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

		p.configSource.MustFulfill(pluginConfig{
			known:           true,
			acceptSecrets:   resp.GetAcceptSecrets(),
			acceptResources: resp.GetAcceptResources(),
			supportsPreview: resp.GetSupportsPreview(),
			acceptOutputs:   resp.GetAcceptOutputs(),
		})
	}()

	return nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *provider) Check(urn resource.URN,
	olds, news resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte,
) (resource.PropertyMap, []CheckFailure, error) {
	label := fmt.Sprintf("%s.Check(%s)", p.label(), urn)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d)", label, len(olds), len(news))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return nil, nil, err
	}

	// If the configuration for this provider was not fully known--e.g. if we are doing a preview and some input
	// property was sourced from another resource's output properties--don't call into the underlying provider.
	if !pcfg.known {
		return news, nil, nil
	}

	molds, err := MarshalProperties(olds, MarshalOptions{
		Label:         label + ".olds",
		KeepUnknowns:  allowUnknowns,
		KeepSecrets:   pcfg.acceptSecrets,
		KeepResources: pcfg.acceptResources,
	})
	if err != nil {
		return nil, nil, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{
		Label:         label + ".news",
		KeepUnknowns:  allowUnknowns,
		KeepSecrets:   pcfg.acceptSecrets,
		KeepResources: pcfg.acceptResources,
	})
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Check(p.requestContext(), &pulumirpc.CheckRequest{
		Urn:        string(urn),
		Olds:       molds,
		News:       mnews,
		RandomSeed: randomSeed,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
		return nil, nil, rpcError
	}

	// Unmarshal the provider inputs.
	var inputs resource.PropertyMap
	if ins := resp.GetInputs(); ins != nil {
		inputs, err = UnmarshalProperties(ins, MarshalOptions{
			Label:          label + ".inputs",
			KeepUnknowns:   allowUnknowns,
			RejectUnknowns: !allowUnknowns,
			KeepSecrets:    true,
			KeepResources:  true,
		})
		if err != nil {
			return nil, nil, err
		}
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !pcfg.acceptSecrets {
		annotateSecrets(inputs, news)
	}

	// And now any properties that failed verification.
	failures := slice.Prealloc[CheckFailure](len(resp.GetFailures()))
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	logging.V(7).Infof("%s success: inputs=#%d failures=#%d", label, len(inputs), len(failures))
	return inputs, failures, nil
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *provider) Diff(urn resource.URN, id resource.ID,
	oldInputs, oldOutputs, newInputs resource.PropertyMap, allowUnknowns bool,
	ignoreChanges []string,
) (DiffResult, error) {
	contract.Assertf(urn != "", "Diff requires a URN")
	contract.Assertf(id != "", "Diff requires an ID")
	contract.Assertf(oldInputs != nil, "Diff requires old input properties")
	contract.Assertf(newInputs != nil, "Diff requires new input properties")
	contract.Assertf(oldOutputs != nil, "Diff requires old output properties")

	label := fmt.Sprintf("%s.Diff(%s,%s)", p.label(), urn, id)
	logging.V(7).Infof("%s: executing (#oldInputs=%d#oldOutputs=%d,#newInputs=%d)",
		label, len(oldInputs), len(oldOutputs), len(newInputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
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

	mOldInputs, err := MarshalProperties(oldInputs, MarshalOptions{
		Label:              label + ".oldInputs",
		ElideAssetContents: true,
		KeepUnknowns:       allowUnknowns,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mOldOutputs, err := MarshalProperties(oldOutputs, MarshalOptions{
		Label:              label + ".oldOutputs",
		ElideAssetContents: true,
		KeepUnknowns:       allowUnknowns,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return DiffResult{}, err
	}

	mNewInputs, err := MarshalProperties(newInputs, MarshalOptions{
		Label:              label + ".newInputs",
		ElideAssetContents: true,
		KeepUnknowns:       allowUnknowns,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return DiffResult{}, err
	}

	resp, err := client.Diff(p.requestContext(), &pulumirpc.DiffRequest{
		Id:            string(id),
		Urn:           string(urn),
		OldInputs:     mOldInputs,
		Olds:          mOldOutputs,
		News:          mNewInputs,
		IgnoreChanges: ignoreChanges,
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
func (p *provider) Create(urn resource.URN, props resource.PropertyMap, timeout float64, preview bool) (resource.ID,
	resource.PropertyMap, resource.Status, error,
) {
	contract.Assertf(urn != "", "Create requires a URN")
	contract.Assertf(props != nil, "Create requires properties")

	label := fmt.Sprintf("%s.Create(%s)", p.label(), urn)
	logging.V(7).Infof("%s executing (#props=%v)", label, len(props))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return "", nil, resource.StatusOK, err
	}

	// If this is a preview and the plugin does not support provider previews, or if the configuration for the provider
	// is not fully known, hand back an empty property map. This will force the language SDK will to treat all properties
	// as unknown, which is conservatively correct.
	//
	// If the provider does not support previews, return the inputs as the state. Note that this can cause problems for
	// the language SDKs if there are input and state properties that share a name but expect differently-shaped values.
	if preview {
		// TODO: it would be great to swap the order of these if statements. This would prevent a behavioral change for
		// providers that do not support provider previews, which will always return the inputs as state regardless of
		// whether or not the config is known. Unfortunately, we can't, since the `supportsPreview` bit depends on the
		// result of `Configure`, which we won't call if the `cfgknown` is false. It may be worth fixing this catch-22
		// by extending the provider gRPC interface with a `SupportsFeature` API similar to the language monitor.
		if !pcfg.known {
			if p.legacyPreview {
				return "", props, resource.StatusOK, nil
			}
			return "", resource.PropertyMap{}, resource.StatusOK, nil
		}
		if !pcfg.supportsPreview || p.disableProviderPreview {
			return "", props, resource.StatusOK, nil
		}
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assertf(pcfg.known, "Create cannot be called if the configuration is unknown")

	mprops, err := MarshalProperties(props, MarshalOptions{
		Label:         label + ".inputs",
		KeepUnknowns:  preview,
		KeepSecrets:   pcfg.acceptSecrets,
		KeepResources: pcfg.acceptResources,
	})
	if err != nil {
		return "", nil, resource.StatusOK, err
	}

	var id resource.ID
	var liveObject *_struct.Struct
	var resourceError error
	resourceStatus := resource.StatusOK
	resp, err := client.Create(p.requestContext(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: mprops,
		Timeout:    timeout,
		Preview:    preview,
	})
	if err != nil {
		resourceStatus, id, liveObject, _, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, resourceError)

		if resourceStatus != resource.StatusPartialFailure {
			return "", nil, resourceStatus, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		id = resource.ID(resp.GetId())
		liveObject = resp.GetProperties()
	}

	if id == "" && !preview {
		return "", nil, resource.StatusUnknown,
			fmt.Errorf("plugin for package '%v' returned empty resource.ID from create '%v'", p.pkg, urn)
	}

	outs, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label:          label + ".outputs",
		RejectUnknowns: !preview,
		KeepUnknowns:   preview,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return "", nil, resourceStatus, err
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !pcfg.acceptSecrets {
		annotateSecrets(outs, props)
	}

	logging.V(7).Infof("%s success: id=%s; #outs=%d", label, id, len(outs))
	if resourceError == nil {
		return id, outs, resourceStatus, nil
	}
	return id, outs, resourceStatus, resourceError
}

// read the current live state associated with a resource.  enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource id, but may also include some properties.
func (p *provider) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap,
) (ReadResult, resource.Status, error) {
	contract.Assertf(urn != "", "Read URN was empty")
	contract.Assertf(id != "", "Read ID was empty")

	label := fmt.Sprintf("%s.Read(%s,%s)", p.label(), id, urn)
	logging.V(7).Infof("%s executing (#inputs=%v, #state=%v)", label, len(inputs), len(state))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return ReadResult{}, resource.StatusUnknown, err
	}

	// If the provider is not fully configured, return an empty bag.
	if !pcfg.known {
		return ReadResult{
			Outputs: resource.PropertyMap{},
			Inputs:  resource.PropertyMap{},
		}, resource.StatusUnknown, nil
	}

	// Marshal the resource inputs and state so we can perform the RPC.
	var minputs *_struct.Struct
	if inputs != nil {
		m, err := MarshalProperties(inputs, MarshalOptions{
			Label:              label,
			ElideAssetContents: true,
			KeepSecrets:        pcfg.acceptSecrets,
			KeepResources:      pcfg.acceptResources,
		})
		if err != nil {
			return ReadResult{}, resource.StatusUnknown, err
		}
		minputs = m
	}
	mstate, err := MarshalProperties(state, MarshalOptions{
		Label:              label,
		ElideAssetContents: true,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return ReadResult{}, resource.StatusUnknown, err
	}

	// Now issue the read request over RPC, blocking until it finished.
	var readID resource.ID
	var liveObject *_struct.Struct
	var liveInputs *_struct.Struct
	var resourceError error
	resourceStatus := resource.StatusOK
	resp, err := client.Read(p.requestContext(), &pulumirpc.ReadRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: mstate,
		Inputs:     minputs,
	})
	if err != nil {
		resourceStatus, readID, liveObject, liveInputs, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, err)

		if resourceStatus != resource.StatusPartialFailure {
			return ReadResult{}, resourceStatus, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		readID = resource.ID(resp.GetId())
		liveObject = resp.GetProperties()
		liveInputs = resp.GetInputs()
	}

	// If the resource was missing, simply return a nil property map.
	if string(readID) == "" {
		return ReadResult{}, resourceStatus, nil
	}

	// Finally, unmarshal the resulting state properties and return them.
	newState, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label:          label + ".outputs",
		RejectUnknowns: true,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return ReadResult{}, resourceStatus, err
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
			return ReadResult{}, resourceStatus, err
		}
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !pcfg.acceptSecrets {
		annotateSecrets(newInputs, inputs)
		annotateSecrets(newState, state)
	}

	// make sure any echoed properties restore their original asset contents if they have not changed
	restoreElidedAssetContents(inputs, newInputs)
	restoreElidedAssetContents(inputs, newState)

	logging.V(7).Infof("%s success; #outs=%d, #inputs=%d", label, len(newState), len(newInputs))
	return ReadResult{
		ID:      readID,
		Outputs: newState,
		Inputs:  newInputs,
	}, resourceStatus, resourceError
}

// Update updates an existing resource with new values.
func (p *provider) Update(urn resource.URN, id resource.ID,
	oldInputs, oldOutputs, newInputs resource.PropertyMap, timeout float64,
	ignoreChanges []string, preview bool,
) (resource.PropertyMap, resource.Status, error) {
	contract.Assertf(urn != "", "Update requires a URN")
	contract.Assertf(id != "", "Update requires an ID")
	contract.Assertf(oldInputs != nil, "Update requires old inputs")
	contract.Assertf(oldOutputs != nil, "Update requires old outputs")
	contract.Assertf(newInputs != nil, "Update requires new properties")

	label := fmt.Sprintf("%s.Update(%s,%s)", p.label(), id, urn)
	logging.V(7).Infof("%s executing (#oldInputs=%v,#oldOutputs=%v,#newInputs=%v)",
		label, len(oldInputs), len(oldOutputs), len(newInputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return newInputs, resource.StatusOK, err
	}

	// If this is a preview and the plugin does not support provider previews, or if the configuration for the provider
	// is not fully known, hand back an empty property map. This will force the language SDK to treat all properties
	// as unknown, which is conservatively correct.
	//
	// If the provider does not support previews, return the inputs as the state. Note that this can cause problems for
	// the language SDKs if there are input and state properties that share a name but expect differently-shaped values.
	if preview {
		// TODO: it would be great to swap the order of these if statements. This would prevent a behavioral change for
		// providers that do not support provider previews, which will always return the inputs as state regardless of
		// whether or not the config is known. Unfortunately, we can't, since the `supportsPreview` bit depends on the
		// result of `Configure`, which we won't call if the `cfgknown` is false. It may be worth fixing this catch-22
		// by extending the provider gRPC interface with a `SupportsFeature` API similar to the language monitor.
		if !pcfg.known {
			if p.legacyPreview {
				return newInputs, resource.StatusOK, nil
			}
			return resource.PropertyMap{}, resource.StatusOK, nil
		}
		if !pcfg.supportsPreview || p.disableProviderPreview {
			return newInputs, resource.StatusOK, nil
		}
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assertf(pcfg.known, "Update cannot be called if the configuration is unknown")

	mOldInputs, err := MarshalProperties(oldInputs, MarshalOptions{
		Label:              label + ".oldInputs",
		ElideAssetContents: true,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return nil, resource.StatusOK, err
	}
	mOldOutputs, err := MarshalProperties(oldOutputs, MarshalOptions{
		Label:              label + ".oldOutputs",
		ElideAssetContents: true,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return nil, resource.StatusOK, err
	}
	mNewInputs, err := MarshalProperties(newInputs, MarshalOptions{
		Label:         label + ".newInputs",
		KeepUnknowns:  preview,
		KeepSecrets:   pcfg.acceptSecrets,
		KeepResources: pcfg.acceptResources,
	})
	if err != nil {
		return nil, resource.StatusOK, err
	}

	var liveObject *_struct.Struct
	var resourceError error
	resourceStatus := resource.StatusOK
	resp, err := client.Update(p.requestContext(), &pulumirpc.UpdateRequest{
		Id:            string(id),
		Urn:           string(urn),
		Olds:          mOldOutputs,
		News:          mNewInputs,
		Timeout:       timeout,
		IgnoreChanges: ignoreChanges,
		Preview:       preview,
		OldInputs:     mOldInputs,
	})
	if err != nil {
		resourceStatus, _, liveObject, _, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, resourceError)

		if resourceStatus != resource.StatusPartialFailure {
			return nil, resourceStatus, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		liveObject = resp.GetProperties()
	}

	outs, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label:          label + ".outputs",
		RejectUnknowns: !preview,
		KeepUnknowns:   preview,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return nil, resourceStatus, err
	}

	// If we could not pass secrets to the provider, retain the secret bit on any property with the same name. This
	// allows us to retain metadata about secrets in many cases, even for providers that do not understand secrets
	// natively.
	if !pcfg.acceptSecrets {
		annotateSecrets(outs, newInputs)
	}
	logging.V(7).Infof("%s success; #outs=%d", label, len(outs))
	if resourceError == nil {
		return outs, resourceStatus, nil
	}
	return outs, resourceStatus, resourceError
}

// Delete tears down an existing resource.
func (p *provider) Delete(urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	contract.Assertf(urn != "", "Delete requires a URN")
	contract.Assertf(id != "", "Delete requires an ID")

	label := fmt.Sprintf("%s.Delete(%s,%s)", p.label(), urn, id)
	logging.V(7).Infof("%s executing (#inputs=%d, #outputs=%d)", label, len(oldInputs), len(oldOutputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return resource.StatusOK, err
	}

	// We should never call delete at preview time, so we should never see unknowns here
	contract.Assertf(pcfg.known, "Delete cannot be called if the configuration is unknown")

	minputs, err := MarshalProperties(oldInputs, MarshalOptions{
		Label:              label,
		ElideAssetContents: true,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return resource.StatusOK, err
	}

	moutputs, err := MarshalProperties(oldOutputs, MarshalOptions{
		Label:              label,
		ElideAssetContents: true,
		KeepSecrets:        pcfg.acceptSecrets,
		KeepResources:      pcfg.acceptResources,
	})
	if err != nil {
		return resource.StatusOK, err
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assertf(pcfg.known, "Delete cannot be called if the configuration is unknown")

	if _, err := client.Delete(p.requestContext(), &pulumirpc.DeleteRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: moutputs,
		Timeout:    timeout,
		OldInputs:  minputs,
	}); err != nil {
		resourceStatus, rpcErr := resourceStateAndError(err)
		logging.V(7).Infof("%s failed: %v", label, rpcErr)
		return resourceStatus, rpcErr
	}

	logging.V(7).Infof("%s success", label)
	return resource.StatusOK, nil
}

// Construct creates a new component resource from the given type, name, parent, options, and inputs, and returns
// its URN and outputs.
func (p *provider) Construct(info ConstructInfo, typ tokens.Type, name string, parent resource.URN,
	inputs resource.PropertyMap, options ConstructOptions,
) (ConstructResult, error) {
	contract.Assertf(typ != "", "Construct requires a type")
	contract.Assertf(name != "", "Construct requires a name")
	contract.Assertf(inputs != nil, "Construct requires input properties")

	label := fmt.Sprintf("%s.Construct(%s, %s, %s)", p.label(), typ, name, parent)
	logging.V(7).Infof("%s executing (#inputs=%v)", label, len(inputs))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return ConstructResult{}, err
	}

	// If the provider is not fully configured, we need to error. We can't support unknown URNs but if the
	// provider isn't configured we can't call into it to get the URN.
	if !pcfg.known {
		return ConstructResult{}, errors.New("cannot construct components if the provider is configured with unknown values")
	}

	if !pcfg.acceptSecrets {
		return ConstructResult{}, errors.New("plugins that can construct components must support secrets")
	}

	// Marshal the input properties.
	minputs, err := MarshalProperties(inputs, MarshalOptions{
		Label:         label + ".inputs",
		KeepUnknowns:  true,
		KeepSecrets:   pcfg.acceptSecrets,
		KeepResources: pcfg.acceptResources,
		// To initially scope the use of this new feature, we only keep output values for
		// Construct and Call (when the client accepts them).
		KeepOutputValues: pcfg.acceptOutputs,
	})
	if err != nil {
		return ConstructResult{}, err
	}

	// Marshal the aliases.
	aliasURNs := make([]string, len(options.Aliases))
	for i, alias := range options.Aliases {
		aliasURNs[i] = string(alias.URN)
	}

	// Marshal the dependencies.
	dependencies := make([]string, len(options.Dependencies))
	for i, dep := range options.Dependencies {
		dependencies[i] = string(dep)
	}

	// Marshal the property dependencies.
	inputDependencies := map[string]*pulumirpc.ConstructRequest_PropertyDependencies{}
	for name, dependencies := range options.PropertyDependencies {
		urns := make([]string, len(dependencies))
		for i, urn := range dependencies {
			urns[i] = string(urn)
		}
		inputDependencies[string(name)] = &pulumirpc.ConstructRequest_PropertyDependencies{Urns: urns}
	}

	// Marshal the config.
	config := map[string]string{}
	for k, v := range info.Config {
		config[k.String()] = v
	}
	configSecretKeys := []string{}
	for _, k := range info.ConfigSecretKeys {
		configSecretKeys = append(configSecretKeys, k.String())
	}

	req := &pulumirpc.ConstructRequest{
		Project:                 info.Project,
		Stack:                   info.Stack,
		Config:                  config,
		ConfigSecretKeys:        configSecretKeys,
		DryRun:                  info.DryRun,
		Parallel:                int32(info.Parallel),
		MonitorEndpoint:         info.MonitorAddress,
		Type:                    string(typ),
		Name:                    name,
		Parent:                  string(parent),
		Inputs:                  minputs,
		Protect:                 options.Protect,
		Providers:               options.Providers,
		InputDependencies:       inputDependencies,
		Aliases:                 aliasURNs,
		Dependencies:            dependencies,
		AdditionalSecretOutputs: options.AdditionalSecretOutputs,
		DeletedWith:             string(options.DeletedWith),
		DeleteBeforeReplace:     options.DeleteBeforeReplace,
		IgnoreChanges:           options.IgnoreChanges,
		ReplaceOnChanges:        options.ReplaceOnChanges,
		RetainOnDelete:          options.RetainOnDelete,
	}
	if ct := options.CustomTimeouts; ct != nil {
		req.CustomTimeouts = &pulumirpc.ConstructRequest_CustomTimeouts{
			Create: ct.Create,
			Update: ct.Update,
			Delete: ct.Delete,
		}
	}

	resp, err := client.Construct(p.requestContext(), req)
	if err != nil {
		return ConstructResult{}, err
	}

	outputs, err := UnmarshalProperties(resp.GetState(), MarshalOptions{
		Label:         label + ".outputs",
		KeepUnknowns:  info.DryRun,
		KeepSecrets:   true,
		KeepResources: true,
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
	return ConstructResult{
		URN:                resource.URN(resp.GetUrn()),
		Outputs:            outputs,
		OutputDependencies: outputDependencies,
	}, nil
}

// Invoke dynamically executes a built-in function in the provider.
func (p *provider) Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap,
	[]CheckFailure, error,
) {
	contract.Assertf(tok != "", "Invoke requires a token")

	label := fmt.Sprintf("%s.Invoke(%s)", p.label(), tok)
	logging.V(7).Infof("%s executing (#args=%d)", label, len(args))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return nil, nil, err
	}

	// If the provider is not fully configured, return an empty property map.
	if !pcfg.known {
		return resource.PropertyMap{}, nil, nil
	}

	margs, err := MarshalProperties(args, MarshalOptions{
		Label:         label + ".args",
		KeepSecrets:   pcfg.acceptSecrets,
		KeepResources: pcfg.acceptResources,
	})
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Invoke(p.requestContext(), &pulumirpc.InvokeRequest{
		Tok:  string(tok),
		Args: margs,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return nil, nil, rpcError
	}

	// Unmarshal any return values.
	ret, err := UnmarshalProperties(resp.GetReturn(), MarshalOptions{
		Label:          label + ".returns",
		RejectUnknowns: true,
		KeepSecrets:    true,
		KeepResources:  true,
	})
	if err != nil {
		return nil, nil, err
	}

	// And now any properties that failed verification.
	failures := slice.Prealloc[CheckFailure](len(resp.GetFailures()))
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	logging.V(7).Infof("%s success (#ret=%d,#failures=%d) success", label, len(ret), len(failures))
	return ret, failures, nil
}

// StreamInvoke dynamically executes a built-in function in the provider, which returns a stream of
// responses.
func (p *provider) StreamInvoke(
	tok tokens.ModuleMember,
	args resource.PropertyMap,
	onNext func(resource.PropertyMap) error,
) ([]CheckFailure, error) {
	contract.Assertf(tok != "", "StreamInvoke requires a token")

	label := fmt.Sprintf("%s.StreamInvoke(%s)", p.label(), tok)
	logging.V(7).Infof("%s executing (#args=%d)", label, len(args))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return nil, err
	}

	// If the provider is not fully configured, return an empty property map.
	if !pcfg.known {
		return nil, onNext(resource.PropertyMap{})
	}

	margs, err := MarshalProperties(args, MarshalOptions{
		Label:         label + ".args",
		KeepSecrets:   pcfg.acceptSecrets,
		KeepResources: pcfg.acceptResources,
	})
	if err != nil {
		return nil, err
	}

	streamClient, err := client.StreamInvoke(
		p.requestContext(), &pulumirpc.InvokeRequest{
			Tok:  string(tok),
			Args: margs,
		})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return nil, rpcError
	}

	for {
		in, err := streamClient.Recv()
		if err == io.EOF {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		// Unmarshal response.
		ret, err := UnmarshalProperties(in.GetReturn(), MarshalOptions{
			Label:          label + ".returns",
			RejectUnknowns: true,
			KeepSecrets:    true,
			KeepResources:  true,
		})
		if err != nil {
			return nil, err
		}

		// Check properties that failed verification.
		var failures []CheckFailure
		for _, failure := range in.GetFailures() {
			failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
		}

		if len(failures) > 0 {
			return failures, nil
		}

		// Send stream message back to whoever is consuming the stream.
		if err := onNext(ret); err != nil {
			return nil, err
		}
	}
}

// Call dynamically executes a method in the provider associated with a component resource.
func (p *provider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info CallInfo,
	options CallOptions,
) (CallResult, error) {
	contract.Assertf(tok != "", "Call requires a token")

	label := fmt.Sprintf("%s.Call(%s)", p.label(), tok)
	logging.V(7).Infof("%s executing (#args=%d)", label, len(args))

	// Ensure that the plugin is configured.
	client := p.clientRaw
	pcfg, err := p.configSource.Promise().Result(context.Background())
	if err != nil {
		return CallResult{}, err
	}

	// If the provider is not fully configured, return an empty property map.
	if !pcfg.known {
		return CallResult{}, nil
	}

	margs, err := MarshalProperties(args, MarshalOptions{
		Label:         label + ".args",
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
		// To initially scope the use of this new feature, we only keep output values for
		// Construct and Call (when the client accepts them).
		KeepOutputValues: pcfg.acceptOutputs,
	})
	if err != nil {
		return CallResult{}, err
	}

	// Marshal the arg dependencies.
	argDependencies := map[string]*pulumirpc.CallRequest_ArgumentDependencies{}
	for name, dependencies := range options.ArgDependencies {
		urns := make([]string, len(dependencies))
		for i, urn := range dependencies {
			urns[i] = string(urn)
		}
		argDependencies[string(name)] = &pulumirpc.CallRequest_ArgumentDependencies{Urns: urns}
	}

	// Marshal the config.
	config := map[string]string{}
	for k, v := range info.Config {
		config[k.String()] = v
	}

	resp, err := client.Call(p.requestContext(), &pulumirpc.CallRequest{
		Tok:             string(tok),
		Args:            margs,
		ArgDependencies: argDependencies,
		Project:         info.Project,
		Stack:           info.Stack,
		Config:          config,
		DryRun:          info.DryRun,
		Parallel:        int32(info.Parallel),
		MonitorEndpoint: info.MonitorAddress,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return CallResult{}, rpcError
	}

	// Unmarshal any return values.
	ret, err := UnmarshalProperties(resp.GetReturn(), MarshalOptions{
		Label:         label + ".returns",
		KeepUnknowns:  info.DryRun,
		KeepSecrets:   true,
		KeepResources: true,
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
func (p *provider) GetPluginInfo() (workspace.PluginInfo, error) {
	label := p.label() + ".GetPluginInfo()"
	logging.V(7).Infof("%s executing", label)

	// Calling GetPluginInfo happens immediately after loading, and does not require configuration to proceed.
	// Thus, we access the clientRaw property, rather than calling getClient.
	resp, err := p.clientRaw.GetPluginInfo(p.requestContext(), &pbempty.Empty{})
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
		Kind:    workspace.ResourcePlugin,
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

func (p *provider) SignalCancellation() error {
	_, err := p.clientRaw.Cancel(p.requestContext(), &pbempty.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(8).Infof("provider received rpc error `%s`: `%s`", rpcError.Code(),
			rpcError.Message())
		switch rpcError.Code() {
		case codes.Unimplemented:
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
	resourceStatus resource.Status, id resource.ID, liveInputs, liveObject *_struct.Struct, resourceErr error,
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
func (p *provider) GetMapping(key, provider string) ([]byte, string, error) {
	label := p.label() + ".GetMapping"
	logging.V(7).Infof("%s executing: key=%s, provider=%s", label, key, provider)

	resp, err := p.clientRaw.GetMapping(p.requestContext(), &pulumirpc.GetMappingRequest{
		Key:      key,
		Provider: provider,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented {
			// For backwards compatibility, just return nothing as if the provider didn't have a mapping for
			// the given key
			logging.V(7).Infof("%s unimplemented", label)
			return nil, "", nil
		}
		logging.V(7).Infof("%s failed: %v", label, rpcError)
		return nil, "", err
	}

	logging.V(7).Infof("%s success: data=#%d provider=%s", label, len(resp.Data), resp.Provider)
	return resp.Data, resp.Provider, nil
}

func (p *provider) GetMappings(key string) ([]string, error) {
	label := p.label() + ".GetMappings"
	logging.V(7).Infof("%s executing: key=%s", label, key)

	resp, err := p.clientRaw.GetMappings(p.requestContext(), &pulumirpc.GetMappingsRequest{
		Key: key,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		code := rpcError.Code()
		if code == codes.Unimplemented {
			// For backwards compatibility just return nil to indicate unimplemented.
			logging.V(7).Infof("%s unimplemented", label)
			return nil, nil
		}
		logging.V(7).Infof("%s failed: %v", label, rpcError)
		return nil, err
	}

	logging.V(7).Infof("%s success: providers=%v", label, resp.Providers)
	// Ensure we don't return nil here because we use it as an "unimplemented" flag elsewhere in the system
	if resp.Providers == nil {
		resp.Providers = []string{}
	}
	return resp.Providers, nil
}
