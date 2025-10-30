// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package autonaming

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveStackExpressions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name		string
		pattern		string
		org		string
		proj		string
		stack		string
		configVals	map[string]string
		configErrs	map[string]error
		expected	string
		expectError	bool
	}{
		{
			name:		"basic variable replacement",
			pattern:	"${organization}-${project}-${stack}",
			org:		"myorg",
			proj:		"myproj",
			stack:		"dev",
			expected:	"myorg-myproj-dev",
		},
		{
			name:		"config value replacement",
			pattern:	"prefix-${config.environment}-suffix",
			configVals: map[string]string{
				"environment": "production",
			},
			expected:	"prefix-production-suffix",
		},
		{
			name:		"multiple replacements",
			pattern:	"${organization}/${project}/${stack}/${config.region}",
			org:		"myorg",
			proj:		"myproj",
			stack:		"dev",
			configVals: map[string]string{
				"region": "us-west-2",
			},
			expected:	"myorg/myproj/dev/us-west-2",
		},
		{
			name:		"missing config value",
			pattern:	"${config.missing}",
			configErrs: map[string]error{
				"missing": fmt.Errorf("no value found for key %q", "missing"),
			},
			expectError:	true,
		},
		{
			name:		"config error",
			pattern:	"${config.secret}",
			configErrs: map[string]error{
				"secret": fmt.Errorf("failed to decrypt value for key %q: unauthorized", "secret"),
			},
			expectError:	true,
		},
		{
			name:		"no replacements needed",
			pattern:	"static-name",
			expected:	"static-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stack := StackContext{
				Organization:	tt.org,
				Project:	tt.proj,
				Stack:		tt.stack,
			}
			eval := &stackPatternEval{
				ctx:	stack,
				getConfigValue: func(key string) (string, error) {
					if err, hasErr := tt.configErrs[key]; hasErr {
						return "", err
					}
					if val, hasVal := tt.configVals[key]; hasVal {
						return val, nil
					}
					return "", fmt.Errorf("unexpected config key: %q", key)
				},
			}

			result, err := eval.resolveStackExpressions(tt.pattern)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
