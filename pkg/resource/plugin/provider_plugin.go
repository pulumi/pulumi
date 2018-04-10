// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/pkg/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// provider reflects a resource plugin, loaded dynamically for a single package.
type provider struct {
	ctx    *Context
	pkg    tokens.Package
	plug   *plugin
	client pulumirpc.ResourceProviderClient
}

// NewProvider attempts to bind to a given package's resource plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewProvider(host Host, ctx *Context, pkg tokens.Package, version *semver.Version) (Provider, error) {
	// Load the plugin's path by using the standard workspace logic.
	_, path, err := workspace.GetPluginPath(
		workspace.ResourcePlugin, strings.Replace(string(pkg), tokens.QNameDelimiter, "_", -1), version)
	if err != nil {
		return nil, err
	} else if path == "" {
		return nil, NewMissingError(workspace.PluginInfo{
			Kind: workspace.ResourcePlugin,
			Name: string(pkg),
		})
	}

	plug, err := newPlugin(ctx, path, fmt.Sprintf("%v (resource)", pkg), []string{host.ServerAddr()})
	if err != nil {
		return nil, err
	}
	contract.Assertf(plug != nil, "unexpected nil resource plugin for %s", pkg)

	return &provider{
		ctx:    ctx,
		pkg:    pkg,
		plug:   plug,
		client: pulumirpc.NewResourceProviderClient(plug.Conn),
	}, nil
}

func (p *provider) Pkg() tokens.Package { return p.pkg }

