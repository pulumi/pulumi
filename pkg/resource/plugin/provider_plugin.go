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
	"fmt"
	"strings"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	_struct "github.com/golang/protobuf/ptypes/struct"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/pkg/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// provider reflects a resource plugin, loaded dynamically for a single package.
type provider struct {
	ctx       *Context                         // a plugin context for caching, etc.
	pkg       tokens.Package                   // the Pulumi package containing this provider's resources.
	plug      *plugin                          // the actual plugin process wrapper.
	clientRaw pulumirpc.ResourceProviderClient // the raw provider client; usually unsafe to use directly.
	cfgerr    error                            // non-nil if a configure call fails.
	cfgknown  bool                             // true if all configuration values are known.
	cfgdone   chan bool                        // closed when configuration has completed.
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
			Kind:    workspace.ResourcePlugin,
			Name:    string(pkg),
			Version: version,
		})
	}

	plug, err := newPlugin(ctx, path, fmt.Sprintf("%v (resource)", pkg), []string{host.ServerAddr()})
	if err != nil {
		return nil, err
	}
	contract.Assertf(plug != nil, "unexpected nil resource plugin for %s", pkg)

	return &provider{
		ctx:       ctx,
		pkg:       pkg,
		plug:      plug,
		clientRaw: pulumirpc.NewResourceProviderClient(plug.Conn),
		cfgdone:   make(chan bool),
	}, nil
}

func (p *provider) Pkg() tokens.Package { return p.pkg }

// label returns a base label for tracing functions.
func (p *provider) label() string {
	return fmt.Sprintf("Provider[%s, %p]", p.pkg, p)
}

// CheckConfig validates the configuration for this resource provider.
func (p *provider) CheckConfig(olds, news resource.PropertyMap) (resource.PropertyMap, []CheckFailure, error) {
	// Ensure that all config values are strings or unknowns.
	var failures []CheckFailure
	for k, v := range news {
		if !v.IsString() && !v.IsComputed() {
			failures = append(failures, CheckFailure{
				Property: k,
				Reason:   "provider property values must be strings",
			})
		}
	}
	if len(failures) != 0 {
		return nil, failures, nil
	}

	// If all config values check out, simply return the new values.
	return news, nil, nil
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *provider) DiffConfig(olds, news resource.PropertyMap) (DiffResult, error) {
	// There are two interesting scenarios with the present gRPC interface:
	// 1. Configuration differences in which all properties are known
	// 2. Configuration differences in which some new property is unknown.
	//
	// In both cases, we return a diff result that indicates that the provider _should not_ be replaced. Although this
	// decision is not conservative--indeed, the conservative decision would be to always require replacement of a
	// provider if any input has changed--we believe that it results in the best possible user experience for providers
	// that do not implement DiffConfig functionality. If we took the conservative route here, any change to a
	// provider's configuration (no matter how inconsequential) would cause all of its resources to be replaced. This
	// is clearly a bad experience, and differs from how things worked prior to first-class providers.
	return DiffResult{Changes: DiffUnknown, ReplaceKeys: nil}, nil
}

// getClient returns the client, and ensures that the target provider has been configured.  This just makes it safer
// to use without forgetting to call ensureConfigured manually.
func (p *provider) getClient() (pulumirpc.ResourceProviderClient, error) {
	if err := p.ensureConfigured(); err != nil {
		return nil, err
	}
	return p.clientRaw, nil
}

// ensureConfigured blocks waiting for the plugin to be configured.  To improve parallelism, all Configure RPCs
// occur in parallel, and we await the completion of them at the last possible moment.  This does mean, however, that
// we might discover failures later than we would have otherwise, but the caller of ensureConfigured will get them.
func (p *provider) ensureConfigured() error {
	<-p.cfgdone
	return p.cfgerr
}

