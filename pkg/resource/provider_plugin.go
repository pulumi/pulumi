// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/workspace"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
)

const providerPrefix = "coco-resource"

// provider reflects a resource plugin, loaded dynamically for a single package.
type provider struct {
	ctx    *Context
	pkg    tokens.Package
	plug   *plugin
	client cocorpc.ResourceProviderClient
}

// NewProvider attempts to bind to a given package's resource plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewProvider(ctx *Context, pkg tokens.Package) (Provider, error) {
	// Setup the search paths; first, the naked name (found on the PATH); next, the fully qualified name.
	srvexe := providerPrefix + "-" + strings.Replace(string(pkg), tokens.QNameDelimiter, "_", -1)
	paths := []string{
		srvexe, // naked PATH.
		filepath.Join(
			workspace.InstallRoot(), workspace.InstallRootLibdir, string(pkg), srvexe), // qualified name.
	}

	// Now go ahead and attempt to load the plugin.
	plug, err := newPlugin(ctx, paths, fmt.Sprintf("resource[%v]", pkg))
	if err != nil {
		return nil, err
	}

	return &provider{
		ctx:    ctx,
		pkg:    pkg,
		plug:   plug,
		client: cocorpc.NewResourceProviderClient(plug.Conn),
	}, nil
}

func (p *provider) Pkg() tokens.Package { return p.pkg }

// Check validates that the given property bag is valid for a resource of the given type.
func (p *provider) Check(t tokens.Type, props PropertyMap) ([]CheckFailure, error) {
	glog.V(7).Infof("resource[%v].Check(t=%v,#props=%v) executing", p.pkg, t, len(props))
	req := &cocorpc.CheckRequest{
		Type: string(t),
		Properties: MarshalProperties(p.ctx, props, MarshalOptions{
			PermitOlds: true, // permit old URNs, since this is pre-update.
			RawURNs:    true, // often used during URN creation; IDs won't be ready.
		}),
	}

	resp, err := p.client.Check(p.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("resource[%v].Check(t=%v,...) failed: err=%v", p.pkg, t, err)
		return nil, err
	}

	var failures []CheckFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, CheckFailure{PropertyKey(failure.Property), failure.Reason})
	}
	glog.V(7).Infof("resource[%v].Check(t=%v,...) success: failures=#%v", p.pkg, t, len(failures))
	return failures, nil
}

