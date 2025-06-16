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

package deploytest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type ResourceMonitor struct {
	conn   *grpc.ClientConn
	resmon pulumirpc.ResourceMonitorClient

	supportsSecrets            bool
	supportsResourceReferences bool
}

func dialMonitor(ctx context.Context, endpoint string) (*ResourceMonitor, error) {
	// Connect to the resource monitor and create an appropriate client.
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not connect to resource monitor: %w", err)
	}
	resmon := pulumirpc.NewResourceMonitorClient(conn)

	// Check feature support.
	supportsSecrets, err := supportsFeature(ctx, resmon, "secrets")
	if err != nil {
		contract.IgnoreError(conn.Close())
		return nil, fmt.Errorf("could not determine whether secrets are supported: %w", err)
	}
	supportsResourceReferences, err := supportsFeature(ctx, resmon, "resourceReferences")
	if err != nil {
		contract.IgnoreError(conn.Close())
		return nil, fmt.Errorf("could not determine whether resource references are supported: %w", err)
	}

	// Fire up a resource monitor client and return.
	return &ResourceMonitor{
		conn:                       conn,
		resmon:                     resmon,
		supportsSecrets:            supportsSecrets,
		supportsResourceReferences: supportsResourceReferences,
	}, nil
}

func supportsFeature(ctx context.Context, resmon pulumirpc.ResourceMonitorClient, id string) (bool, error) {
	resp, err := resmon.SupportsFeature(ctx, &pulumirpc.SupportsFeatureRequest{Id: id})
	if err != nil {
		return false, err
	}
	return resp.GetHasSupport(), nil
}

func parseSourcePosition(raw string) (*pulumirpc.SourcePosition, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	pos := pulumirpc.SourcePosition{
		Uri: fmt.Sprintf("%v://%v", u.Scheme, u.Path),
	}

	line, col, _ := strings.Cut(u.Fragment, ",")
	if line != "" {
		l, err := strconv.ParseInt(line, 10, 32)
		if err != nil {
			return nil, err
		}
		//nolint:gosec // ParseInt will return an error if the size is too large.
		pos.Line = int32(l)
	}
	if col != "" {
		c, err := strconv.ParseInt(col, 10, 32)
		if err != nil {
			return nil, err
		}
		//nolint:gosec // ParseInt will return an error if the size is too large.
		pos.Column = int32(c)
	}

	return &pos, nil
}

func (rm *ResourceMonitor) Close() error {
	return rm.conn.Close()
}

func NewResourceMonitor(resmon pulumirpc.ResourceMonitorClient) *ResourceMonitor {
	return &ResourceMonitor{resmon: resmon}
}

type ResourceOptions struct {
	Parent                  resource.URN
	Protect                 *bool
	Dependencies            []resource.URN
	Provider                string
	Inputs                  resource.PropertyMap
	PropertyDeps            map[resource.PropertyKey][]resource.URN
	DeleteBeforeReplace     *bool
	Version                 string
	PluginDownloadURL       string
	PluginChecksums         map[string][]byte
	IgnoreChanges           []string
	ReplaceOnChanges        []string
	AliasURNs               []resource.URN
	Aliases                 []*pulumirpc.Alias
	ImportID                resource.ID
	CustomTimeouts          *resource.CustomTimeouts
	RetainOnDelete          *bool
	DeletedWith             resource.URN
	SupportsPartialValues   *bool
	Remote                  bool
	Providers               map[string]string
	AdditionalSecretOutputs []resource.PropertyKey
	AliasSpecs              bool

	SourcePosition            string
	DisableSecrets            bool
	DisableResourceReferences bool
	GrpcRequestHeaders        map[string]string

	Transforms []*pulumirpc.Callback

	SupportsResultReporting bool
	PackageRef              string
}

func (rm *ResourceMonitor) unmarshalProperties(props *structpb.Struct) (resource.PropertyMap, error) {
	// Note that `Keep*` flags are set to `true` so the caller can detect secrets, resource refs, etc that are
	// erroneously returned (e.g. secrets/resource refs that are returned even though the caller has not set
	// the relevant `Accept*` to `true` above).
	return plugin.UnmarshalProperties(props, plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      true,
		KeepResources:    true,
		KeepOutputValues: true,
	})
}

type RegisterResourceResponse struct {
	URN          resource.URN
	ID           resource.ID
	Outputs      resource.PropertyMap
	Dependencies map[resource.PropertyKey][]resource.URN
	Result       pulumirpc.Result
}