// Configure configures the resource provider with "globals" that control its behavior.
func (p *provider) Configure(inputs resource.PropertyMap) error {
	label := fmt.Sprintf("%s.Configure()", p.label())
	logging.V(7).Infof("%s executing (#vars=%d)", label, len(inputs))

	// Convert the inputs to a config map. If any are unknown, do not configure the underlying plugin: instead, leavce
	// the cfgknown bit unset and carry on.
	config := make(map[string]string)
	for k, v := range inputs {
		if k == "version" {
			continue
		}
		switch {
		case v.IsComputed():
			p.cfgknown = false
			close(p.cfgdone)
			return nil
		case v.IsString():
			// Pass the older spelling of a configuration key across the RPC interface, for now, to support
			// providers which are on the older plan.
			config[string(p.Pkg())+":config:"+string(k)] = v.StringValue()
		default:
			p.cfgerr = errors.Errorf("provider property values must be strings; '%v' is a %v", k, v.TypeString())
			close(p.cfgdone)
			return p.cfgerr
		}
	}

	// Spawn the configure to happen in parallel.  This ensures that we remain responsive elsewhere that might
	// want to make forward progress, even as the configure call is happening.
	go func() {
		_, err := p.clientRaw.Configure(p.ctx.Request(), &pulumirpc.ConfigureRequest{Variables: config})
		if err != nil {
			rpcError := rpcerror.Convert(err)
			logging.V(7).Infof("%s failed: err=%v", label, rpcError.Message())
			err = createConfigureError(rpcError)
		}
		// Acquire the lock, publish the results, and notify any waiters.
		p.cfgknown, p.cfgerr = true, err
		close(p.cfgdone)
	}()

	return nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *provider) Check(urn resource.URN,
	olds, news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []CheckFailure, error) {
	label := fmt.Sprintf("%s.Check(%s)", p.label(), urn)
	logging.V(7).Infof("%s executing (#olds=%d,#news=%d", label, len(olds), len(news))

	// Get the RPC client and ensure it's configured.
	client, err := p.getClient()
	if err != nil {
		return nil, nil, err
	}

	// If the configuration for this provider was not fully known--e.g. if we are doing a preview and some input
	// property was sourced from another resource's output properties--don't call into the underlying provider.
	if !p.cfgknown {
		return news, nil, nil
	}

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

	resp, err := client.Check(p.ctx.Request(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
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

	logging.V(7).Infof("%s success: inputs=#%d failures=#%d", label, len(inputs), len(failures))
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
	logging.V(7).Infof("%s: executing (#olds=%d,#news=%d)", label, len(olds), len(news))

	// Get the RPC client and ensure it's configured.
	client, err := p.getClient()
	if err != nil {
		return DiffResult{}, err
	}

	// If the configuration for this provider was not fully known--e.g. if we are doing a preview and some input
	// property was sourced from another resource's output properties--don't call into the underlying provider.
	// Instead, indicate that the diff is unavailable and write a message
	if !p.cfgknown {
		logging.V(7).Infof("%s: cannot diff due to unknown config", label)
		const message = "The provider for this resource has inputs that are not known during preview.\n" +
			"This preview may not correctly represent the changes that will be applied during an update."
		return DiffResult{}, DiffUnavailable(message)
	}

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

	resp, err := client.Diff(p.ctx.Request(), &pulumirpc.DiffRequest{
		Id:   string(id),
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
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
	logging.V(7).Infof("%s success: changes=%d #replaces=%v #stables=%v delbefrepl=%v",
		label, changes, replaces, stables, deleteBeforeReplace)
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
	logging.V(7).Infof("%s executing (#props=%v)", label, len(props))

	mprops, err := MarshalProperties(props, MarshalOptions{Label: fmt.Sprintf("%s.inputs", label)})
	if err != nil {
		return "", nil, resource.StatusOK, err
	}

	// Get the RPC client and ensure it's configured.
	client, err := p.getClient()
	if err != nil {
		return "", nil, resource.StatusOK, err
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assert(p.cfgknown)

	var id resource.ID
	var liveObject *_struct.Struct
	var resourceError error
	var resourceStatus = resource.StatusOK
	resp, err := client.Create(p.ctx.Request(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: mprops,
	})
	if err != nil {
		resourceStatus, id, liveObject, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, resourceError)

		if resourceStatus != resource.StatusPartialFailure {
			return "", nil, resourceStatus, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		id = resource.ID(resp.GetId())
		liveObject = resp.GetProperties()
	}

	if id == "" {
		return "", nil, resource.StatusUnknown,
			errors.Errorf("plugin for package '%v' returned empty resource.ID from create '%v'", p.pkg, urn)
	}

	outs, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label: fmt.Sprintf("%s.outputs", label), RejectUnknowns: true})
	if err != nil {
		return "", nil, resourceStatus, err
	}

	logging.V(7).Infof("%s success: id=%s; #outs=%d", label, id, len(outs))
	if resourceError == nil {
		return id, outs, resourceStatus, nil
	}
	return id, outs, resourceStatus, resourceError
}

// read the current live state associated with a resource.  enough state must be include in the inputs to uniquely
// identify the resource; this is typically just the resource id, but may also include some properties.
func (p *provider) Read(
	urn resource.URN, id resource.ID, props resource.PropertyMap,
) (resource.PropertyMap, resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")

	label := fmt.Sprintf("%s.Read(%s,%s)", p.label(), id, urn)
	logging.V(7).Infof("%s executing (#props=%v)", label, len(props))

	// Get the RPC client and ensure it's configured.
	client, err := p.getClient()
	if err != nil {
		return nil, resource.StatusUnknown, err
	}

	// If the provider is not fully configured, return an empty bag.
	if !p.cfgknown {
		return resource.PropertyMap{}, resource.StatusUnknown, nil
	}

	// Marshal the input state so we can perform the RPC.
	marshaled, err := MarshalProperties(props, MarshalOptions{Label: label, ElideAssetContents: true})
	if err != nil {
		return nil, resource.StatusUnknown, err
	}

	// Now issue the read request over RPC, blocking until it finished.
	var readID resource.ID
	var liveObject *_struct.Struct
	var resourceError error
	var resourceStatus = resource.StatusOK
	resp, err := client.Read(p.ctx.Request(), &pulumirpc.ReadRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: marshaled,
	})
	if err != nil {
		resourceStatus, readID, liveObject, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, err)

		if resourceStatus != resource.StatusPartialFailure {
			return nil, resourceStatus, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		readID = resource.ID(resp.GetId())
		liveObject = resp.GetProperties()
	}

	// If the resource was missing, simply return a nil property map.
	if string(readID) == "" {
		return nil, resourceStatus, nil
	} else if readID != id {
		return nil, resourceStatus, errors.Errorf(
			"reading resource %s yielded an unexpected ID; expected %s, got %s", urn, id, readID)
	}

	// Finally, unmarshal the resulting state properties and return them.
	results, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label: fmt.Sprintf("%s.outputs", label), RejectUnknowns: true})
	if err != nil {
		return nil, resourceStatus, err
	}

	logging.V(7).Infof("%s success; #outs=%d", label, len(results))
	return results, resourceStatus, resourceError
}

