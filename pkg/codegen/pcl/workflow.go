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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/zclconf/go-cty/cty"
)

type ProgramKind int

const (
	ProgramKindResource ProgramKind = iota
	ProgramKindWorkflow
	ProgramKindMixed
)

type WorkflowProgram struct {
	Triggers  []WorkflowTriggerDefinition `hcl:"trigger,block"`
	Steps     []WorkflowStepDefinition    `hcl:"step,block"`
	Jobs      []WorkflowJobDefinition     `hcl:"job,block"`
	Workflows []WorkflowGraph             `hcl:"workflow,block"`
	Packages  map[string]*schema.PackageDescriptor

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
	Name       string            `hcl:"name,label"`
	InputType  WorkflowInputType `hcl:"-"`
	OutputType string            `hcl:"output_type,optional"`
	Command    string            `hcl:"command,optional"`
	Expr       string            `hcl:"expr,optional"`
}

type WorkflowJobDefinition struct {
	Name      string            `hcl:"name,label"`
	InputType WorkflowInputType `hcl:"-"`
	Expr      string            `hcl:"expr,optional"`
	Steps     []WorkflowJobStep `hcl:"step,block"`
}

type WorkflowInputType struct {
	// Token supports existing token-style declarations:
	// input_type = "pkg:index:Type"
	Token string
	// Fields supports struct-style declarations:
	// input_type = { field = bool, other = string }
	Fields map[string]string
}

func (t WorkflowInputType) TokenOrEmpty() string {
	return t.Token
}

func (t WorkflowInputType) IsStruct() bool {
	return len(t.Fields) > 0
}

type workflowProgramFile struct {
	Triggers  []WorkflowTriggerDefinition `hcl:"trigger,block"`
	Steps     []workflowStepDefinitionRaw `hcl:"step,block"`
	Jobs      []workflowJobDefinitionRaw  `hcl:"job,block"`
	Workflows []WorkflowGraph             `hcl:"workflow,block"`
	Remain    hcl.Body                    `hcl:",remain"`
}

type workflowStepDefinitionRaw struct {
	Name       string         `hcl:"name,label"`
	InputType  hcl.Expression `hcl:"input_type,optional"`
	OutputType string         `hcl:"output_type,optional"`
	Command    string         `hcl:"command,optional"`
	Expr       hcl.Expression `hcl:"expr,optional"`
}

type workflowJobDefinitionRaw struct {
	Name      string               `hcl:"name,label"`
	InputType hcl.Expression       `hcl:"input_type,optional"`
	Expr      hcl.Expression       `hcl:"expr,optional"`
	Steps     []workflowJobStepRaw `hcl:"step,block"`
}

type workflowJobStepRaw struct {
	Name    string         `hcl:"name,label"`
	Uses    string         `hcl:"uses,optional"`
	Command string         `hcl:"command,optional"`
	Expr    hcl.Expression `hcl:"expr,optional"`
	Filter  *bool          `hcl:"filter,optional"`
}

type WorkflowJobStep struct {
	Name    string `hcl:"name,label"`
	Uses    string `hcl:"uses,optional"`
	Command string `hcl:"command,optional"`
	Expr    string `hcl:"expr,optional"`
	Filter  *bool  `hcl:"filter,optional"`
}

type WorkflowGraphJob struct {
	Name   string            `hcl:"name,label"`
	Uses   string            `hcl:"uses,optional"`
	Expr   string            `hcl:"expr,optional"`
	Filter *bool             `hcl:"filter,optional"`
	Steps  []WorkflowJobStep `hcl:"step,block"`
}

func BindWorkflowProgram(programPath string) (*WorkflowProgram, error) {
	source, err := os.ReadFile(programPath)
	if err != nil {
		return nil, fmt.Errorf("read workflow pcl file %q: %w", programPath, err)
	}
	return BindWorkflowSource(map[string]string{programPath: string(source)})
}

func BindWorkflowDirectory(dir string) (*WorkflowProgram, error) {
	source, err := ReadProgramSourcesFromDirectory(dir)
	if err != nil {
		return nil, err
	}
	return BindWorkflowSource(source)
}

