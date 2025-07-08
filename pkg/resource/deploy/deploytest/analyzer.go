// Copyright 2016-2018, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Analyzer struct {
	Info plugin.AnalyzerInfo

	AnalyzeF      func(r plugin.AnalyzerResource) ([]plugin.AnalyzeDiagnostic, error)
	AnalyzeStackF func(resources []plugin.AnalyzerStackResource) ([]plugin.AnalyzeDiagnostic, error)
	RemediateF    func(r plugin.AnalyzerResource) ([]plugin.Remediation, error)

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

func (a *Analyzer) Analyze(r plugin.AnalyzerResource) ([]plugin.AnalyzeDiagnostic, error) {
	if a.AnalyzeF != nil {
		return a.AnalyzeF(r)
	}
	return nil, nil
}

func (a *Analyzer) AnalyzeStack(resources []plugin.AnalyzerStackResource) ([]plugin.AnalyzeDiagnostic, error) {
	if a.AnalyzeStackF != nil {
		return a.AnalyzeStackF(resources)
	}
	return nil, nil
}

func (a *Analyzer) Remediate(r plugin.AnalyzerResource) ([]plugin.Remediation, error) {
	if a.RemediateF != nil {
		return a.RemediateF(r)
	}
	return nil, nil
}

func (a *Analyzer) GetAnalyzerInfo() (plugin.AnalyzerInfo, error) {
	return a.Info, nil
}

func (a *Analyzer) GetPluginInfo() (workspace.PluginInfo, error) {
	return workspace.PluginInfo{
		Kind: apitype.AnalyzerPlugin,
		Name: a.Info.Name,
	}, nil
}

func (a *Analyzer) Configure(policyConfig map[string]plugin.AnalyzerPolicyConfig) error {
	if a.ConfigureF != nil {
		return a.ConfigureF(policyConfig)
	}
	return nil
}

func (a *Analyzer) Cancel(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	if a.CancelF != nil {
		a.CancelF()
	}
	return &emptypb.Empty{}, nil
}