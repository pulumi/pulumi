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

package authhelpers

import (
	"net/url"
	"testing"
)

func TestAssumeRoleFromURLParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		output  []AssumeRoleConfig
		wantErr bool
	}{
		{
			name:    "empty param returns nil",
			input:   "",
			output:  nil,
			wantErr: false,
		},
		{
			name:  "single role with all fields",
			input: `[{"roleArn":"arn:aws:iam::123456789012:role/Role1","externalId":"ext1","sessionName":"sess1"}]`,
			output: []AssumeRoleConfig{
				{
					RoleArn:     "arn:aws:iam::123456789012:role/Role1",
					ExternalID:  ptr("ext1"),
					SessionName: "sess1",
				},
			},
			wantErr: false,
		},
		{
			name:  "single role with minimal fields",
			input: `[{"roleArn":"arn:aws:iam::123456789012:role/Role1"}]`,
			output: []AssumeRoleConfig{
				{
					RoleArn: "arn:aws:iam::123456789012:role/Role1",
				},
			},
			wantErr: false,
		},
		{
			name:  "multiple roles chained",
			input: `[{"roleArn":"arn:aws:iam::111111111111:role/Role1"},{"roleArn":"arn:aws:iam::222222222222:role/Role2"}]`,
			output: []AssumeRoleConfig{
				{
					RoleArn: "arn:aws:iam::111111111111:role/Role1",
				},
				{
					RoleArn: "arn:aws:iam::222222222222:role/Role2",
				},
			},
			wantErr: false,
		},
		{
			name:    "empty roleArn returns error",
			input:   `[{"roleArn":""}]`,
			output:  nil,
			wantErr: true,
		},
		{
			name:    "invalid ARN returns error",
			input:   `[{"roleArn":"invalid-arn"}]`,
			output:  nil,
			wantErr: true,
		},
		{
			name:    "invalid JSON returns error",
			input:   `not json`,
			output:  nil,
			wantErr: true,
		},
		{
			name:    "invalid JSON array returns error",
			input:   `{}`,
			output:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			q := url.Values{}
			if tt.input != "" {
				q.Set("assumeRoles", tt.input)
			}

			output, err := AssumeRoleFromURLParams(q)
			if tt.wantErr {
				if err == nil {
					t.Errorf("AssumeRoleFromURLParams() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("AssumeRoleFromURLParams() unexpected error: %v", err)
				return
			}

			if len(tt.output) != len(output) {
				t.Errorf("AssumeRoleFromURLParams() expected %d roles, got %d", len(tt.output), len(output))
				return
			}

			for i, role := range tt.output {
				if role.RoleArn != output[i].RoleArn {
					t.Errorf("AssumeRoleFromURLParams() role[%d].RoleArn expected %q, got %q", i, role.RoleArn, output[i].RoleArn)
				}
				if role.ExternalID != nil {
					if output[i].ExternalID == nil || *role.ExternalID != *output[i].ExternalID {
						t.Errorf("role[%d].ExternalID expected %q, got %v", i, *role.ExternalID, *output[i].ExternalID)
					}
				} else if output[i].ExternalID != nil {
					t.Errorf("role[%d].ExternalID expected nil, got %v", i, *output[i].ExternalID)
				}
				if role.SessionName != output[i].SessionName {
					t.Errorf("role[%d].SessionName expected %q, got %q", i, role.SessionName, output[i].SessionName)
				}
			}
		})
	}
}

func TestAssumeRoleFromURLParams_URLDecoded(t *testing.T) {
	t.Parallel()

	// Test URL-encoded JSON (as it would come from a URL)
	input := url.QueryEscape(`[{"roleArn":"arn:aws:iam::123456789012:role/Role1","externalId":"ext1"}]`)

	q := url.Values{}
	q.Set("assumeRoles", input)

	output, err := AssumeRoleFromURLParams(q)
	if err != nil {
		t.Errorf("AssumeRoleFromURLParams() unexpected error: %v", err)
		return
	}

	if len(output) != 1 {
		t.Errorf("AssumeRoleFromURLParams() expected 1 role, got %d", len(output))
		return
	}

	if output[0].RoleArn != "arn:aws:iam::123456789012:role/Role1" {
		want := "arn:aws:iam::123456789012:role/Role1"
		t.Errorf("expected roleArn %q, got %q", want, output[0].RoleArn)
	}
}

func ptr[T any](v T) *T {
	return &v
}