// Update updates an existing resource with new values.
func (p *provider) Update(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")
	contract.Assert(news != nil)
	contract.Assert(olds != nil)

	label := fmt.Sprintf("%s.Update(%s,%s)", p.label(), id, urn)
	logging.V(7).Infof("%s executing (#olds=%v,#news=%v)", label, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{
		Label: fmt.Sprintf("%s.olds", label), ElideAssetContents: true})
	if err != nil {
		return nil, resource.StatusOK, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{Label: fmt.Sprintf("%s.news", label)})
	if err != nil {
		return nil, resource.StatusOK, err
	}

	// Get the RPC client and ensure it's configured.
	client, err := p.getClient()
	if err != nil {
		return nil, resource.StatusOK, err
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assert(p.cfgknown)

	var liveObject *_struct.Struct
	var resourceError error
	var resourceStatus = resource.StatusOK
	resp, err := client.Update(p.ctx.Request(), &pulumirpc.UpdateRequest{
		Id:   string(id),
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
	})
	if err != nil {
		resourceStatus, _, liveObject, resourceError = parseError(err)
		logging.V(7).Infof("%s failed: %v", label, resourceError)

		if resourceStatus != resource.StatusPartialFailure {
			return nil, resourceStatus, resourceError
		}
		// Else it's a `StatusPartialFailure`.
	} else {
		liveObject = resp.GetProperties()
	}

	outs, err := UnmarshalProperties(liveObject, MarshalOptions{
		Label: fmt.Sprintf("%s.outputs", label), RejectUnknowns: true})
	if err != nil {
		return nil, resourceStatus, err
	}

	logging.V(7).Infof("%s success; #outs=%d", label, len(outs))
	if resourceError == nil {
		return outs, resourceStatus, nil
	}
	return outs, resourceStatus, resourceError
}

// Delete tears down an existing resource.
func (p *provider) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")

	label := fmt.Sprintf("%s.Delete(%s,%s)", p.label(), urn, id)
	logging.V(7).Infof("%s executing (#props=%d)", label, len(props))

	mprops, err := MarshalProperties(props, MarshalOptions{Label: label, ElideAssetContents: true})
	if err != nil {
		return resource.StatusOK, err
	}

	// Get the RPC client and ensure it's configured.
	client, err := p.getClient()
	if err != nil {
		return resource.StatusOK, err
	}

	// We should only be calling {Create,Update,Delete} if the provider is fully configured.
	contract.Assert(p.cfgknown)

	if _, err := client.Delete(p.ctx.Request(), &pulumirpc.DeleteRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: mprops,
	}); err != nil {
		resourceStatus, rpcErr := resourceStateAndError(err)
		logging.V(7).Infof("%s failed: %v", label, rpcErr)
		return resourceStatus, rpcErr
	}

	logging.V(7).Infof("%s success", label)
	return resource.StatusOK, nil
}