func (rm *ResourceMonitor) RegisterResource(t tokens.Type, name string, custom bool,
	options ...ResourceOptions,
) (*RegisterResourceResponse, error) {
	var opts ResourceOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.Inputs == nil {
		opts.Inputs = resource.PropertyMap{}
	}

	// marshal inputs
	ins, err := plugin.MarshalProperties(opts.Inputs, plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepSecrets:      rm.supportsSecrets,
		KeepResources:    rm.supportsResourceReferences,
		KeepOutputValues: opts.Remote,
	})
	if err != nil {
		return nil, err
	}

	// marshal dependencies
	deps := []string{}
	for _, d := range opts.Dependencies {
		deps = append(deps, string(d))
	}

	// marshal aliases
	aliasStrings := []string{}
	for _, a := range opts.AliasURNs {
		aliasStrings = append(aliasStrings, string(a))
	}

	inputDeps := make(map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies)
	for pk, pd := range opts.PropertyDeps {
		pdeps := []string{}
		for _, d := range pd {
			pdeps = append(pdeps, string(d))
		}
		inputDeps[string(pk)] = &pulumirpc.RegisterResourceRequest_PropertyDependencies{
			Urns: pdeps,
		}
	}

	var timeouts *pulumirpc.RegisterResourceRequest_CustomTimeouts
	if opts.CustomTimeouts != nil {
		timeouts = &pulumirpc.RegisterResourceRequest_CustomTimeouts{
			Create: prepareTestTimeout(opts.CustomTimeouts.Create),
			Update: prepareTestTimeout(opts.CustomTimeouts.Update),
			Delete: prepareTestTimeout(opts.CustomTimeouts.Delete),
		}
	}

	deleteBeforeReplace := false
	if opts.DeleteBeforeReplace != nil {
		deleteBeforeReplace = *opts.DeleteBeforeReplace
	}
	supportsPartialValues := true
	if opts.SupportsPartialValues != nil {
		supportsPartialValues = *opts.SupportsPartialValues
	}
	additionalSecretOutputs := make([]string, len(opts.AdditionalSecretOutputs))
	for i, v := range opts.AdditionalSecretOutputs {
		additionalSecretOutputs[i] = string(v)
	}

	var sourcePosition *pulumirpc.SourcePosition
	if opts.SourcePosition != "" {
		sourcePosition, err = parseSourcePosition(opts.SourcePosition)
		if err != nil {
			return nil, err
		}
	}

	requestInput := &pulumirpc.RegisterResourceRequest{
		Type:                       string(t),
		Name:                       name,
		Custom:                     custom,
		Parent:                     string(opts.Parent),
		Protect:                    opts.Protect,
		Dependencies:               deps,
		Provider:                   opts.Provider,
		Object:                     ins,
		PropertyDependencies:       inputDeps,
		DeleteBeforeReplace:        deleteBeforeReplace,
		DeleteBeforeReplaceDefined: opts.DeleteBeforeReplace != nil,
		IgnoreChanges:              opts.IgnoreChanges,
		AcceptSecrets:              !opts.DisableSecrets,
		AcceptResources:            !opts.DisableResourceReferences,
		Version:                    opts.Version,
		AliasURNs:                  aliasStrings,
		ImportId:                   string(opts.ImportID),
		CustomTimeouts:             timeouts,
		SupportsPartialValues:      supportsPartialValues,
		Remote:                     opts.Remote,
		ReplaceOnChanges:           opts.ReplaceOnChanges,
		Providers:                  opts.Providers,
		PluginDownloadURL:          opts.PluginDownloadURL,
		PluginChecksums:            opts.PluginChecksums,
		RetainOnDelete:             opts.RetainOnDelete,
		AdditionalSecretOutputs:    additionalSecretOutputs,
		Aliases:                    opts.Aliases,
		DeletedWith:                string(opts.DeletedWith),
		AliasSpecs:                 opts.AliasSpecs,
		SourcePosition:             sourcePosition,
		Transforms:                 opts.Transforms,
		SupportsResultReporting:    opts.SupportsResultReporting,
		PackageRef:                 opts.PackageRef,
	}

	ctx := context.Background()
	if len(opts.GrpcRequestHeaders) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(opts.GrpcRequestHeaders))
	}

	// submit request
	resp, err := rm.resmon.RegisterResource(ctx, requestInput)
	if err != nil {
		return nil, err
	}
	// unmarshal outputs
	outs, err := rm.unmarshalProperties(resp.Object)
	if err != nil {
		return nil, err
	}

	// unmarshal dependencies
	depsMap := make(map[resource.PropertyKey][]resource.URN)
	for k, p := range resp.PropertyDependencies {
		var urns []resource.URN
		for _, urn := range p.Urns {
			urns = append(urns, resource.URN(urn))
		}
		depsMap[resource.PropertyKey(k)] = urns
	}

	return &RegisterResourceResponse{
		URN:          resource.URN(resp.Urn),
		ID:           resource.ID(resp.Id),
		Outputs:      outs,
		Dependencies: depsMap,
		Result:       resp.Result,
	}, nil
}

func (rm *ResourceMonitor) RegisterResourceOutputs(urn resource.URN, outputs resource.PropertyMap) error {
	// marshal outputs
	outs, err := plugin.MarshalProperties(outputs, plugin.MarshalOptions{
		KeepUnknowns: true,
	})
	if err != nil {
		return err
	}

	// submit request
	_, err = rm.resmon.RegisterResourceOutputs(context.Background(), &pulumirpc.RegisterResourceOutputsRequest{
		Urn:     string(urn),
		Outputs: outs,
	})
	return err
}