func BindWorkflowSource(source map[string]string) (*WorkflowProgram, error) {
	packages, err := bindWorkflowPackageDescriptors(source)
	if err != nil {
		return nil, err
	}

	parser := hclparse.NewParser()
	keys := make([]string, 0, len(source))
	for path := range source {
		keys = append(keys, path)
	}
	sort.Strings(keys)

	var p WorkflowProgram
	p.Packages = packages
	for _, filePath := range keys {
		hclFile, diags := parser.ParseHCL([]byte(source[filePath]), filePath)
		if diags.HasErrors() {
			return nil, fmt.Errorf("parse workflow pcl file %q: %s", filePath, diags.Error())
		}
		var file workflowProgramFile
		decodeDiags := gohcl.DecodeBody(hclFile.Body, nil, &file)
		if decodeDiags.HasErrors() {
			return nil, fmt.Errorf("decode workflow pcl file %q: %s", filePath, decodeDiags.Error())
		}
		p.Triggers = append(p.Triggers, file.Triggers...)
		for _, rawStep := range file.Steps {
			inputType, err := parseWorkflowInputType(rawStep.InputType)
			if err != nil {
				return nil, fmt.Errorf("decode workflow pcl file %q: step %q input_type: %w", filePath, rawStep.Name, err)
			}
			expr, err := parseWorkflowExpr(rawStep.Expr)
			if err != nil {
				return nil, fmt.Errorf("decode workflow pcl file %q: step %q expr: %w", filePath, rawStep.Name, err)
			}
			p.Steps = append(p.Steps, WorkflowStepDefinition{
				Name:       rawStep.Name,
				InputType:  inputType,
				OutputType: rawStep.OutputType,
				Command:    rawStep.Command,
				Expr:       expr,
			})
		}
		for _, rawJob := range file.Jobs {
			inputType, err := parseWorkflowInputType(rawJob.InputType)
			if err != nil {
				return nil, fmt.Errorf("decode workflow pcl file %q: job %q input_type: %w", filePath, rawJob.Name, err)
			}
			expr, err := parseWorkflowExpr(rawJob.Expr)
			if err != nil {
				return nil, fmt.Errorf("decode workflow pcl file %q: job %q expr: %w", filePath, rawJob.Name, err)
			}
			steps := make([]WorkflowJobStep, 0, len(rawJob.Steps))
			for _, rawStep := range rawJob.Steps {
				stepExpr, err := parseWorkflowExpr(rawStep.Expr)
				if err != nil {
					return nil, fmt.Errorf(
						"decode workflow pcl file %q: job %q step %q expr: %w",
						filePath,
						rawJob.Name,
						rawStep.Name,
						err,
					)
				}
				steps = append(steps, WorkflowJobStep{
					Name:    rawStep.Name,
					Uses:    rawStep.Uses,
					Command: rawStep.Command,
					Expr:    stepExpr,
					Filter:  rawStep.Filter,
				})
			}
			p.Jobs = append(p.Jobs, WorkflowJobDefinition{
				Name:      rawJob.Name,
				InputType: inputType,
				Expr:      expr,
				Steps:     steps,
			})
		}
		p.Workflows = append(p.Workflows, file.Workflows...)
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

	if len(p.graphsByName) == 0 && len(p.jobsByName) == 0 && len(p.stepsByName) == 0 && len(p.triggersByName) == 0 {
		return nil, errors.New("no workflow blocks found")
	}

	return &p, nil
}

func bindWorkflowPackageDescriptors(source map[string]string) (map[string]*schema.PackageDescriptor, error) {
	parser := syntax.NewParser()
	keys := make([]string, 0, len(source))
	for path := range source {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	for _, filePath := range keys {
		if filepath.Ext(filePath) != ".pp" {
			continue
		}
		if err := parser.ParseFile(strings.NewReader(source[filePath]), filePath); err != nil {
			return nil, fmt.Errorf("parse workflow pcl file %q: %w", filePath, err)
		}
	}
	if parser.Diagnostics.HasErrors() {
		return nil, fmt.Errorf("parse workflow pcl package blocks: %s", parser.Diagnostics.Error())
	}
	packages, diags := ReadAllPackageDescriptors(parser.Files)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decode workflow pcl package blocks: %s", diags.Error())
	}
	return packages, nil
}

func parseWorkflowInputType(expr hcl.Expression) (WorkflowInputType, error) {
	if expr == nil {
		return WorkflowInputType{}, nil
	}

	// Backward compatible token form:
	// input_type = "pkg:index:Type"
	if v, diags := expr.Value(nil); !diags.HasErrors() {
		if v.IsNull() {
			return WorkflowInputType{}, nil
		}
		if v.Type() == cty.String {
			return WorkflowInputType{Token: v.AsString()}, nil
		}
	}

	obj, ok := expr.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return WorkflowInputType{}, fmt.Errorf("must be string token or object field map")
	}

	fields := map[string]string{}
	for _, item := range obj.Items {
		key, err := parseTypeNameExpr(item.KeyExpr)
		if err != nil {
			return WorkflowInputType{}, fmt.Errorf("invalid field name: %w", err)
		}
		if key == "" {
			return WorkflowInputType{}, fmt.Errorf("field name must not be empty")
		}

		typeName, err := parseTypeNameExpr(item.ValueExpr)
		if err != nil {
			return WorkflowInputType{}, fmt.Errorf("invalid type for field %q: %w", key, err)
		}
		if typeName == "" {
			return WorkflowInputType{}, fmt.Errorf("invalid type for field %q", key)
		}
		fields[key] = typeName
	}
	return WorkflowInputType{Fields: fields}, nil
}