// label returns a base label for tracing functions.
func (p *provider) label() string {
	return fmt.Sprintf("Provider[%s]", p.pkg)
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *provider) Configure(vars map[config.Key]string) error {
	label := fmt.Sprintf("%s.Configure()", p.label())
	glog.V(7).Infof("%s executing (#vars=%d)", label, len(vars))
	config := make(map[string]string)
	for k, v := range vars {
		// Pass the older spelling of a configuration key across the RPC interface, for now, to support
		// providers which are on the older plan.
		config[k.Namespace()+":config:"+k.Name()] = v
	}
	_, err := p.client.Configure(p.ctx.Request(), &pulumirpc.ConfigureRequest{Variables: config})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		glog.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
		return createConfigureError(rpcError)
	}

	return nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *provider) Check(urn resource.URN,
	olds, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []CheckFailure, error) {

	label := fmt.Sprintf("%s.Check(%s)", p.label(), urn)
	glog.V(7).Infof("%s executing (#olds=%d,#news=%d", label, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{Label: fmt.Sprintf("%s.olds", label),
		KeepUnknowns: allowUnknowns})
	if err != nil {
		return nil, nil, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{Label: fmt.Sprintf("%s.news", label),
		KeepUnknowns: allowUnknowns})
	if err != nil {
		return nil, nil, err
	}

	resp, err := p.client.Check(p.ctx.Request(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		glog.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
		return nil, nil, rpcError
	}

	// Unmarshal the provider inputs.
	var inputs resource.PropertyMap
	if ins := resp.GetInputs(); ins != nil {
		inputs, err = UnmarshalProperties(ins, MarshalOptions{
			Label: fmt.Sprintf("%s.inputs", label), KeepUnknowns: allowUnknowns, RejectUnknowns: !allowUnknowns})
		if err != nil {
			return nil, nil, err
		}
	}

	// And now any properties that failed verification.
	var failures []CheckFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	glog.V(7).Infof("%s success: inputs=#%d failures=#%d", label, len(inputs), len(failures))
	return inputs, failures, nil
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *provider) Diff(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap, allowUnknowns bool) (DiffResult, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")
	contract.Assert(news != nil)
	contract.Assert(olds != nil)

	label := fmt.Sprintf("%s.Diff(%s,%s)", p.label(), urn, id)
	glog.V(7).Infof("%s: executing (#olds=%d,#news=%d)", label, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{
		Label: fmt.Sprintf("%s.olds", label), ElideAssetContents: true, KeepUnknowns: allowUnknowns})
	if err != nil {
		return DiffResult{}, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{Label: fmt.Sprintf("%s.news", label),
		KeepUnknowns: allowUnknowns})
	if err != nil {
		return DiffResult{}, err
	}

	resp, err := p.client.Diff(p.ctx.Request(), &pulumirpc.DiffRequest{
		Id:   string(id),
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		glog.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return DiffResult{}, rpcError
	}

	var replaces []resource.PropertyKey
	for _, replace := range resp.GetReplaces() {
		replaces = append(replaces, resource.PropertyKey(replace))
	}
	var stables []resource.PropertyKey
	for _, stable := range resp.GetStables() {
		stables = append(stables, resource.PropertyKey(stable))
	}
	changes := resp.GetChanges()
	deleteBeforeReplace := resp.GetDeleteBeforeReplace()
	glog.V(7).Infof("%s success: changes=%d #replaces=%d #stables=%d delbefrepl=%v",
		label, changes, len(replaces), len(stables), deleteBeforeReplace)
	return DiffResult{
		Changes:             DiffChanges(changes),
		ReplaceKeys:         replaces,
		StableKeys:          stables,
		DeleteBeforeReplace: deleteBeforeReplace,
	}, nil
}

// Create allocates a new instance of the provided resource and assigns its unique resource.ID and outputs afterwards.
func (p *provider) Create(urn resource.URN, props resource.PropertyMap) (resource.ID,
	resource.PropertyMap, resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(props != nil)

	label := fmt.Sprintf("%s.Create(%s)", p.label(), urn)
	glog.V(7).Infof("%s executing (#props=%v)", label, len(props))

	mprops, err := MarshalProperties(props, MarshalOptions{Label: fmt.Sprintf("%s.inputs", label)})
	if err != nil {
		return "", nil, resource.StatusOK, err
	}

	resp, err := p.client.Create(p.ctx.Request(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: mprops,
	})
	if err != nil {
		resourceStatus, rpcErr := resourceStateAndError(err)
		glog.V(7).Infof("%s failed: err=%v", label, rpcErr)
		return "", nil, resourceStatus, rpcErr
	}

	id := resource.ID(resp.GetId())
	if id == "" {
		return "", nil, resource.StatusUnknown,
			errors.Errorf("plugin for package '%v' returned empty resource.ID from create '%v'", p.pkg, urn)
	}

	outs, err := UnmarshalProperties(resp.GetProperties(), MarshalOptions{
		Label: fmt.Sprintf("%s.outputs", label), RejectUnknowns: true})
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}

	glog.V(7).Infof("%s success: id=%s; #outs=%d", label, id, len(outs))
	return id, outs, resource.StatusOK, nil
}

// read the current live state associated with a resource.  enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource id, but may also include some properties.
func (p *provider) Read(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.PropertyMap, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")

	label := fmt.Sprintf("%s.Read(%s,%s)", p.label(), id, urn)
	glog.V(7).Infof("%s executing (#props=%v)", label, len(props))

	// Marshal the input state so we can perform the RPC.
	marshaled, err := MarshalProperties(props, MarshalOptions{Label: label, ElideAssetContents: true})
	if err != nil {
		return nil, err
	}

	// Now issue the read request over RPC, blocking until it finished.
	resp, err := p.client.Read(p.ctx.Request(), &pulumirpc.ReadRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: marshaled,
	})
	if err != nil {
		glog.V(7).Infof("%s failed: %v", label, err)
		return nil, err
	}

	// If the resource was missing, simply return a nil property map.
	readID := resp.GetId()
	if readID == "" {
		return nil, nil
	} else if readID != string(id) {
		return nil, errors.Errorf(
			"reading resource %s yielded an unexpected ID; expected %s, got %s", urn, id, readID)
	}

	// Finally, unmarshal the resulting state properties and return them.
	results, err := UnmarshalProperties(resp.GetProperties(), MarshalOptions{
		Label: fmt.Sprintf("%s.outputs", label), RejectUnknowns: true})
	if err != nil {
		return nil, err
	}

	glog.V(7).Infof("%s success; #outs=%d", label, len(results))
	return results, nil
}