func (rm *ResourceMonitor) ReadResource(t tokens.Type, name string, id resource.ID, parent resource.URN,
	inputs resource.PropertyMap, provider, version, sourcePosition string, packageRef string,
) (resource.URN, resource.PropertyMap, error) {
	// marshal inputs
	ins, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepResources: true,
	})
	if err != nil {
		return "", nil, err
	}

	var sourcePos *pulumirpc.SourcePosition
	if sourcePosition != "" {
		sourcePos, err = parseSourcePosition(sourcePosition)
		if err != nil {
			return "", nil, err
		}
	}

	// submit request
	resp, err := rm.resmon.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
		Type:           string(t),
		Name:           name,
		Id:             string(id),
		Parent:         string(parent),
		Provider:       provider,
		Properties:     ins,
		Version:        version,
		SourcePosition: sourcePos,
		PackageRef:     packageRef,
	})
	if err != nil {
		return "", nil, err
	}

	// unmarshal outputs
	outs, err := rm.unmarshalProperties(resp.Properties)
	if err != nil {
		return "", nil, err
	}

	return resource.URN(resp.Urn), outs, nil
}

func (rm *ResourceMonitor) Invoke(tok tokens.ModuleMember, inputs resource.PropertyMap,
	provider string, version string, packageRef string,
) (resource.PropertyMap, []*pulumirpc.CheckFailure, error) {
	// marshal inputs
	ins, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepResources: true,
	})
	if err != nil {
		return nil, nil, err
	}

	// submit request
	resp, err := rm.resmon.Invoke(context.Background(), &pulumirpc.ResourceInvokeRequest{
		Tok:        string(tok),
		Provider:   provider,
		Args:       ins,
		Version:    version,
		PackageRef: packageRef,
	})
	if err != nil {
		return nil, nil, err
	}

	// handle failures
	if len(resp.Failures) != 0 {
		return nil, resp.Failures, nil
	}

	// unmarshal outputs
	outs, err := rm.unmarshalProperties(resp.Return)
	if err != nil {
		return nil, nil, err
	}

	return outs, nil, nil
}

func (rm *ResourceMonitor) Call(
	tok tokens.ModuleMember, args resource.PropertyMap, argDependencies map[resource.PropertyKey][]resource.URN,
	provider string, version string, packageRef string) (resource.PropertyMap, map[resource.PropertyKey][]resource.URN,
	[]*pulumirpc.CheckFailure, error,
) {
	// marshal inputs
	mArgs, err := plugin.MarshalProperties(args, plugin.MarshalOptions{
		KeepUnknowns:     true,
		KeepResources:    true,
		KeepSecrets:      true,
		KeepOutputValues: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	mArgDependencies := make(map[string]*pulumirpc.ResourceCallRequest_ArgumentDependencies)
	for k, p := range argDependencies {
		urns := make([]string, len(p))
		for i, urn := range p {
			urns[i] = string(urn)
		}
		mArgDependencies[string(k)] = &pulumirpc.ResourceCallRequest_ArgumentDependencies{
			Urns: urns,
		}
	}

	// submit request
	resp, err := rm.resmon.Call(context.Background(), &pulumirpc.ResourceCallRequest{
		Tok:             string(tok),
		Provider:        provider,
		Args:            mArgs,
		ArgDependencies: mArgDependencies,
		Version:         version,
		PackageRef:      packageRef,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	// handle failures
	if len(resp.Failures) != 0 {
		return nil, nil, resp.Failures, nil
	}

	// unmarshal outputs
	outs, err := rm.unmarshalProperties(resp.Return)
	if err != nil {
		return nil, nil, nil, err
	}

	// unmarshal return deps
	deps := make(map[resource.PropertyKey][]resource.URN)
	for k, p := range resp.ReturnDependencies {
		var urns []resource.URN
		for _, urn := range p.Urns {
			urns = append(urns, resource.URN(urn))
		}
		deps[resource.PropertyKey(k)] = urns
	}

	return outs, deps, nil, nil
}

func (rm *ResourceMonitor) RegisterStackTransform(callback *pulumirpc.Callback) error {
	_, err := rm.resmon.RegisterStackTransform(context.Background(), callback)
	return err
}

func (rm *ResourceMonitor) RegisterStackInvokeTransform(callback *pulumirpc.Callback) error {
	_, err := rm.resmon.RegisterStackInvokeTransform(context.Background(), callback)
	return err
}

func (rm *ResourceMonitor) RegisterPackage(pkg, version, downloadURL string, checksums map[string][]byte,
	parameterization *pulumirpc.Parameterization,
) (string, error) {
	resp, err := rm.resmon.RegisterPackage(context.Background(), &pulumirpc.RegisterPackageRequest{
		Name:             pkg,
		Version:          version,
		DownloadUrl:      downloadURL,
		Checksums:        checksums,
		Parameterization: parameterization,
	})
	if err != nil {
		return "", err
	}
	return resp.Ref, nil
}

func (rm *ResourceMonitor) SignalAndWaitForShutdown(ctx context.Context) error {
	_, err := rm.resmon.SignalAndWaitForShutdown(ctx, &emptypb.Empty{})
	return err
}

func prepareTestTimeout(timeout float64) string {
	if timeout == 0 {
		return ""
	}
	return time.Duration(timeout * float64(time.Second)).String()
}
