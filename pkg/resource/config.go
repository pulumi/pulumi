// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"sort"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/compiler"
	"github.com/pulumi/lumi/pkg/compiler/binder"
	"github.com/pulumi/lumi/pkg/compiler/errors"
	"github.com/pulumi/lumi/pkg/compiler/symbols"
	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// ConfigMap contains a mapping from variable token to the value to poke into that variable.
type ConfigMap map[tokens.Token]interface{}

// ConfigApplier returns a compiler preexec function that applies the configuration when invoked, and copies the
// resulting configuration variables into the supplied map (if it is non-nil).
func (cfg *ConfigMap) ConfigApplier(vars map[tokens.Token]*rt.Object) compiler.Preexec {
	return func(ctx *binder.Context, pkg *symbols.Package, e eval.Interpreter) {
		configs := cfg.ApplyConfig(ctx, pkg, e)
		contract.Assert(configs != nil)
		if vars != nil {
			for k, v := range configs {
				vars[k] = v
			}
		}
	}
}

// ApplyConfig applies the configuration map to an existing interpreter context.  The map is simply a map of tokens --
// which must be globally settable variables (module properties or static class properties) -- to serializable constant
// values.  The routine simply walks these tokens in sorted order, and assigns the constant objects.  Note that, because
// we are accessing module and class members, this routine will also trigger the relevant initialization routines.
func (cfg *ConfigMap) ApplyConfig(ctx *binder.Context, pkg *symbols.Package,
	e eval.Interpreter) map[tokens.Token]*rt.Object {
	glog.V(5).Infof("Applying configuration values for package '%v'", pkg)

	// Track all configuration variables that get set, for diagnostics and plumbing.
	vars := make(map[tokens.Token]*rt.Object)

	if cfg != nil {
		// For each config entry, bind the token to its symbol, and then attempt to assign to it.
		for _, tok := range StableConfigKeys(*cfg) {
			glog.V(5).Infof("Applying configuration value for token '%v'", tok)

			// Bind to the symbol; if it returns nil, this means an error has resulted, and we can skip it.
			var tree diag.Diagable // there is no source info for this; eventually we may assign one.
			if sym := ctx.LookupSymbol(tree, tokens.Token(tok), true); sym != nil {
				var ok bool
				switch s := sym.(type) {
				case *symbols.ModuleProperty:
					ok = true
				case *symbols.ClassProperty:
					// class properties are ok, so long as they are static.
					ok = s.Static()
				default:
					ok = false
				}
				if !ok {
					ctx.Diag.Errorf(errors.ErrorIllegalConfigToken, tok)
					continue // skip to the next one
				}

				// Load up the location as an l-value; because we don't support instance properties, this is nil.
				if loc := e.LoadLocation(tree, sym, nil, true); loc != nil {
					// Allocate a new constant for the value we are about to assign, and assign it to the location.
					v := (*cfg)[tok]
					obj := e.NewConstantObject(nil, v)
					loc.Assign(tree, obj)
					vars[tok] = obj
				}
			}
		}
	}

	if !ctx.Diag.Success() {
		ctx.Diag.Errorf(errors.ErrorConfigApplyFailure, pkg)
	}

	return vars
}

func StableConfigKeys(config ConfigMap) []tokens.Token {
	sorted := make(configKeys, 0, len(config))
	for key := range config {
		sorted = append(sorted, key)
	}
	sort.Sort(sorted)
	return sorted
}

type configKeys []tokens.Token

func (s configKeys) Len() int {
	return len(s)
}

func (s configKeys) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s configKeys) Less(i, j int) bool {
	return s[i] < s[j]
}
