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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBindWorkflowProgram(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "example.pp")
	program := `
trigger "cron" {
  schedule = "* * * * *"
}

step "build" {
  command = "echo build"
}

job "compile" {
  step "build" {
    uses = "build"
  }
}

workflow "main" {
  trigger "cron" {
    uses = "example:cron"
  }
  job "compile" {
    uses = "compile"
  }
}
`
	if err := os.WriteFile(path, []byte(program), 0o600); err != nil {
		t.Fatalf("write program: %v", err)
	}

	bound, err := BindWorkflowProgram(path)
	if err != nil {
		t.Fatalf("bind failed: %v", err)
	}
	if _, ok := bound.GraphByName("main"); !ok {
		t.Fatalf("expected graph main")
	}
	if _, ok := bound.StepDefinitionForUse("example:build"); !ok {
		t.Fatalf("expected resolved step definition")
	}
	if _, ok := bound.JobDefinitionForUse("example:compile"); !ok {
		t.Fatalf("expected resolved job definition")
	}
	if _, resolved, ok := bound.ResolveTriggerNameFromUse("example:cron"); !ok || resolved != "cron" {
		t.Fatalf("expected trigger use to resolve to local trigger name")
	}
}

func TestBindWorkflowProgramDuplicateGraph(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pp")
	program := `
workflow "main" {}
workflow "main" {}
`
	if err := os.WriteFile(path, []byte(program), 0o600); err != nil {
		t.Fatalf("write program: %v", err)
	}

	_, err := BindWorkflowProgram(path)
	if err == nil {
		t.Fatalf("expected duplicate graph error")
	}
	if !strings.Contains(err.Error(), "duplicate workflow graph") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDetectProgramKindFromSource(t *testing.T) {
	t.Parallel()

	kind, err := DetectProgramKindFromSource(map[string]string{
		"main.pp": `workflow "main" {}`,
	})
	if err != nil {
		t.Fatalf("detect kind failed: %v", err)
	}
	if kind != ProgramKindWorkflow {
		t.Fatalf("expected workflow kind, got %v", kind)
	}

	kind, err = DetectProgramKindFromSource(map[string]string{
		"main.pp": `resource r "random:index/randomPet:RandomPet" {}`,
	})
	if err != nil {
		t.Fatalf("detect kind failed: %v", err)
	}
	if kind != ProgramKindResource {
		t.Fatalf("expected resource kind, got %v", kind)
	}

	kind, err = DetectProgramKindFromSource(map[string]string{
		"main.pp": `resource r "random:index/randomPet:RandomPet" {}
workflow "main" {}`,
	})
	if err != nil {
		t.Fatalf("detect kind failed: %v", err)
	}
	if kind != ProgramKindMixed {
		t.Fatalf("expected mixed kind, got %v", kind)
	}
}
