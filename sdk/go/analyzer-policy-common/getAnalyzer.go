// Copyright 2016-2023, Pulumi Corporation.
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

package policyAnalyzer

import (
	"context"
	"fmt"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"os"
)

type GetAnalyzerConfig struct {
	CompileConfig *CompileConfig

	PolicyPackPath string
}

type GetAnalyzerFunc func(context.Context, *GetAnalyzerConfig) (plugin.Analyzer, error)

func GetAnalyzerSimple(analyzer plugin.Analyzer) GetAnalyzerFunc {
	return func(ctx context.Context, config *GetAnalyzerConfig) (plugin.Analyzer, error) {
		return analyzer, nil
	}
}

func GetAnalyzerWithCompilerFunc(f CompileProgramFunc) GetAnalyzerFunc {
	return func(ctx context.Context, config *GetAnalyzerConfig) (plugin.Analyzer, error) {

		// The current directory is the policy directory
		program, err := f(config.CompileConfig)
		if err != nil {
			return nil, fmt.Errorf("could not compile policy: %w", err)
		}

		defSink := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{
			Color: colors.Raw, // TODO
		})

		pctx, err := plugin.NewContextWithContext(
			ctx,
			defSink, defSink, nil,
			"pwd", "root", nil, false,
			nil, nil, nil)

		if err != nil {

		}

		hostServerAddr := "hostServerAddr"

		name := tokens.QName("policy-proxy")

		plug, err := plugin.NewPolicyAnalyzerWithExe(hostServerAddr, pctx, name, config.PolicyPackPath, program.Program, nil)
		if err != nil {
			return nil, fmt.Errorf("could not start policy analyzer: %w", err)
		}
		return plug, nil
	}
}
