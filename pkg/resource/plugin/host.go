// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// A Host hosts provider plugins and makes them easily accessible by package name.
type Host interface {
	// ServerAddr returns the address at which the host's RPC interface may be found.
	ServerAddr() string

	// Log logs a global message, including errors and warnings.
	Log(sev diag.Severity, msg string)
	// ReadLocation reads the value from a static or module property.
	ReadLocation(tok tokens.Token) (resource.PropertyValue, error)
	// ReadLocations reads takes a class or module token and reads all (static) properties belonging to it.
	ReadLocations(tok tokens.Token) (resource.PropertyMap, error)

	// Analyzer fetches the analyzer with a given name, possibly lazily allocating the plugins for it.  If an analyzer
	// could not be found, or an error occurred while creating it, a non-nil error is returned.
	Analyzer(nm tokens.QName) (Analyzer, error)
	// Provider fetches the provider for a given package, lazily allocating it if necessary.  If a provider for this
	// package could not be found, or an error occurs while creating it, a non-nil error is returned.
	Provider(pkg tokens.Package) (Provider, error)

	// Close reclaims any resources associated with the host.
	Close() error
}

// NewDefaultHost implements the standard plugin logic, using the standard installation root to find them.
func NewDefaultHost(ctx *Context) (Host, error) {
	host := &defaultHost{
		ctx:       ctx,
		analyzers: make(map[tokens.QName]Analyzer),
		providers: make(map[tokens.Package]Provider),
	}

	// Fire up a gRPC server to listen for requests.  This acts as a RPC interface that plugins can use
	// to "phone home" in case there are things the host must do on behalf of the plugins (like log, etc).
	svr, err := newHostServer(host, ctx)
	if err != nil {
		return nil, err
	}
	host.server = svr

	return host, nil
}

type defaultHost struct {
	ctx       *Context                    // the shared context for this host.
	analyzers map[tokens.QName]Analyzer   // a cache of analyzer plugins and their processes.
	providers map[tokens.Package]Provider // a cache of provider plugins and their processes.
	server    *hostServer                 // the server's RPC machinery.
}

func (host *defaultHost) ServerAddr() string {
	return host.server.Address()
}

func (host *defaultHost) Log(sev diag.Severity, msg string) {
	host.ctx.Diag.Logf(sev, diag.Message(msg))
}

func (host *defaultHost) ReadLocation(tok tokens.Token) (resource.PropertyValue, error) {
	// First lookup the symbol.  If it's missing, just return nil, so that this is "dynamic-like".
	e := host.ctx.E
	sym := e.Ctx().LookupSymbol(nil, tok, false)
	if sym == nil {
		return resource.NewNullProperty(), nil
	}

	// Next do a read of the location.  This may trigger code (initializers), and so it might fail.
	var obj *rt.Object
	loc, uw := e.LoadLocation(nil, sym, nil, false)
	if uw == nil {
		obj, uw = loc.Read(nil)
	}
	if uw != nil {
		contract.Assert(uw.Throw())
		return resource.PropertyValue{},
			errors.Errorf("An error occurred reading property '%v': %v", tok, uw.Thrown().Message(host.ctx.Diag))
	}

	return resource.CopyObject(obj), nil
}

func (host *defaultHost) ReadLocations(tok tokens.Token) (resource.PropertyMap, error) {
	props := make(resource.PropertyMap)

	// First load up the class or module object.  If missing, return an empty map.
	e := host.ctx.E
	sym := e.Ctx().LookupSymbol(nil, tok, false)
	if sym == nil {
		glog.V(9).Infof("Reading locations at token: %v; location not found", tok)
		return props, nil
	}

	// Now, for each (static) property, read it and add it to the list.
	switch t := sym.(type) {
	case *symbols.Class:
		glog.V(9).Infof("Reading locations at token: %v; %v class members found", tok, len(t.Members))
		for _, pname := range t.StableMembers() {
			prop := t.Members[pname]
			if _, isprop := prop.(*symbols.ClassProperty); isprop {
				if prop.Static() {
					v, err := host.ReadLocation(prop.Token())
					if err != nil {
						return nil, err
					}
					props[resource.PropertyKey(pname)] = v
				}
			}
		}
	case *symbols.Module:
		glog.V(9).Infof("Reading locations at token: %v; %v module members found", tok, len(t.Members))
		for _, pname := range t.StableMembers() {
			prop := t.Members[pname]
			if _, isprop := prop.(*symbols.ModuleProperty); isprop {
				v, err := host.ReadLocation(prop.Token())
				if err != nil {
					return nil, err
				}
				props[resource.PropertyKey(pname)] = v
			}
		}
	default:
		return nil, errors.Errorf("Only reads of class or module properties supported; '%v' is neither", tok)
	}

	return props, nil
}

func (host *defaultHost) Analyzer(name tokens.QName) (Analyzer, error) {
	// First see if we already loaded this plugin.
	if plug, has := host.analyzers[name]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewAnalyzer(host, host.ctx, name)
	if err == nil {
		host.analyzers[name] = plug // memoize the result.
	}
	return plug, err
}

func (host *defaultHost) Provider(pkg tokens.Package) (Provider, error) {
	// First see if we already loaded this plugin.
	if plug, has := host.providers[pkg]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewProvider(host, host.ctx, pkg)
	if err == nil {
		host.providers[pkg] = plug // memoize the result.
	}
	return plug, err
}

func (host *defaultHost) Close() error {
	// Close all plugins.
	for _, plugin := range host.analyzers {
		if err := plugin.Close(); err != nil {
			glog.Infof("Error closing '%v' analyzer plugin during shutdown; ignoring: %v", plugin.Name(), err)
		}
	}
	for _, plugin := range host.providers {
		if err := plugin.Close(); err != nil {
			glog.Infof("Error closing '%v' provider plugin during shutdown; ignoring: %v", plugin.Pkg(), err)
		}
	}

	// Empty out all maps.
	host.analyzers = make(map[tokens.QName]Analyzer)
	host.providers = make(map[tokens.Package]Provider)

	// Finally, shut down the host's gRPC server.
	return host.server.Cancel()
}
