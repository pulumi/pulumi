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
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sort"

	"github.com/dop251/goja"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

var enforcementLevels = map[string]pulumirpc.EnforcementLevel{
	"advisory":  pulumirpc.EnforcementLevel_ADVISORY,
	"mandatory": pulumirpc.EnforcementLevel_MANDATORY,
	"remediate": pulumirpc.EnforcementLevel_REMEDIATE,
	"disabled":  pulumirpc.EnforcementLevel_DISABLED,
}

var severities = map[string]pulumirpc.PolicySeverity{
	"":         pulumirpc.PolicySeverity_POLICY_SEVERITY_UNSPECIFIED,
	"low":      pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW,
	"medium":   pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM,
	"high":     pulumirpc.PolicySeverity_POLICY_SEVERITY_HIGH,
	"critical": pulumirpc.PolicySeverity_POLICY_SEVERITY_CRITICAL,
}

func levelName(l pulumirpc.EnforcementLevel) string {
	for name, v := range enforcementLevels {
		if v == l {
			return name
		}
	}
	return "advisory"
}

// analyzer serves a loaded pack as the engine's Analyzer: the same service a Node.js policy pack serves, so the engine
// can't tell them apart.
type analyzer struct {
	pulumirpc.UnimplementedAnalyzerServer

	pack *pack
}

func (a *analyzer) Handshake(
	context.Context, *pulumirpc.AnalyzerHandshakeRequest,
) (*pulumirpc.AnalyzerHandshakeResponse, error) {
	return &pulumirpc.AnalyzerHandshakeResponse{}, nil
}

func (a *analyzer) ConfigureStack(
	context.Context, *pulumirpc.AnalyzerStackConfigureRequest,
) (*pulumirpc.AnalyzerStackConfigureResponse, error) {
	return &pulumirpc.AnalyzerStackConfigureResponse{}, nil
}

func (a *analyzer) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version.Version}, nil
}

func (a *analyzer) Analyze(_ context.Context, req *pulumirpc.AnalyzeRequest) (*pulumirpc.AnalyzeResponse, error) {
	pk := a.pack
	pk.mu.Lock()
	defer pk.mu.Unlock()

	resp := &pulumirpc.AnalyzeResponse{}
	var input *goja.Object
	for _, p := range pk.policies {
		if p.stack || p.level == "disabled" {
			continue
		}
		if p.types != nil {
			if _, ok := p.types[req.GetType()]; !ok {
				// As in the Node.js policy SDK, only a policy for a single type reports that it doesn't apply; one with
				// several types counts as passed.
				if len(p.desc.ResourceTypes) == 1 {
					resp.NotApplicable = append(resp.NotApplicable, &pulumirpc.PolicyNotApplicable{
						PolicyName: p.desc.Name,
						Reason:     fmt.Sprintf("Policy only applies to resources of type '%s'", p.desc.ResourceTypes[0]),
					})
				}
				continue
			}
		}
		if input == nil {
			input = pk.rt.NewObject()
			_ = input.Set("type", req.GetType())
			_ = input.Set("name", req.GetName())
			_ = input.Set("urn", req.GetUrn())
			_ = input.Set("props", pk.propsToJS(req.GetProperties()))
		}
		r, err := pk.run(p, input)
		if err != nil {
			return nil, err
		}
		pk.report(p, r, resp)
	}
	return resp, nil
}

func (a *analyzer) AnalyzeStack(
	_ context.Context, req *pulumirpc.AnalyzeStackRequest,
) (*pulumirpc.AnalyzeResponse, error) {
	pk := a.pack
	pk.mu.Lock()
	defer pk.mu.Unlock()

	resp := &pulumirpc.AnalyzeResponse{}
	var input *goja.Object
	for _, p := range pk.policies {
		if !p.stack || p.level == "disabled" {
			continue
		}
		if p.types != nil && !hasType(req.GetResources(), p.types) {
			resp.NotApplicable = append(resp.NotApplicable, &pulumirpc.PolicyNotApplicable{
				PolicyName: p.desc.Name,
				Reason:     fmt.Sprintf("Policy only applies to resources of type '%s'", p.desc.ResourceTypes[0]),
			})
			continue
		}
		if input == nil {
			input = pk.rt.NewObject()
			_ = input.Set("resources", pk.stackToJS(req.GetResources()))
		}
		r, err := pk.run(p, input)
		if err != nil {
			return nil, err
		}
		pk.report(p, r, resp)
	}
	return resp, nil
}

