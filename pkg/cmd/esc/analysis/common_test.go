// Copyright 2023, Pulumi Corporation.
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

package analysis

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/eval"
	"github.com/pulumi/esc/schema"
	"golang.org/x/exp/maps"
)

var testProviderSchema = schema.Object().
	Properties(map[string]schema.Builder{
		"address": schema.String().
			Description("The URL of the Vault server. Must contain a scheme and hostname, but no path."),
		"jwt": schema.Object().
			Properties(map[string]schema.Builder{
				"mount": schema.String().Description("The name of the authentication engine mount."),
				"role":  schema.String().Description("The name of the role to use for login."),
			}).
			Required("role").
			Description("Options for JWT login. JWT login uses an OIDC token issued by the Pulumi Cloud to generate an ephemeral token."),
		"token": schema.Object().
			Properties(map[string]schema.Builder{
				"displayName": schema.String().Description("The display name of the ephemeral token. Defaults to 'pulumi'."),
				"token":       schema.String().Description("The parent token."),
				"maxTtl": schema.String().
					Pattern(`^([0-9]+h)?([0-9]+m)?([0-9]+s)?$`).
					Description("The maximum TTL of the ephemeral token."),
			}).
			Required("token").
			Description("Options for token login. Token login creates an ephemeral child token."),
	}).
	Required("address").
	Schema()

type testProvider struct{}

func (testProvider) Schema() (*schema.Schema, *schema.Schema) {
	return testProviderSchema, schema.Always()
}

func (testProvider) Open(ctx context.Context, inputs map[string]esc.Value, context esc.EnvExecContext) (esc.Value, error) {
	return esc.NewValue(inputs), nil
}

type testProviders struct{}

func (testProviders) LoadProvider(ctx context.Context, name string) (esc.Provider, error) {
	switch name {
	case "test":
		return testProvider{}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", name)
}

type testEnvironments struct{}

func (testEnvironments) LoadEnvironment(ctx context.Context, name string) ([]byte, eval.Decrypter, error) {
	if name != "a" {
		return nil, nil, errors.New("not found")
	}
	return []byte(`{"values": {}}`), nil, nil
}

const def = `imports:
  - a
values:
  fromBase64:
    fn::fromBase64: ${toBase64}
  fromJSON:
    fn::fromJSON: ${toJSON}
  join:
    fn::join: [ ",", "${strings}" ]
  open:
    fn::open::test:
      address: some-url
      jwt:
        mount: foo
        role: bar
      token:
        displayName: foo
        token: bar
        maxTtl: 2h
  secret:
    fn::secret:
      hunter2
  toBase64:
    fn::toBase64: ${join}
  toJSON:
    fn::toJSON: ${open}
  toString:
    fn::toString: ${open}
  open2:
    fn::open::test: ${open}
  interp: hello, ${toString}
  access: ${open["baz"]}
  strings: [ hello, world ]
`

func sortedKeys[T any](m map[string]T) []string {
	keys := maps.Keys(m)
	sort.Strings(keys)
	return keys
}

func visitExprs(env *esc.Environment, visitor func(path string, x esc.Expr)) {
	var visit func(root esc.Expr, path string)
	visit = func(root esc.Expr, path string) {
		visitor(path, root)

		switch {
		case len(root.List) != 0:
			for i, element := range root.List {
				visit(element, fmt.Sprintf("%v/%v", path, i))
			}
		case len(root.Object) != 0:
			for _, key := range sortedKeys(root.Object) {
				visit(root.Object[key], fmt.Sprintf("%v/%v", path, key))
			}
		case root.Builtin != nil:
			visit(root.Builtin.Arg, fmt.Sprintf("%v/%v", path, root.Builtin.Name))
		}
	}

	for _, key := range sortedKeys(env.Exprs) {
		visit(env.Exprs[key], key)
	}
}
