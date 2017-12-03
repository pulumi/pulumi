// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

const ProviderPluginPrefix = "pulumi-provider-"

// provider reflects a resource plugin, loaded dynamically for a single package.
type provider struct {
	ctx    *Context
	pkg    tokens.Package
	plug   *plugin
	client pulumirpc.ResourceProviderClient
}

// NewProvider attempts to bind to a given package's resource plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewProvider(host Host, ctx *Context, pkg tokens.Package) (Provider, error) {
	// Go ahead and attempt to load the plugin from the PATH.
	srvexe := ProviderPluginPrefix + strings.Replace(string(pkg), tokens.QNameDelimiter, "_", -1)
	plug, err := newPlugin(ctx, srvexe, fmt.Sprintf("%v (resource)", pkg), []string{host.ServerAddr()})
	if err != nil {
		return nil, err
	} else if plug == nil {
		return nil, nil
	}

	return &provider{
		ctx:    ctx,
		pkg:    pkg,
		plug:   plug,
		client: pulumirpc.NewResourceProviderClient(plug.Conn),
	}, nil
}

func (p *provider) Pkg() tokens.Package { return p.pkg }

// Configure configures the resource provider with "globals" that control its behavior.
func (p *provider) Configure(vars map[tokens.ModuleMember]string) error {
	glog.V(7).Infof("resource[%v].Configure(#vars=%v) executing", p.pkg, len(vars))
	config := make(map[string]string)
	for k, v := range vars {
		config[string(k)] = v
	}
	_, err := p.client.Configure(p.ctx.Request(), &pulumirpc.ConfigureRequest{Variables: config})
	if err != nil {
		glog.V(7).Infof("resource[%v].Configure(#vars=%v,...) failed: err=%v", p.pkg, len(vars), err)
		return err
	}
	return nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *provider) Check(urn resource.URN,
	olds, news resource.PropertyMap) (resource.PropertyMap, []CheckFailure, error) {

	glog.V(7).Infof("resource[%v].Check(urn=%v,#olds=%v,#news=%v) executing", p.pkg, urn, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, nil, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, nil, err
	}

	resp, err := p.client.Check(p.ctx.Request(), &pulumirpc.CheckRequest{
		Urn:  string(urn),
		Olds: molds,
		News: mnews,
	})
	if err != nil {
		glog.V(7).Infof("resource[%v].Check(urn=%v,...) failed: err=%v", p.pkg, urn, err)
		return nil, nil, err
	}

	// Unmarshal the provider inputs.
	var inputs resource.PropertyMap
	if ins := resp.GetInputs(); ins != nil {
		inputs, err = UnmarshalProperties(ins, MarshalOptions{KeepUnknowns: true})
		if err != nil {
			return nil, nil, err
		}
	}

	// And now any properties that failed verification.
	var failures []CheckFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	glog.V(7).Infof("resource[%v].Check(urn=%v,...) success: inputs=#%v failures=#%v",
		p.pkg, urn, len(inputs), len(failures))
	return inputs, failures, nil
}

// Diff checks what impacts a hypothetical update will have on the resource's properties.
func (p *provider) Diff(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (DiffResult, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")
	contract.Assert(news != nil)
	contract.Assert(olds != nil)
	glog.V(7).Infof("resource[%v].Diff(id=%v,urn=%v,#olds=%v,#news=%v) executing",
		p.pkg, id, urn, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{ElideAssetContents: true})
	if err != nil {
		return DiffResult{}, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{KeepUnknowns: true})
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
		glog.V(7).Infof("resource[%v].Diff(id=%v,urn=%v,...) failed: %v", p.pkg, id, urn, err)
		return DiffResult{}, err
	}

	var replaces []resource.PropertyKey
	for _, replace := range resp.GetReplaces() {
		replaces = append(replaces, resource.PropertyKey(replace))
	}
	var stables []resource.PropertyKey
	for _, stable := range resp.GetStables() {
		stables = append(stables, resource.PropertyKey(stable))
	}
	glog.V(7).Infof("resource[%v].Update(id=%v,urn=%v,...) success: #replaces=%v #stables=%v",
		p.pkg, id, urn, len(replaces), len(stables))
	return DiffResult{
		ReplaceKeys: replaces,
		StableKeys:  stables,
	}, nil
}