func hasType(resources []*pulumirpc.AnalyzerResource, types map[string]struct{}) bool {
	for _, r := range resources {
		if _, ok := types[r.GetType()]; ok {
			return true
		}
	}
	return false
}

func (pk *pack) stackToJS(resources []*pulumirpc.AnalyzerResource) *goja.Object {
	vals := make([]any, len(resources))
	for i, r := range resources {
		o := pk.rt.NewObject()
		_ = o.Set("type", r.GetType())
		_ = o.Set("name", r.GetName())
		_ = o.Set("urn", r.GetUrn())
		_ = o.Set("props", pk.propsToJS(r.GetProperties()))
		if r.GetParent() != "" {
			_ = o.Set("parent", r.GetParent())
		}
		deps := make([]any, len(r.GetDependencies()))
		for j, d := range r.GetDependencies() {
			deps[j] = d
		}
		_ = o.Set("dependencies", pk.rt.NewArray(deps...))
		pd := pk.rt.NewObject()
		for k, v := range r.GetPropertyDependencies() {
			if len(v.GetUrns()) == 0 {
				continue
			}
			urns := make([]any, len(v.GetUrns()))
			for j, u := range v.GetUrns() {
				urns[j] = u
			}
			_ = pd.Set(k, pk.rt.NewArray(urns...))
		}
		_ = o.Set("propertyDependencies", pd)
		vals[i] = o
	}
	return pk.rt.NewArray(vals...)
}

// report adds what one policy reported to resp, following the Node.js policy SDK's reportViolation.
func (pk *pack) report(p *policy, r result, resp *pulumirpc.AnalyzeResponse) {
	level := p.level
	if level == "remediate" {
		// This host can't remediate, so a remediate policy's violations are reported as mandatory.
		level = "mandatory"
	}
	for _, v := range r.violations {
		d := &pulumirpc.AnalyzeDiagnostic{
			PolicyName:        p.desc.Name,
			PolicyPackName:    pk.desc.Name,
			PolicyPackVersion: pk.version,
			Description:       p.desc.Description,
			EnforcementLevel:  enforcementLevels[level],
			Urn:               v.urn,
			Severity:          severities[p.desc.Severity],
		}
		if obj, ok := v.message.(map[string]any); ok {
			str := func(k string) string { s, _ := obj[k].(string); return s }
			if s := str("name"); s != "" {
				d.PolicyName = s
			}
			if s := str("description"); s != "" {
				d.Description = s
			}
			d.Message = str("message")
			if d.Message == "" {
				d.Message = d.Description
			}
			if s := str("urn"); s != "" {
				d.Urn = s
			}
			if l, ok := enforcementLevels[str("enforcementLevel")]; ok {
				d.EnforcementLevel = l
			}
			if s, ok := severities[str("severity")]; ok && s != pulumirpc.PolicySeverity_POLICY_SEVERITY_UNSPECIFIED {
				d.Severity = s
			}
		} else {
			d.Message = p.desc.Description
			if v.message != nil {
				if msg := fmt.Sprint(v.message); msg != "" {
					d.Message += "\n" + msg
				}
			}
		}
		resp.Diagnostics = append(resp.Diagnostics, d)
	}
	if r.notApplicable != nil {
		resp.NotApplicable = append(resp.NotApplicable, &pulumirpc.PolicyNotApplicable{
			PolicyName: p.desc.Name, Reason: *r.notApplicable,
		})
	}
	if r.unknown != "" {
		resp.Diagnostics = append(resp.Diagnostics, &pulumirpc.AnalyzeDiagnostic{
			PolicyName:        p.desc.Name,
			PolicyPackName:    pk.desc.Name,
			PolicyPackVersion: pk.version,
			Description:       p.desc.Description,
			Message: fmt.Sprintf("can't run policy '%s' from policy pack '%s@v%s' during preview: %s",
				p.desc.Name, pk.desc.Name, pk.version, r.unknown),
			EnforcementLevel: pulumirpc.EnforcementLevel_ADVISORY,
			Severity:         severities[p.desc.Severity],
		})
	}
}

func (a *analyzer) Remediate(context.Context, *pulumirpc.AnalyzeRequest) (*pulumirpc.RemediateResponse, error) {
	return &pulumirpc.RemediateResponse{}, nil
}