func parseTypeNameExpr(expr hcl.Expression) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("missing expression")
	}
	if v, diags := expr.Value(nil); !diags.HasErrors() && v.Type() == cty.String {
		return strings.TrimSpace(v.AsString()), nil
	}
	traversal, diags := hcl.AbsTraversalForExpr(expr)
	if diags.HasErrors() {
		return "", fmt.Errorf("expected identifier or string")
	}
	if len(traversal) == 0 {
		return "", fmt.Errorf("empty traversal")
	}
	root, ok := traversal[0].(hcl.TraverseRoot)
	if !ok {
		return "", fmt.Errorf("expected identifier")
	}
	return strings.TrimSpace(root.Name), nil
}

func parseWorkflowExpr(expr hcl.Expression) (string, error) {
	if expr == nil {
		return "", nil
	}
	if v, diags := expr.Value(nil); !diags.HasErrors() {
		if v.IsNull() {
			return "", nil
		}
		if v.Type() == cty.String {
			return strings.TrimSpace(v.AsString()), nil
		}
	}
	switch e := expr.(type) {
	case *hclsyntax.UnaryOpExpr:
		if e.Op == hclsyntax.OpLogicalNot {
			inner, err := parseWorkflowTraversalExpr(e.Val)
			if err != nil {
				return "", err
			}
			return "!" + inner, nil
		}
	case *hclsyntax.TemplateExpr:
		var b strings.Builder
		for _, part := range e.Parts {
			if v, diags := part.Value(nil); !diags.HasErrors() && v.Type() == cty.String {
				b.WriteString(v.AsString())
				continue
			}
			partExpr, err := parseWorkflowExpr(part)
			if err != nil {
				return "", err
			}
			b.WriteString("${")
			b.WriteString(partExpr)
			b.WriteString("}")
		}
		return strings.TrimSpace(b.String()), nil
	}
	return parseWorkflowTraversalExpr(expr)
}

func parseWorkflowTraversalExpr(expr hcl.Expression) (string, error) {
	traversal, diags := hcl.AbsTraversalForExpr(expr)
	if diags.HasErrors() {
		return "", fmt.Errorf("must be string, identifier, traversal, or !traversal")
	}
	if len(traversal) == 0 {
		return "", fmt.Errorf("empty expression")
	}
	root, ok := traversal[0].(hcl.TraverseRoot)
	if !ok {
		return "", fmt.Errorf("expected identifier")
	}
	parts := []string{root.Name}
	for _, step := range traversal[1:] {
		attr, ok := step.(hcl.TraverseAttr)
		if !ok {
			return "", fmt.Errorf("unsupported traversal")
		}
		parts = append(parts, attr.Name)
	}
	return strings.Join(parts, "."), nil
}

func DetectProgramKindFromDirectory(dir string) (ProgramKind, error) {
	source, err := ReadProgramSourcesFromDirectory(dir)
	if err != nil {
		return ProgramKindResource, err
	}
	return DetectProgramKindFromSource(source)
}

func DetectProgramKindFromSource(source map[string]string) (ProgramKind, error) {
	hasWorkflowBlocks := false
	hasResourceBlocks := false

	for filePath, contents := range source {
		hclFile, diags := hclsyntax.ParseConfig([]byte(contents), filePath, hcl.Pos{})
		if diags.HasErrors() {
			return ProgramKindResource, fmt.Errorf("parse pcl file %q: %s", filePath, diags.Error())
		}
		body, ok := hclFile.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, block := range body.Blocks {
			switch block.Type {
			case "workflow", "graph", "trigger", "job", "step":
				hasWorkflowBlocks = true
			case "resource", "config", "output", "local", "component", "pulumi":
				hasResourceBlocks = true
			}
		}
	}

	switch {
	case hasWorkflowBlocks && hasResourceBlocks:
		return ProgramKindMixed, nil
	case hasWorkflowBlocks:
		return ProgramKindWorkflow, nil
	default:
		return ProgramKindResource, nil
	}
}

func ReadProgramSourcesFromDirectory(dir string) (map[string]string, error) {
	source := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".pp" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relative path %q: %w", path, err)
		}
		source[filepath.ToSlash(relPath)] = string(contents)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(source) == 0 {
		return nil, fmt.Errorf("no .pp files found in %q", dir)
	}
	return source, nil
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

func (p *WorkflowProgram) StepByName(name string) (WorkflowStepDefinition, bool) {
	step, ok := p.stepsByName[name]
	return step, ok
}

func (p *WorkflowProgram) StepNames() []string {
	names := make([]string, 0, len(p.stepsByName))
	for name := range p.stepsByName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

func (p *WorkflowProgram) JobByName(name string) (WorkflowJobDefinition, bool) {
	job, ok := p.jobsByName[name]
	return job, ok
}

func (p *WorkflowProgram) JobNames() []string {
	names := make([]string, 0, len(p.jobsByName))
	for name := range p.jobsByName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