// Create allocates a new instance of the provided resource and assigns its unique resource.ID and outputs afterwards.
func (p *provider) Create(urn resource.URN, props resource.PropertyMap) (resource.ID,
	resource.PropertyMap, resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(props != nil)
	glog.V(7).Infof("resource[%v].Create(urn=%v,#props=%v) executing", p.pkg, urn, len(props))

	mprops, err := MarshalProperties(props, MarshalOptions{})
	if err != nil {
		return "", nil, resource.StatusOK, err
	}

	resp, err := p.client.Create(p.ctx.Request(), &pulumirpc.CreateRequest{
		Urn:        string(urn),
		Properties: mprops,
	})
	if err != nil {
		glog.V(7).Infof("resource[%v].Create(urn=%v,...) failed: err=%v", p.pkg, urn, err)
		return "", nil, resource.StatusUnknown, err
	}

	id := resource.ID(resp.GetId())
	if id == "" {
		return "", nil, resource.StatusUnknown,
			errors.Errorf("plugin for package '%v' returned empty resource.ID from create '%v'", p.pkg, urn)
	}

	outs, err := UnmarshalProperties(resp.GetProperties(), MarshalOptions{})
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}

	glog.V(7).Infof("resource[%v].Create(urn=%v,...) success: id=%v; #outs=%v", p.pkg, urn, id, len(outs))
	return id, outs, resource.StatusOK, nil
}

// Update updates an existing resource with new values.
func (p *provider) Update(urn resource.URN, id resource.ID,
	olds resource.PropertyMap, news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")
	contract.Assert(news != nil)
	contract.Assert(olds != nil)
	glog.V(7).Infof("resource[%v].Update(id=%v,urn=%v,#olds=%v,#news=%v) executing",
		p.pkg, id, urn, len(olds), len(news))

	molds, err := MarshalProperties(olds, MarshalOptions{ElideAssetContents: true})
	if err != nil {
		return nil, resource.StatusOK, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{})
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
		glog.V(7).Infof("resource[%v].Update(id=%v,urn=%v,...) failed: %v", p.pkg, id, urn, err)
		return nil, resource.StatusUnknown, err
	}

	outs, err := UnmarshalProperties(resp.GetProperties(), MarshalOptions{})
	if err != nil {
		return nil, resource.StatusUnknown, err
	}

	glog.V(7).Infof("resource[%v].Update(id=%v,urn=%v,...) success; #out=%v", p.pkg, id, urn, len(outs))
	return outs, resource.StatusOK, nil
}

// Delete tears down an existing resource.
func (p *provider) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	contract.Assert(urn != "")
	contract.Assert(id != "")

	mprops, err := MarshalProperties(props, MarshalOptions{ElideAssetContents: true})
	if err != nil {
		return resource.StatusOK, err
	}

	glog.V(7).Infof("resource[%v].Delete(id=%v,urn=%v) executing", p.pkg, id, urn)
	req := &pulumirpc.DeleteRequest{
		Id:         string(id),
		Urn:        string(urn),
		Properties: mprops,
	}

	if _, err := p.client.Delete(p.ctx.Request(), req); err != nil {
		glog.V(7).Infof("resource[%v].Delete(id=%v,urn=%v) failed: %v", p.pkg, id, urn, err)
		return resource.StatusUnknown, err
	}

	glog.V(7).Infof("resource[%v].Delete(id=%v,urn=%v) success", p.pkg, id, urn)
	return resource.StatusOK, nil
}

// Invoke dynamically executes a built-in function in the provider.
func (p *provider) Invoke(tok tokens.ModuleMember, args resource.PropertyMap) (resource.PropertyMap,
	[]CheckFailure, error) {
	contract.Assert(tok != "")

	margs, err := MarshalProperties(args, MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, nil, err
	}

	glog.V(7).Infof("resource[%v].Invoke(tok=%v,#args=%v) executing", tok, len(args))
	resp, err := p.client.Invoke(p.ctx.Request(), &pulumirpc.InvokeRequest{Tok: string(tok), Args: margs})
	if err != nil {
		glog.V(7).Infof("resource[%v].Invoke(tok=%v,#args=%v) failed: %v", p.pkg, tok, len(args), err)
		return nil, nil, err
	}

	// Unmarshal any return values.
	ret, err := UnmarshalProperties(resp.GetReturn(), MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, nil, err
	}

	// And now any properties that failed verification.
	var failures []CheckFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	glog.V(7).Infof("resource[%v].Invoke(tok=%v,#args=%v,#ret=%v,#failures=%v) success",
		p.pkg, tok, len(args), len(ret), len(failures))
	return ret, failures, nil
}

// GetPluginInfo returns this plugin's information.
func (p *provider) GetPluginInfo() (Info, error) {
	glog.V(7).Infof("resource[%v].GetPluginInfo() executing", p.pkg)
	resp, err := p.client.GetPluginInfo(p.ctx.Request(), &pbempty.Empty{})
	if err != nil {
		glog.V(7).Infof("resource[%v].GetPluginInfo() failed: err=%v", p.pkg, err)
		return Info{}, err
	}
	return Info{
		Name:    p.plug.Bin,
		Type:    ResourceType,
		Version: resp.Version,
	}, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (p *provider) Close() error {
	return p.plug.Close()
}
