// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi-fabric/pkg/resource"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
	lumirpc "github.com/pulumi/pulumi-fabric/sdk/proto/go"
)

const ProviderPluginPrefix = "pulumi-provider-"

// provider reflects a resource plugin, loaded dynamically for a single package.
type provider struct {
	ctx    *Context
	pkg    tokens.Package
	plug   *plugin
	client lumirpc.ResourceProviderClient
}

// NewProvider attempts to bind to a given package's resource plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewProvider(host Host, ctx *Context, pkg tokens.Package) (Provider, error) {
	// Go ahead and attempt to load the plugin from the PATH.
	srvexe := ProviderPluginPrefix + strings.Replace(string(pkg), tokens.QNameDelimiter, "_", -1)
	plug, err := newPlugin(ctx, srvexe, fmt.Sprintf("resource[%v]", pkg), []string{host.ServerAddr()})
	if err != nil {
		return nil, err
	} else if plug == nil {
		return nil, nil
	}

	return &provider{
		ctx:    ctx,
		pkg:    pkg,
		plug:   plug,
		client: lumirpc.NewResourceProviderClient(plug.Conn),
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
	_, err := p.client.Configure(p.ctx.Request(), &lumirpc.ConfigureRequest{Variables: config})
	if err != nil {
		glog.V(7).Infof("resource[%v].Configure(#vars=%v,...) failed: err=%v", p.pkg, len(vars), err)
		return err
	}
	return nil
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *provider) Check(urn resource.URN, props resource.PropertyMap) (resource.PropertyMap, []CheckFailure, error) {
	glog.V(7).Infof("resource[%v].Check(urn=%v,#props=%v) executing", p.pkg, urn, len(props))
	mprops, err := MarshalProperties(props, MarshalOptions{AllowUnknowns: true})
	if err != nil {
		return nil, nil, err
	}

	resp, err := p.client.Check(p.ctx.Request(), &lumirpc.CheckRequest{
		Urn:        string(urn),
		Properties: mprops,
	})
	if err != nil {
		glog.V(7).Infof("resource[%v].Check(urn=%v,...) failed: err=%v", p.pkg, urn, err)
		return nil, nil, err
	}

	// Unmarshal any defaults.
	var defaults resource.PropertyMap
	if defs := resp.GetDefaults(); defs != nil {
		defaults, err = UnmarshalProperties(defs, MarshalOptions{AllowUnknowns: true})
		if err != nil {
			return nil, nil, err
		}
	}

	// And now any properties that failed verification.
	var failures []CheckFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{resource.PropertyKey(failure.Property), failure.Reason})
	}

	glog.V(7).Infof("resource[%v].Check(urn=%v,...) success: defs=#%v failures=#%v",
		p.pkg, urn, len(defaults), len(failures))
	return defaults, failures, nil
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

	molds, err := MarshalProperties(olds, MarshalOptions{})
	if err != nil {
		return DiffResult{}, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{AllowUnknowns: true})
	if err != nil {
		return DiffResult{}, err
	}

	resp, err := p.client.Diff(p.ctx.Request(), &lumirpc.DiffRequest{
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
	glog.V(7).Infof("resource[%v].Update(id=%v,urn=%v,...) success: #replaces=%v", p.pkg, id, urn, len(replaces))
	return DiffResult{ReplaceKeys: replaces}, nil
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

	resp, err := p.client.Create(p.ctx.Request(), &lumirpc.CreateRequest{
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

	molds, err := MarshalProperties(olds, MarshalOptions{})
	if err != nil {
		return nil, resource.StatusOK, err
	}
	mnews, err := MarshalProperties(news, MarshalOptions{})
	if err != nil {
		return nil, resource.StatusOK, err
	}

	req := &lumirpc.UpdateRequest{
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

	mprops, err := MarshalProperties(props, MarshalOptions{})
	if err != nil {
		return resource.StatusOK, err
	}

	glog.V(7).Infof("resource[%v].Delete(id=%v,urn=%v) executing", p.pkg, id, urn)
	req := &lumirpc.DeleteRequest{
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

// Close tears down the underlying plugin RPC connection and process.
func (p *provider) Close() error {
	return p.plug.Close()
}
