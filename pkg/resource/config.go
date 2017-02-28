// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"sort"

	"github.com/pulumi/coconut/pkg/compiler/binder"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/eval"
	"github.com/pulumi/coconut/pkg/tokens"
)

// ConfigMap contains a mapping from variable token to the value to poke into that variable.
type ConfigMap map[tokens.ModuleMember]interface{}

// ApplyConfig applies the configuration map to an existing interpreter context.  The map is simply a map of tokens --
// which must be globally settable variables (module properties or static class properties) -- to serializable constant
// values.  The routine simply walks these tokens in sorted order, and assigns the constant objects.  Note that, because
// we are accessing module and class members, this routine will also trigger the relevant initialization routines.
func (cfg *ConfigMap) ApplyConfig(ctx *binder.Context, pkg *symbols.Package, e eval.Interpreter) {
	if cfg != nil {
		// For each config entry, bind the token to its symbol, and then attempt to assign to it.
		for _, tok := range stableConfigKeys(*cfg) {
			// Bind to the symbol; if it returns nil, this means an error has resulted, and we can skip it.
			var tree diag.Diagable = nil // there is no source info for this; eventually we may assign one.
			if sym := ctx.LookupSymbol(tree, tokens.Token(tok), true); sym != nil {
				switch s := sym.(type) {
				case *symbols.ModuleProperty:
					// ok
				case *symbols.ClassProperty:
					// class properties are ok, so long as they are static.
					if !s.Static() {
						ctx.Diag.Errorf(errors.ErrorIllegalConfigToken, tok)
						return
					}
				default:
					ctx.Diag.Errorf(errors.ErrorIllegalConfigToken, tok)
					return
				}

				// Load up the location as an l-value; because we don't support instance properties, this is nil.
				if loc := e.LoadLocation(tree, sym, nil, true); loc != nil {
					// Allocate a new constant for the value we are about to assign, and assign it to the location.
					v := (*cfg)[tok]
					loc.Assign(tree, e.NewConstantObject(nil, v))
				}
			}
		}
	}
}

func stableConfigKeys(config ConfigMap) []tokens.ModuleMember {
	sorted := make(configKeys, 0, len(config))
	for key := range config {
		sorted = append(sorted, key)
	}
	sort.Sort(sorted)
	return sorted
}

type configKeys []tokens.ModuleMember

func (s configKeys) Len() int {
	return len(s)
}

func (s configKeys) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s configKeys) Less(i, j int) bool {
	return s[i] < s[j]
}
