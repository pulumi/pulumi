// Copyright 2026, Pulumi Corporation.
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

package stack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// BaseEnvironmentName is the environment every stack environment in a project imports. It gives a
// project one place to put configuration shared by all of its stacks.
const BaseEnvironmentName = "base"

// ErrBackendNoEnvironments indicates that the given backend does not support ESC environments and
// points the user at the Pulumi Cloud backend, which does.
func ErrBackendNoEnvironments(b backend.Backend) error {
	return fmt.Errorf("backend %v does not support environments; Pulumi ESC environments require the "+
		"Pulumi Cloud backend, use `pulumi login` without arguments to log into the Pulumi Cloud backend", b.Name())
}

// CheckEnvironmentSupport reports whether a backend can host the environments a stack is born with.
//
// Callers check this *before* creating a stack or writing any file, so that an unsupported backend
// fails without leaving anything half-built behind.
func CheckEnvironmentSupport(b backend.Backend) error {
	if _, ok := b.(backend.EnvironmentsBackend); !ok {
		return ErrBackendNoEnvironments(b)
	}
	if _, ok := b.(backend.EnvironmentDefinitionsBackend); !ok {
		return ErrBackendNoEnvironments(b)
	}
	return nil
}

// StackEnvironmentOptions describes the environments to create for a newly born stack.
type StackEnvironmentOptions struct {
	// EnvProject is the ESC project the environments live in; it is the Pulumi project's name.
	EnvProject string
	// EnvName is the environment that backs the stack; it is the stack's name.
	EnvName string
	// Values are the `values.pulumiConfig` entries the stack environment starts life with. Secrets are
	// already wrapped in `fn::secret` by the caller, so no plaintext secret is ever written elsewhere.
	Values map[string]yaml.Node
	// Stdout receives the progress messages. A nil writer discards them.
	Stdout io.Writer
}

// EnvironmentBirthError reports that creating a stack's environments failed, naming the environments
// that were created before the failure so the user knows exactly what state they are in.
type EnvironmentBirthError struct {
	// Created holds the fully qualified references of the environments that were created.
	Created []string
	// Failed is the fully qualified reference of the environment that could not be created.
	Failed string
	Err    error
}

func (e *EnvironmentBirthError) Error() string {
	var b strings.Builder
	if len(e.Created) > 0 {
		fmt.Fprintf(&b, "created environment(s) %s, but ", strings.Join(e.Created, ", "))
	}
	fmt.Fprintf(&b, "could not create environment %s: %v\n", e.Failed, e.Err)
	b.WriteString("the stack was created without 'mainEnvironment', so it works as an ordinary stack; " +
		"re-run with --esc-config once the problem is fixed, " +
		"or migrate later with 'pulumi config env init --main'")
	return b.String()
}

func (e *EnvironmentBirthError) Unwrap() error { return e.Err }

// CreateStackEnvironments gives a newly created stack the environments its configuration lives in:
// `<EnvProject>/base`, shared by every stack in the project, and `<EnvProject>/<EnvName>`, which
// imports it and holds the stack's own values.
//
// Creation is additive only: an environment that already exists is reused with a message and is never
// modified, so re-running the command — or creating a second stack in a project — cannot clobber
// configuration that is already there.
func CreateStackEnvironments(
	ctx context.Context, s backend.Stack, opts StackEnvironmentOptions,
) (*workspace.MainEnvironment, error) {
	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}

	b := s.Backend()
	if err := CheckEnvironmentSupport(b); err != nil {
		return nil, err
	}
	creator := b.(backend.EnvironmentsBackend)
	definitions := b.(backend.EnvironmentDefinitionsBackend)

	orgNamer, ok := s.(interface{ OrgName() string })
	if !ok {
		return nil, fmt.Errorf("cannot determine organization for stack %v", s.Ref())
	}
	org := orgNamer.OrgName()

	var created []string
	ensure := func(envName string, definition []byte, creating string) error {
		fullName := fmt.Sprintf("%s/%s/%s", org, opts.EnvProject, envName)

		_, _, _, err := definitions.GetEnvironmentDefinition(ctx, org, opts.EnvProject, envName, "")
		switch {
		case err == nil:
			fmt.Fprintf(out, "Environment '%s' already exists — reusing.\n", fullName)
			return nil
		case !errors.Is(err, backend.ErrEnvironmentNotFound):
			return &EnvironmentBirthError{Created: created, Failed: fullName, Err: err}
		}

		fmt.Fprintf(out, "Creating environment '%s'...%s\n", fullName, creating)
		diags, err := creator.CreateEnvironment(ctx, org, opts.EnvProject, envName, definition)
		if errors.Is(err, backend.ErrEnvironmentConflict) {
			// Someone created it in the window between the probe and the create. Reuse it, exactly as
			// the probe would have.
			fmt.Fprintf(out, "Environment '%s' already exists — reusing.\n", fullName)
			return nil
		}
		if err != nil {
			return &EnvironmentBirthError{Created: created, Failed: fullName, Err: err}
		}
		if len(diags) != 0 {
			return &EnvironmentBirthError{Created: created, Failed: fullName, Err: diags}
		}
		created = append(created, fullName)
		return nil
	}

	base, err := renderStackEnvironment(nil /*imports*/, nil /*values*/)
	if err != nil {
		return nil, err
	}
	if err := ensure(BaseEnvironmentName, base, ""); err != nil {
		return nil, err
	}

	// A stack literally named `base` is its own base environment; importing itself would be a cycle.
	if opts.EnvName == BaseEnvironmentName {
		return &workspace.MainEnvironment{Project: opts.EnvProject, Name: BaseEnvironmentName}, nil
	}

	baseRef := opts.EnvProject + "/" + BaseEnvironmentName
	definition, err := renderStackEnvironment([]string{baseRef}, opts.Values)
	if err != nil {
		return nil, err
	}
	if err := ensure(opts.EnvName, definition, fmt.Sprintf(" (imports %s)", baseRef)); err != nil {
		return nil, err
	}

	return &workspace.MainEnvironment{Project: opts.EnvProject, Name: opts.EnvName}, nil
}

// renderStackEnvironment renders the definition a stack environment is created with.
//
// `values:` is always present, even when empty, so that the document a later `pulumi config set`
// reads back already has the mapping its writes extend.
func renderStackEnvironment(imports []string, values map[string]yaml.Node) ([]byte, error) {
	scalar := func(s string) *yaml.Node {
		n := &yaml.Node{}
		n.SetString(s)
		return n
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	if len(imports) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, i := range imports {
			seq.Content = append(seq.Content, scalar(i))
		}
		root.Content = append(root.Content, scalar("imports"), seq)
	}

	valuesNode := &yaml.Node{Kind: yaml.MappingNode}
	if len(values) > 0 {
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		pulumiConfig := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range keys {
			value := values[k]
			pulumiConfig.Content = append(pulumiConfig.Content, scalar(k), &value)
		}
		valuesNode.Content = append(valuesNode.Content, scalar("pulumiConfig"), pulumiConfig)
	}
	root.Content = append(root.Content, scalar("values"), valuesNode)

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, fmt.Errorf("rendering environment definition: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("rendering environment definition: %w", err)
	}
	return b.Bytes(), nil
}
