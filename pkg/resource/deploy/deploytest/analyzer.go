// Copyright 2016-2025, Pulumi Corporation.
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

package deploytest

import (
	"context"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type Analyzer struct {
	Info plugin.AnalyzerInfo

	AnalyzeF      func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error)
	AnalyzeStackF func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error)
	RemediateF    func(r plugin.AnalyzerResource) (plugin.RemediateResponse, error)

	ConfigureF func(policyConfig map[string]plugin.AnalyzerPolicyConfig) error
	CancelF    func() error
}

var _ = plugin.Analyzer((*Analyzer)(nil))

func (a *Analyzer) Close() error {
	return nil
}

func (a *Analyzer) Name() tokens.QName {
	return tokens.QName(a.Info.Name)
}

func (a *Analyzer) Analyze(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
	if a.AnalyzeF != nil {
		return a.AnalyzeF(r)
	}
	return plugin.AnalyzeResponse{}, nil
}

func (a *Analyzer) AnalyzeStack(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
	if a.AnalyzeStackF != nil {
		return a.AnalyzeStackF(resources)
	}
	return plugin.AnalyzeResponse{}, nil
}

func (a *Analyzer) Remediate(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
	if a.RemediateF != nil {
		return a.RemediateF(r)
	}
	return plugin.RemediateResponse{}, nil
}

func (a *Analyzer) GetAnalyzerInfo() (plugin.AnalyzerInfo, error) {
	return a.Info, nil
}

func (a *Analyzer) GetPluginInfo() (plugin.PluginInfo, error) {
	var version *semver.Version
	if a.Info.Version != "" {
		sv, err := semver.ParseTolerant(a.Info.Version)
		if err != nil {
			return plugin.PluginInfo{}, err
		}
		version = &sv
	}

	return plugin.PluginInfo{
		Version: version,
	}, nil
}

func (a *Analyzer) Configure(policyConfig map[string]plugin.AnalyzerPolicyConfig) error {
	if a.ConfigureF != nil {
		return a.ConfigureF(policyConfig)
	}
	return nil
}

func (a *Analyzer) Cancel(ctx context.Context) error {
	if a.CancelF != nil {
		return a.CancelF()
	}
	return nil
}
