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

package pcl

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type WorkflowProgram struct {
	Triggers  []WorkflowTriggerDefinition `hcl:"trigger,block"`
	Steps     []WorkflowStepDefinition    `hcl:"step,block"`
	Jobs      []WorkflowJobDefinition     `hcl:"job,block"`
	Workflows []WorkflowGraph             `hcl:"workflow,block"`

	graphsByName   map[string]WorkflowGraph
	triggersByName map[string]WorkflowTriggerDefinition
	stepsByName    map[string]WorkflowStepDefinition
	jobsByName     map[string]WorkflowJobDefinition
}

type WorkflowGraph struct {
	Name        string                 `hcl:"name,label"`
	TriggerRefs []WorkflowTriggerRef   `hcl:"trigger_ref,block"`
	Triggers    []WorkflowGraphTrigger `hcl:"trigger,block"`
	Jobs        []WorkflowGraphJob     `hcl:"job,block"`
}

type WorkflowTriggerDefinition struct {
	Name     string `hcl:"name,label"`
	Type     string `hcl:"type,optional"`
	Schedule string `hcl:"schedule,optional"`
}

type WorkflowTriggerRef struct {
	Name string `hcl:"name,label"`
}

type WorkflowGraphTrigger struct {
	Name     string `hcl:"name,label"`
	Uses     string `hcl:"uses,optional"`
	Schedule string `hcl:"schedule,optional"`
}

type WorkflowStepDefinition struct {
	Name    string `hcl:"name,label"`
	Command string `hcl:"command,optional"`
	Expr    string `hcl:"expr,optional"`
}

type WorkflowJobDefinition struct {
	Name  string            `hcl:"name,label"`
	Steps []WorkflowJobStep `hcl:"step,block"`
}

type WorkflowJobStep struct {
	Name      string   `hcl:"name,label"`
	Uses      string   `hcl:"uses,optional"`
	Command   string   `hcl:"command,optional"`
	Expr      string   `hcl:"expr,optional"`
	Filter    *bool    `hcl:"filter,optional"`
	DependsOn []string `hcl:"depends_on,optional"`
}

type WorkflowGraphJob struct {
	Name      string            `hcl:"name,label"`
	Uses      string            `hcl:"uses,optional"`
	Filter    *bool             `hcl:"filter,optional"`
	Steps     []WorkflowJobStep `hcl:"step,block"`
	DependsOn []string          `hcl:"depends_on,optional"`
}

func BindWorkflowProgram(programPath string) (*WorkflowProgram, error) {
	parser := hclparse.NewParser()
	hclFile, diags := parser.ParseHCLFile(programPath)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse workflow pcl file %q: %s", programPath, diags.Error())
	}

	var p WorkflowProgram
	decodeDiags := gohcl.DecodeBody(hclFile.Body, nil, &p)
	if decodeDiags.HasErrors() {
		return nil, fmt.Errorf("decode workflow pcl file %q: %s", programPath, decodeDiags.Error())
	}

	p.graphsByName = map[string]WorkflowGraph{}
	for _, graph := range p.Workflows {
		if _, exists := p.graphsByName[graph.Name]; exists {
			return nil, fmt.Errorf("duplicate workflow graph %q", graph.Name)
		}
		p.graphsByName[graph.Name] = graph
	}

	p.triggersByName = map[string]WorkflowTriggerDefinition{}
	for _, trigger := range p.Triggers {
		if _, exists := p.triggersByName[trigger.Name]; exists {
			return nil, fmt.Errorf("duplicate trigger definition %q", trigger.Name)
		}
		p.triggersByName[trigger.Name] = trigger
	}

	p.stepsByName = map[string]WorkflowStepDefinition{}
	for _, step := range p.Steps {
		if _, exists := p.stepsByName[step.Name]; exists {
			return nil, fmt.Errorf("duplicate step definition %q", step.Name)
		}
		p.stepsByName[step.Name] = step
	}

	p.jobsByName = map[string]WorkflowJobDefinition{}
	for _, job := range p.Jobs {
		if _, exists := p.jobsByName[job.Name]; exists {
			return nil, fmt.Errorf("duplicate job definition %q", job.Name)
		}
		p.jobsByName[job.Name] = job
	}

	if len(p.graphsByName) == 0 {
		return nil, fmt.Errorf("no workflow blocks found in %q", programPath)
	}

	return &p, nil
}

func (p *WorkflowProgram) GraphByName(name string) (WorkflowGraph, bool) {
	graph, ok := p.graphsByName[name]
	return graph, ok
}

func (p *WorkflowProgram) TriggerByName(name string) (WorkflowTriggerDefinition, bool) {
	trigger, ok := p.triggersByName[name]
	return trigger, ok
}

func (p *WorkflowProgram) TriggerNames() []string {
	names := make([]string, 0, len(p.triggersByName))
	for name := range p.triggersByName {
		names = append(names, name)
	}
	return names
}

func (p *WorkflowProgram) ResolveTriggerNameFromUse(uses string) (string, string, bool) {
	if uses == "" {
		return "", "", false
	}
	if name, ok := p.triggersByName[uses]; ok {
		return name.Name, uses, true
	}
	pkg, name, ok := splitUseReference(uses)
	if !ok {
		return "", "", false
	}
	_, exists := p.triggersByName[name]
	if !exists {
		return "", "", false
	}
	return pkg, name, true
}

func (p *WorkflowProgram) StepDefinitionForUse(uses string) (WorkflowStepDefinition, bool) {
	if uses == "" {
		return WorkflowStepDefinition{}, false
	}
	if step, ok := p.stepsByName[uses]; ok {
		return step, true
	}
	_, name, ok := splitUseReference(uses)
	if !ok {
		return WorkflowStepDefinition{}, false
	}
	step, exists := p.stepsByName[name]
	return step, exists
}

func (p *WorkflowProgram) JobDefinitionForUse(uses string) (WorkflowJobDefinition, bool) {
	if uses == "" {
		return WorkflowJobDefinition{}, false
	}
	if job, ok := p.jobsByName[uses]; ok {
		return job, true
	}
	_, name, ok := splitUseReference(uses)
	if !ok {
		return WorkflowJobDefinition{}, false
	}
	job, exists := p.jobsByName[name]
	return job, exists
}

func splitUseReference(uses string) (string, string, bool) {
	for i := 0; i < len(uses); i++ {
		if uses[i] != ':' {
			continue
		}
		if i == 0 || i+1 >= len(uses) {
			return "", "", false
		}
		return uses[:i], uses[i+1:], true
	}
	return "", "", false
}
