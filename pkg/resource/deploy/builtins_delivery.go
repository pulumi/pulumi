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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

const (
	deliveryStackType      = "pulumi:delivery:Stack"
	deliveryStackGroupType = "pulumi:delivery:StackGroup"
)

// CoherenceWindowClient is a BackendClient whose update runs in a coherence window, which the
// delivery resources run their stacks in as well.
type CoherenceWindowClient interface {
	CoherenceWindow() string
	CreateCoherenceWindowSubgroup(ctx context.Context, size int) (string, error)
	CloseCoherenceWindowSubgroup(ctx context.Context, subgroupID string) error
}

type deliveryStack struct {
	stack     string
	directory string
}

func runPulumi(ctx context.Context, dir string, args []string) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func checkDeliveryStack(inputs property.Map) []plugin.CheckFailure {
	var failures []plugin.CheckFailure
	for k := range inputs.All {
		if k != "stack" && k != "source" {
			failures = append(failures, plugin.CheckFailure{
				Property: resource.PropertyKey(k), Reason: fmt.Sprintf("unknown property %q", k),
			})
		}
	}
	if _, _, err := deliveryStackFromInputs(inputs); err != nil {
		failures = append(failures, plugin.CheckFailure{Property: "stack", Reason: err.Error()})
	}
	return failures
}

func checkDeliveryStackGroup(inputs property.Map) []plugin.CheckFailure {
	var failures []plugin.CheckFailure
	for k := range inputs.All {
		if k != "stacks" {
			failures = append(failures, plugin.CheckFailure{
				Property: resource.PropertyKey(k), Reason: fmt.Sprintf("unknown property %q", k),
			})
		}
	}
	if _, _, err := deliveryStacksFromInputs(inputs.Get("stacks")); err != nil {
		failures = append(failures, plugin.CheckFailure{Property: "stacks", Reason: err.Error()})
	}
	return failures
}

// deliveryStackFromInputs reads a stack and where its program is, reporting false while either is
// still unknown.
func deliveryStackFromInputs(inputs property.Map) (deliveryStack, bool, error) {
	stack, ok := inputs.GetOk("stack")
	if !ok {
		return deliveryStack{}, false, errors.New(`missing required property "stack"`)
	}
	source, ok := inputs.GetOk("source")
	if !ok {
		return deliveryStack{}, false, errors.New(`missing required property "source"`)
	}
	if stack.IsComputed() || source.IsComputed() {
		return deliveryStack{}, false, nil
	}
	if !stack.IsString() {
		return deliveryStack{}, false, errors.New(`property "stack" must be a string`)
	}
	if !source.IsMap() {
		return deliveryStack{}, false, errors.New(`property "source" must be an object`)
	}
	directory, ok := source.AsMap().GetOk("directory")
	if !ok {
		return deliveryStack{}, false, errors.New(`missing required property "source.directory"`)
	}
	if directory.IsComputed() {
		return deliveryStack{}, false, nil
	}
	if !directory.IsString() {
		return deliveryStack{}, false, errors.New(`property "source.directory" must be a string`)
	}
	return deliveryStack{stack: stack.AsString(), directory: directory.AsString()}, true, nil
}

func deliveryStacksFromInputs(stacks property.Value) ([]deliveryStack, bool, error) {
	if stacks.IsNull() {
		return nil, false, errors.New(`missing required property "stacks"`)
	}
	if stacks.IsComputed() {
		return nil, false, nil
	}
	if !stacks.IsArray() {
		return nil, false, errors.New(`property "stacks" must be an array`)
	}
	known := true
	var out []deliveryStack
	for i, v := range stacks.AsArray().All {
		if v.IsComputed() {
			known = false
			continue
		}
		if !v.IsMap() {
			return nil, false, fmt.Errorf(`property "stacks[%d]" must be an object`, i)
		}
		stack, stackKnown, err := deliveryStackFromInputs(v.AsMap())
		if err != nil {
			return nil, false, fmt.Errorf("stacks[%d]: %w", i, err)
		}
		known = known && stackKnown
		out = append(out, stack)
	}
	return out, known, nil
}

func (p *builtinProvider) coherenceWindowClient() (CoherenceWindowClient, error) {
	c, ok := p.backendClient.(CoherenceWindowClient)
	if !ok || c.CoherenceWindow() == "" {
		return nil, errors.New(
			"delivery resources need the Pulumi Cloud backend, which runs their stacks in a coherence window")
	}
	return c, nil
}

func (p *builtinProvider) deliver(
	ctx context.Context, urn resource.URN, inputs property.Map, preview bool,
) (property.Map, error) {
	if urn.Type() == deliveryStackGroupType {
		return p.deliverStackGroup(ctx, urn, inputs, preview)
	}
	return p.deliverStack(ctx, urn, inputs, preview)
}