// Invoke dynamically executes a built-in function in the provider.
func (p *provider) Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap,
	[]CheckFailure, error) {
	contract.Assert(tok != "")

	label := fmt.Sprintf("%s.Invoke(%s)", p.label(), tok)
	logging.V(7).Infof("%s executing (#args=%d)", label, len(args))

	// Get the RPC client and ensure it's configured.
	client, err := p.getClient()
	if err != nil {
		return nil, nil, err
	}

	// If the provider is not fully configured, return an empty property map.
	if !p.cfgknown {
		return resource.PropertyMap{}, nil, nil
	}

	margs, err := MarshalProperties(args, MarshalOptions{Label: fmt.Sprintf("%s.args", label)})
	if err != nil {
		return nil, nil, err
	}

	resp, err := client.Invoke(p.ctx.Request(), &pulumirpc.InvokeRequest{Tok: string(tok), Args: margs})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: %v", label, rpcError.Message())
		return nil, nil, rpcError
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

	logging.V(7).Infof("%s success (#ret=%d,#failures=%d) success", label, len(ret), len(failures))
	return ret, failures, nil
}

// GetPluginInfo returns this plugin's information.
func (p *provider) GetPluginInfo() (workspace.PluginInfo, error) {
	label := fmt.Sprintf("%s.GetPluginInfo()", p.label())
	logging.V(7).Infof("%s executing", label)

	// Calling GetPluginInfo happens immediately after loading, and does not require configuration to proceed.
	// Thus, we access the clientRaw property, rather than calling getClient.
	resp, err := p.clientRaw.GetPluginInfo(p.ctx.Request(), &pbempty.Empty{})
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

	return workspace.PluginInfo{
		Name:    string(p.pkg),
		Path:    p.plug.Bin,
		Kind:    workspace.ResourcePlugin,
		Version: version,
	}, nil
}

func (p *provider) SignalCancellation() error {
	_, err := p.clientRaw.Cancel(p.ctx.Request(), &pbempty.Empty{})
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
	resourceStatus resource.Status, id resource.ID, liveObject *_struct.Struct, resourceErr error,
) {
	var responseErr *rpcerror.Error
	resourceStatus, responseErr = resourceStateAndError(err)
	contract.Assert(responseErr != nil)

	// If resource was successfully created but failed to initialize, the error will be packed
	// with the live properties of the object.
	resourceErr = responseErr
	for _, detail := range responseErr.Details() {
		if initErr, ok := detail.(*pulumirpc.ErrorResourceInitFailed); ok {
			id = resource.ID(initErr.GetId())
			liveObject = initErr.GetProperties()
			resourceStatus = resource.StatusPartialFailure
			resourceErr = &InitError{Reasons: initErr.Reasons}
			break
		}
	}

	return resourceStatus, id, liveObject, resourceErr
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