// Name names a given resource.
func (p *provider) Name(t tokens.Type, props PropertyMap) (tokens.QName, error) {
	glog.V(7).Infof("resource[%v].Name(t=%v,#props=%v) executing", p.pkg, t, len(props))
	req := &cocorpc.NameRequest{
		Type: string(t),
		Properties: MarshalProperties(p.ctx, props, MarshalOptions{
			PermitOlds: true, // permit old URNs, since this is pre-update.
			RawURNs:    true, // often used during URN creation; IDs won't be ready.
		}),
	}

	resp, err := p.client.Name(p.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("resource[%v].Name(t=%v,...) failed: err=%v", p.pkg, t, err)
		return "", err
	}

	name := tokens.QName(resp.GetName())
	glog.V(7).Infof("resource[%v].Name(t=%v,...) success: name=%v", p.pkg, t, name)
	return name, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.
func (p *provider) Create(t tokens.Type, props PropertyMap) (ID, State, error) {
	glog.V(7).Infof("resource[%v].Create(t=%v,#props=%v) executing", p.pkg, t, len(props))
	req := &cocorpc.CreateRequest{
		Type:       string(t),
		Properties: MarshalProperties(p.ctx, props, MarshalOptions{}),
	}

	resp, err := p.client.Create(p.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("resource[%v].Create(t=%v,...) failed: err=%v", p.pkg, t, err)
		return ID(""), StateUnknown, err
	}

	id := ID(resp.GetId())
	glog.V(7).Infof("resource[%v].Create(t=%v,...) success: id=%v", p.pkg, t, id)
	if id == "" {
		return id,
			StateUnknown,
			errors.Errorf("plugin for package '%v' returned empty ID from create '%v'", p.pkg, t)
	}
	return id, StateOK, nil
}

// Read reads the instance state identified by id/t, and returns a bag of properties.
func (p *provider) Read(id ID, t tokens.Type) (PropertyMap, error) {
	glog.V(7).Infof("resource[%v].Read(id=%v,t=%v) executing", p.pkg, id, t)
	req := &cocorpc.ReadRequest{
		Id:   string(id),
		Type: string(t),
	}

	resp, err := p.client.Read(p.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("resource[%v].Read(id=%v,t=%v) failed: err=%v", p.pkg, id, t, err)
		return nil, err
	}

	props := UnmarshalProperties(resp.GetProperties())
	glog.V(7).Infof("resource[%v].Read(id=%v,t=%v) success: #props=%v", p.pkg, t, id, len(props))
	return props, nil
}

// Update updates an existing resource with new values.
func (p *provider) Update(id ID, t tokens.Type, olds PropertyMap, news PropertyMap) (State, error) {
	contract.Requiref(id != "", "id", "not empty")
	contract.Requiref(t != "", "t", "not empty")

	glog.V(7).Infof("resource[%v].Update(id=%v,t=%v,#olds=%v,#news=%v) executing",
		p.pkg, id, t, len(olds), len(news))
	req := &cocorpc.UpdateRequest{
		Id:   string(id),
		Type: string(t),
		Olds: MarshalProperties(p.ctx, olds, MarshalOptions{
			PermitOlds: true, // permit old URNs since these are the old values.
		}),
		News: MarshalProperties(p.ctx, news, MarshalOptions{}),
	}

	_, err := p.client.Update(p.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("resource[%v].Update(id=%v,t=%v,...) failed: %v", p.pkg, id, t, err)
		return StateUnknown, err
	}

	glog.V(7).Infof("resource[%v].Update(id=%v,t=%v,...) success", p.pkg, id, t)
	return StateOK, nil
}

// UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
func (p *provider) UpdateImpact(id ID, t tokens.Type,
	olds PropertyMap, news PropertyMap) ([]string, PropertyMap, error) {
	contract.Requiref(id != "", "id", "not empty")
	contract.Requiref(t != "", "t", "not empty")

	glog.V(7).Infof("resource[%v].UpdateImpact(id=%v,t=%v,#olds=%v,#news=%v) executing",
		p.pkg, id, t, len(olds), len(news))
	req := &cocorpc.UpdateRequest{
		Id:   string(id),
		Type: string(t),
		Olds: MarshalProperties(p.ctx, olds, MarshalOptions{
			RawURNs: true, // often used during URN creation; IDs won't be ready.
		}),
		News: MarshalProperties(p.ctx, news, MarshalOptions{
			RawURNs: true, // often used during URN creation; IDs won't be ready.
		}),
	}

	resp, err := p.client.UpdateImpact(p.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("resource[%v].UpdateImpact(id=%v,t=%v,...) failed: %v", p.pkg, id, t, err)
		return nil, nil, err
	}

	replaces := resp.GetReplaces()
	changes := UnmarshalProperties(resp.GetChanges())
	glog.V(7).Infof("resource[%v].Update(id=%v,t=%v,...) success: #replaces=%v #changes=%v",
		p.pkg, id, t, len(replaces), len(changes))
	return replaces, changes, nil
}

// Delete tears down an existing resource.
func (p *provider) Delete(id ID, t tokens.Type) (State, error) {
	contract.Requiref(id != "", "id", "not empty")
	contract.Requiref(t != "", "t", "not empty")

	glog.V(7).Infof("resource[%v].Delete(id=%v,t=%v) executing", p.pkg, id, t)
	req := &cocorpc.DeleteRequest{
		Id:   string(id),
		Type: string(t),
	}

	if _, err := p.client.Delete(p.ctx.Request(), req); err != nil {
		glog.V(7).Infof("resource[%v].Delete(id=%v,t=%v) failed: %v", p.pkg, id, t, err)
		return StateUnknown, err
	}

	glog.V(7).Infof("resource[%v].Delete(id=%v,t=%v) success", p.pkg, id, t)
	return StateOK, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (p *provider) Close() error {
	return p.plug.Close()
}
