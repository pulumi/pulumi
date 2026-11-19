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

package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v3"
)

//go:embed prelude.js
var preludeSrc string

var prelude = sync.OnceValue(func() *goja.Program {
	return goja.MustCompile("prelude.js", preludeSrc, true)
})

// project is the part of PulumiPolicy.yaml this host reads.
type project struct {
	Main        string `yaml:"main"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

// packDesc is what a bundle sets as globalThis.__pulumiPolicyPack, without the validate functions.
type packDesc struct {
	AbiVersion       int            `json:"abiVersion"`
	Name             string         `json:"name"`
	Version          string         `json:"version"`
	DisplayName      string         `json:"displayName"`
	Description      string         `json:"description"`
	EnforcementLevel string         `json:"enforcementLevel"`
	Readme           string         `json:"readme"`
	Provider         string         `json:"provider"`
	Repository       string         `json:"repository"`
	Tags             []string       `json:"tags"`
	InitialConfig    map[string]any `json:"initialConfig"`
	Policies         []policyDesc   `json:"policies"`
}

type policyDesc struct {
	Name             string `json:"name"`
	DisplayName      string `json:"displayName"`
	Description      string `json:"description"`
	Message          string `json:"message"`
	EnforcementLevel string `json:"enforcementLevel"`
	Severity         string `json:"severity"`
	Framework        *struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		Reference     string `json:"reference"`
		Specification string `json:"specification"`
	} `json:"framework"`
	Tags             []string `json:"tags"`
	RemediationSteps string   `json:"remediationSteps"`
	URL              string   `json:"url"`
	ConfigSchema     *struct {
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	} `json:"configSchema"`
	// Kind is "resource" (the default) or "stack".
	Kind          string   `json:"kind"`
	ResourceTypes []string `json:"resourceTypes"`
}

type policy struct {
	desc     policyDesc
	stack    bool
	types    map[string]struct{} // nil means every type
	validate goja.Value
	level    string // the resolved enforcement level; Configure can change it
	config   goja.Value
}

// pack is a loaded purejs policy pack: its bundle evaluated in a goja runtime. A goja runtime is not safe for
// concurrent use, so every call into it holds mu.
type pack struct {
	mu          sync.Mutex
	rt          *goja.Runtime
	invoke      goja.Callable
	unknown     goja.Callable
	desc        packDesc
	version     string // PulumiPolicy.yaml's version wins over the bundle's
	description string
	policies    []*policy
}

// loadPack reads PulumiPolicy.yaml in dir and evaluates the bundle it names (policies.js by default).
func loadPack(dir string) (*pack, error) {
	var proj project
	b, err := os.ReadFile(filepath.Join(dir, "PulumiPolicy.yaml"))
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, &proj); err != nil {
		return nil, fmt.Errorf("reading PulumiPolicy.yaml: %w", err)
	}
	if proj.Main == "" {
		proj.Main = "policies.js"
	}
	main := filepath.Join(dir, proj.Main)
	src, err := os.ReadFile(main)
	if err != nil {
		return nil, fmt.Errorf("reading policy pack bundle: %w", err)
	}
	prog, err := goja.Compile(main, string(src), false)
	if err != nil {
		return nil, fmt.Errorf("compiling policy pack bundle %s: %w", main, err)
	}

	rt := goja.New()
	rt.SetMaxCallStackSize(4096)
	if err := installHostFuncs(rt); err != nil {
		return nil, err
	}
	if _, err := rt.RunProgram(prelude()); err != nil {
		return nil, fmt.Errorf("running policy prelude: %w", err)
	}
	if _, err := rt.RunProgram(prog); err != nil {
		return nil, fmt.Errorf("evaluating policy pack bundle %s: %w", main, err)
	}

	describe, _ := goja.AssertFunction(rt.Get("__describe"))
	descJSON, err := describe(goja.Undefined())
	if err != nil {
		return nil, fmt.Errorf("reading policy pack descriptor: %w", err)
	}
	var desc packDesc
	if err := json.Unmarshal([]byte(descJSON.String()), &desc); err != nil {
		return nil, fmt.Errorf("decoding policy pack descriptor: %w", err)
	}
	if desc.AbiVersion > 1 {
		return nil, fmt.Errorf("policy pack %s uses ABI version %d; this host runs version 1", desc.Name, desc.AbiVersion)
	}
	invoke, _ := goja.AssertFunction(rt.Get("__invoke"))
	unknown, _ := goja.AssertFunction(rt.Get("__unknownGetter"))

	packLevel := desc.EnforcementLevel
	if packLevel == "" {
		packLevel = "advisory"
	}
	jsPolicies := rt.Get("__pulumiPolicyPack").ToObject(rt).Get("policies").ToObject(rt)
	policies := make([]*policy, len(desc.Policies))
	for i, d := range desc.Policies {
		if d.Name == "all" {
			return nil, errors.New(`policy pack cannot contain a policy named "all"`)
		}
		if d.ConfigSchema != nil {
			if _, has := d.ConfigSchema.Properties["enforcementLevel"]; has {
				return nil, fmt.Errorf("policy %q: configSchema cannot contain an enforcementLevel property", d.Name)
			}
		}
		validate := jsPolicies.Get(strconv.Itoa(i)).ToObject(rt).Get("validate")
		if _, ok := goja.AssertFunction(validate); !ok {
			return nil, fmt.Errorf("policy %q has no validate function", d.Name)
		}
		p := &policy{desc: d, validate: validate, config: rt.NewObject(), level: d.EnforcementLevel}
		switch d.Kind {
		case "stack":
			p.stack = true
		case "resource", "":
		default:
			return nil, fmt.Errorf("policy %q has unknown kind %q", d.Name, d.Kind)
		}
		if len(d.ResourceTypes) > 0 && !slices.Contains(d.ResourceTypes, "*") {
			p.types = make(map[string]struct{}, len(d.ResourceTypes))
			for _, t := range d.ResourceTypes {
				p.types[t] = struct{}{}
			}
		}
		if p.level == "" {
			p.level = packLevel
		}
		if _, ok := enforcementLevels[p.level]; !ok {
			return nil, fmt.Errorf("policy %q has unknown enforcement level %q", d.Name, p.level)
		}
		policies[i] = p
	}

	pk := &pack{
		rt: rt, invoke: invoke, unknown: unknown, desc: desc, policies: policies,
		version: desc.Version, description: desc.Description,
	}
	if proj.Version != "" {
		pk.version = proj.Version
	}
	if pk.description == "" {
		pk.description = proj.Description
	}
	return pk, nil
}

// result is what one policy invocation reported.
type result struct {
	violations []violation
	// notApplicable is set when the policy called notApplicable.
	notApplicable *string
	// unknown is set when the policy read a value that is unknown during preview.
	unknown string
}

type violation struct {
	message any // a string, or an object with message, urn, severity, ...
	urn     string
}

// run invokes one policy. The caller holds mu.
func (pk *pack) run(p *policy, input *goja.Object) (result, error) {
	_ = input.Set("config", p.config)
	res, err := pk.invoke(goja.Undefined(), p.validate, input)
	if err != nil {
		return result{}, fmt.Errorf("Error validating resource with policy '%s' from policy pack '%s@v%s': %w",
			p.desc.Name, pk.desc.Name, pk.version, err)
	}
	obj := res.ToObject(pk.rt)
	var r result
	if vs, ok := obj.Get("v").Export().([]any); ok {
		for _, v := range vs {
			pair, _ := v.([]any)
			if len(pair) == 0 {
				continue
			}
			var urn string
			if len(pair) > 1 {
				urn, _ = pair[1].(string)
			}
			r.violations = append(r.violations, violation{message: pair[0], urn: urn})
		}
	}
	if na := obj.Get("na"); na != nil && !goja.IsUndefined(na) {
		s := na.String()
		r.notApplicable = &s
	}
	if unk := obj.Get("unk"); unk != nil && !goja.IsUndefined(unk) {
		r.unknown = unk.String()
	}
	return r, nil
}