func (p *builtinProvider) deliverStack(
	ctx context.Context, urn resource.URN, inputs property.Map, preview bool,
) (property.Map, error) {
	stack, known, err := deliveryStackFromInputs(inputs)
	if err != nil {
		return property.Map{}, err
	}
	if !known {
		return property.NewMap(map[string]property.Value{
			"stack":             inputs.Get("stack"),
			"outputs":           property.New(property.Computed),
			"secretOutputNames": property.New(property.Computed),
		}), nil
	}
	client, err := p.coherenceWindowClient()
	if err != nil {
		return property.Map{}, err
	}
	if err := p.runDeliveryStack(ctx, urn, stack, client.CoherenceWindow(), preview); err != nil {
		return property.Map{}, err
	}
	return p.readDeliveryStack(ctx, stack.stack)
}

func (p *builtinProvider) deliverStackGroup(
	ctx context.Context, urn resource.URN, inputs property.Map, preview bool,
) (property.Map, error) {
	stacks, known, err := deliveryStacksFromInputs(inputs.Get("stacks"))
	if err != nil {
		return property.Map{}, err
	}
	if !known {
		return property.NewMap(map[string]property.Value{
			"stacks":  inputs.Get("stacks"),
			"members": property.New(property.Computed),
		}), nil
	}
	client, err := p.coherenceWindowClient()
	if err != nil {
		return property.Map{}, err
	}
	subgroup, err := client.CreateCoherenceWindowSubgroup(ctx, len(stacks))
	if err != nil {
		return property.Map{}, fmt.Errorf("creating a coherence window subgroup for %s: %w", urn.Name(), err)
	}
	// Stacks that never joined stop holding their siblings' reads once the group is done with them.
	defer func() {
		if err := client.CloseCoherenceWindowSubgroup(context.WithoutCancel(ctx), subgroup); err != nil {
			p.diag.Warningf(diag.Message(urn, "closing coherence window subgroup %s: %v"), subgroup, err)
		}
	}()

	var runs errgroup.Group
	for _, stack := range stacks {
		runs.Go(func() error { return p.runDeliveryStack(ctx, urn, stack, subgroup, preview) })
	}
	if err := runs.Wait(); err != nil {
		return property.Map{}, err
	}

	members := make([]property.Value, len(stacks))
	for i, stack := range stacks {
		member, err := p.readDeliveryStack(ctx, stack.stack)
		if err != nil {
			return property.Map{}, err
		}
		members[i] = property.New(member)
	}
	return property.NewMap(map[string]property.Value{
		"stacks":  inputs.Get("stacks"),
		"members": property.New(members),
	}), nil
}

func (p *builtinProvider) runDeliveryStack(
	ctx context.Context, urn resource.URN, stack deliveryStack, window string, preview bool,
) error {
	verb := "updating"
	args := []string{
		"up", "--yes", "--skip-preview", "--stack", stack.stack, "--non-interactive", "--coherence-window", window,
	}
	if preview {
		verb = "previewing"
		args = []string{"preview", "--stack", stack.stack, "--non-interactive", "--coherence-window", window}
	}
	dir, err := filepath.Abs(stack.directory)
	if err != nil {
		return err
	}

	p.diag.Infof(diag.Message(urn, "%s %s in %s"), verb, stack.stack, stack.directory)
	out, err := p.runPulumi(ctx, dir, args)
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", verb, stack.stack, err, lastLines(string(out), 40))
	}
	return nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func (p *builtinProvider) readDeliveryStack(ctx context.Context, stack string) (property.Map, error) {
	read, err := p.readStackReference(ctx, property.NewMap(map[string]property.Value{"name": property.New(stack)}))
	if err != nil {
		return property.Map{}, err
	}
	return property.NewMap(map[string]property.Value{
		"stack":             property.New(stack),
		"outputs":           read.Get("outputs"),
		"secretOutputNames": read.Get("secretOutputNames"),
	}), nil
}

func (p *builtinProvider) refreshDelivery(
	ctx context.Context, urn resource.URN, inputs property.Map,
) (property.Map, error) {
	if urn.Type() == deliveryStackType {
		stack, known, err := deliveryStackFromInputs(inputs)
		if err != nil || !known {
			return property.Map{}, errors.New("a delivery stack can not be imported")
		}
		return p.readDeliveryStack(ctx, stack.stack)
	}

	stacks, known, err := deliveryStacksFromInputs(inputs.Get("stacks"))
	if err != nil || !known {
		return property.Map{}, errors.New("a delivery stack group can not be imported")
	}
	members := make([]property.Value, len(stacks))
	for i, stack := range stacks {
		member, err := p.readDeliveryStack(ctx, stack.stack)
		if err != nil {
			return property.Map{}, err
		}
		members[i] = property.New(member)
	}
	return property.NewMap(map[string]property.Value{
		"stacks":  inputs.Get("stacks"),
		"members": property.New(members),
	}), nil
}
