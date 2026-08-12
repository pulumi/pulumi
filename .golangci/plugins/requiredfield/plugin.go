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

// Package requiredfield wraps go.abhg.dev/requiredfield as a golangci-lint
// module plugin. Upstream exposes a standard analysis.Analyzer but does not
// itself register with the plugin-module-register package, so we adapt it
// here. See .custom-gcl.yml for how the plugin is wired into the custom
// golangci-lint binary.
package requiredfield

import (
	"github.com/golangci/plugin-module-register/register"
	"go.abhg.dev/requiredfield"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("requiredfield", New)
}

func New(any) (register.LinterPlugin, error) {
	return &plugin{}, nil
}

type plugin struct{}

func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{requiredfield.Analyzer}, nil
}

func (p *plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