// Update updates an existing resource with new values.
func (p *provider) Update(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")
	contract.Assert(news != nil)
	contract.Assert(olds != nil)

	label := fmt.Sprintf("%s.Update(%s,%s)", p.label(), id, urn)
	glog.V(7).Infof("%s executing (#olds=%v,#news=%v)", label, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{
		Label: fmt.Sprintf("%s.olds", label), ElideAssetContents: true})
	if err != nil {
		return nil, resource.StatusOK, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{Label: fmt.Sprintf("%s.news", label)})
	if err != nil {
		return nil, resource.StatusOK, err
	}

	req := &pulumirpc.UpdateRequest{
		Id:   string(id),
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
	}

	resp, err := p.client.Update(p.ctx.Request(), req)
	if err != nil {
		resourceStatus, rpcErr := resourceStateAndError(err)
		glog.V(7).Infof("%s failed: %v", label, rpcErr)
		return nil, resourceStatus, rpcErr
	}

	outs, err := UnmarshalProperties(resp.GetProperties(), MarshalOptions{
		Label: fmt.Sprintf("%s.outputs", label), RejectUnknowns: true})
	if err != nil {
		return nil, resource.StatusUnknown, err
	}

	glog.V(7).Infof("%s success; #outs=%d", label, len(outs))
	return outs, resource.StatusOK, nil
}

// Delete tears down an existing resource.
func (p *provider) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")

	label := fmt.Sprintf("%s.Delete(%s,%s)", p.label(), urn, id)
	glog.V(7).Infof("%s executing (#props=%d)", label, len(props))

	mprops, err := MarshalProperties(props, MarshalOptions{Label: label, ElideAssetContents: true})
	if err != nil {
		return resource.StatusOK, err
	}

	req := &pulumirpc.DeleteRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: mprops,
	}

	if _, err := p.client.Delete(p.ctx.Request(), req); err != nil {
		resourceStatus, rpcErr := resourceStateAndError(err)
		glog.V(7).Infof("%s failed: %v", label, rpcErr)
		return resourceStatus, rpcErr
	}

	glog.V(7).Infof("%s success", label)
	return resource.StatusOK, nil
}

// Invoke dynamically executes a built-in function in the provider.
func (p *provider) Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap,
	[]CheckFailure, error) {
	contract.Assert(tok != "")

	label := fmt.Sprintf("%s.Invoke(%s)", p.label(), tok)
	glog.V(7).Infof("%s executing (#args=%d)", label, len(args))

	margs, err := MarshalProperties(args, MarshalOptions{Label: fmt.Sprintf("%s.args", label)})
	if err != nil {
		return nil, nil, err
	}

	resp, err := p.client.Invoke(p.ctx.Request(), &pulumirpc.InvokeRequest{Tok: string(tok), Args: margs})
	if err != nil {
		glog.V(7).Infof("%s failed: %v", label, err)
		return nil, nil, err
	}

	// Unmarshal any return values.
	ret, err := UnmarshalProperties(resp.GetReturn(), MarshalOptions{
		Label: fmt.Sprintf("%s.returns", label), RejectUnknowns: true})
	if err != nil {
		return nil, nil, err
	}

	// And now any properties that failed verification.
	var failures []CheckFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	glog.V(7).Infof("%s success (#ret=%d,#failures=%d) success", label, len(ret), len(failures))
	return ret, failures, nil
}

// GetPluginInfo returns this plugin's information.
func (p *provider) GetPluginInfo() (workspace.PluginInfo, error) {
	label := fmt.Sprintf("%s.GetPluginInfo()", p.label())
	glog.V(7).Infof("%s executing", label)
	resp, err := p.client.GetPluginInfo(p.ctx.Request(), &pbempty.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		glog.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
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

	return workspace.PluginInfo{
		Name:    string(p.pkg),
		Path:    p.plug.Bin,
		Kind:    workspace.ResourcePlugin,
		Version: version,
	}, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (p *provider) Close() error {
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
func resourceStateAndError(err error) (resource.Status, error) {
	rpcError := rpcerror.Convert(err)
	glog.V(8).Infof("provider received rpc error `%s`: `%s`", rpcError.Code(), rpcError.Message())
	switch rpcError.Code() {
	case codes.Internal, codes.DataLoss, codes.Unknown:
		glog.V(8).Infof("rpc error kind `%s` may not be recoverable", rpcError.Code())
		return resource.StatusUnknown, rpcError
	}

	glog.V(8).Infof("rpc error kind `%s` is well-understood and recoverable", rpcError.Code())
	return resource.StatusOK, rpcError
}