func (a *analyzer) GetAnalyzerInfo(context.Context, *emptypb.Empty) (*pulumirpc.AnalyzerInfo, error) {
	pk := a.pack
	pk.mu.Lock()
	defer pk.mu.Unlock()

	policies := make([]*pulumirpc.PolicyInfo, len(pk.policies))
	for i, p := range pk.policies {
		d := p.desc
		props := map[string]any{}
		var required []string
		if d.ConfigSchema != nil {
			maps.Copy(props, d.ConfigSchema.Properties)
			required = d.ConfigSchema.Required
		}
		props["enforcementLevel"] = map[string]any{
			"type": "string",
			"enum": []any{"advisory", "mandatory", "remediate", "disabled"},
		}
		schema, err := structpb.NewStruct(props)
		if err != nil {
			return nil, fmt.Errorf("policy %q: config schema: %w", d.Name, err)
		}
		var fw *pulumirpc.PolicyComplianceFramework
		if d.Framework != nil {
			fw = &pulumirpc.PolicyComplianceFramework{
				Name: d.Framework.Name, Version: d.Framework.Version,
				Reference: d.Framework.Reference, Specification: d.Framework.Specification,
			}
		}
		kind := pulumirpc.PolicyType_POLICY_TYPE_RESOURCE
		if p.stack {
			kind = pulumirpc.PolicyType_POLICY_TYPE_STACK
		}
		policies[i] = &pulumirpc.PolicyInfo{
			Name:             d.Name,
			DisplayName:      d.DisplayName,
			Description:      d.Description,
			Message:          d.Message,
			EnforcementLevel: enforcementLevels[p.level],
			ConfigSchema:     &pulumirpc.PolicyConfigSchema{Properties: schema, Required: required},
			PolicyType:       kind,
			Severity:         severities[d.Severity],
			Framework:        fw,
			Tags:             d.Tags,
			RemediationSteps: d.RemediationSteps,
			Url:              d.URL,
		}
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].Name < policies[j].Name })

	initial, err := initialConfig(pk.desc.InitialConfig)
	if err != nil {
		return nil, err
	}
	return &pulumirpc.AnalyzerInfo{
		Name:           pk.desc.Name,
		DisplayName:    pk.desc.DisplayName,
		Version:        pk.version,
		SupportsConfig: true,
		Policies:       policies,
		InitialConfig:  initial,
		Description:    pk.description,
		Readme:         pk.desc.Readme,
		Provider:       pk.desc.Provider,
		Tags:           pk.desc.Tags,
		Repository:     pk.desc.Repository,
	}, nil
}

// initialConfig converts a pack's initialConfig: each policy maps to an enforcement level, or to an object of config
// properties that may include one.
func initialConfig(c map[string]any) (map[string]*pulumirpc.PolicyConfig, error) {
	if len(c) == 0 {
		return nil, nil
	}
	out := make(map[string]*pulumirpc.PolicyConfig, len(c))
	for name, v := range c {
		cfg := &pulumirpc.PolicyConfig{}
		props := map[string]any{}
		switch v := v.(type) {
		case string:
			cfg.EnforcementLevel = enforcementLevels[v]
		case map[string]any:
			for k, pv := range v {
				if k == "enforcementLevel" {
					if s, ok := pv.(string); ok {
						cfg.EnforcementLevel = enforcementLevels[s]
					}
					continue
				}
				props[k] = pv
			}
		default:
			continue
		}
		s, err := structpb.NewStruct(props)
		if err != nil {
			return nil, fmt.Errorf("initial config for policy %q: %w", name, err)
		}
		cfg.Properties = s
		out[name] = cfg
	}
	return out, nil
}

func (a *analyzer) Configure(_ context.Context, req *pulumirpc.ConfigureAnalyzerRequest) (*emptypb.Empty, error) {
	pk := a.pack
	pk.mu.Lock()
	defer pk.mu.Unlock()

	for _, p := range pk.policies {
		c, ok := req.GetPolicyConfig()[p.desc.Name]
		if !ok {
			continue
		}
		p.level = levelName(c.GetEnforcementLevel())
		props := c.GetProperties().AsMap()
		delete(props, "enforcementLevel")
		b, err := json.Marshal(props)
		if err != nil {
			return nil, fmt.Errorf("config for policy %q: %w", p.desc.Name, err)
		}
		// JSON.parse gives the policy plain JS values, as the Node.js policy SDK does.
		parse, _ := goja.AssertFunction(pk.rt.Get("JSON").ToObject(pk.rt).Get("parse"))
		cfg, err := parse(goja.Undefined(), pk.rt.ToValue(string(b)))
		if err != nil {
			return nil, fmt.Errorf("config for policy %q: %w", p.desc.Name, err)
		}
		p.config = cfg
	}
	return &emptypb.Empty{}, nil
}

func (a *analyzer) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	a.pack.rt.Interrupt("cancelled")
	return &emptypb.Empty{}, nil
}
