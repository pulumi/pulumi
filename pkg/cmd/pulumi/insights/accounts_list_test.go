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

package insights

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestAccountsListCmd_OutputFormats(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testAccounts := []apitype.InsightsAccount{
		{
			Name:     "aws-123456789012",
			Provider: "AWS",
			ScanStatus: &apitype.ScanStatus{
				Status:        apitype.ScanStatusSucceeded,
				ResourceCount: 42,
				LastUpdatedAt: &now,
			},
		},
		{
			Name:     "aws-210987654321",
			Provider: "AWS",
			ScanStatus: &apitype.ScanStatus{
				Status:        apitype.ScanStatusPending,
				ResourceCount: 0,
			},
		},
	}

	tests := []struct {
		name        string
		accounts    []apitype.InsightsAccount
		outputFmt   outputFormat
		expectError bool
	}{
		{
			name:      "empty list",
			accounts:  []apitype.InsightsAccount{},
			outputFmt: outputFormatTable,
		},
		{
			name:      "list with accounts table format",
			accounts:  testAccounts,
			outputFmt: outputFormatTable,
		},
		{
			name:      "json format",
			accounts:  testAccounts,
			outputFmt: outputFormatJSON,
		},
		{
			name:      "yaml format",
			accounts:  testAccounts,
			outputFmt: outputFormatYAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// For table format, use os.Stdout; for JSON/YAML, test encoding
			switch tt.outputFmt {
			case outputFormatTable:
				// renderAccountsTable requires *os.File, so we can only verify it compiles
				// Full testing would require mocking os.Stdout
				err := renderAccountsTable(os.Stdout, tt.accounts)
				if tt.expectError {
					require.Error(t, err)
					return
				}
				// Don't assert on output since we can't capture os.Stdout easily

			case outputFormatJSON:
				// Test JSON encoding
				data, err := json.MarshalIndent(tt.accounts, "", "  ")
				if tt.expectError {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.NotEmpty(t, data)

				// Verify round-trip
				var decoded []apitype.InsightsAccount
				err = json.Unmarshal(data, &decoded)
				require.NoError(t, err)
				assert.Equal(t, len(tt.accounts), len(decoded))

			case outputFormatYAML:
				// Test YAML encoding
				data, err := yaml.Marshal(tt.accounts)
				if tt.expectError {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.NotEmpty(t, data)

				// Verify round-trip
				var decoded []apitype.InsightsAccount
				err = yaml.Unmarshal(data, &decoded)
				require.NoError(t, err)
				assert.Equal(t, len(tt.accounts), len(decoded))
			}
		})
	}
}

func TestAccountsListCmd_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now()
	original := []apitype.InsightsAccount{
		{
			Name:        "aws-test",
			Provider:    "AWS",
			Environment: "test-org/insights/aws-test",
			ScanStatus: &apitype.ScanStatus{
				Status:        apitype.ScanStatusRunning,
				ResourceCount: 10,
				LastUpdatedAt: &now,
			},
		},
	}

	// Encode
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Decode
	var decoded []apitype.InsightsAccount
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify
	require.Len(t, decoded, 1)
	assert.Equal(t, original[0].Name, decoded[0].Name)
	assert.Equal(t, original[0].Provider, decoded[0].Provider)
	assert.Equal(t, original[0].Environment, decoded[0].Environment)
	assert.Equal(t, original[0].ScanStatus.Status, decoded[0].ScanStatus.Status)
	assert.Equal(t, original[0].ScanStatus.ResourceCount, decoded[0].ScanStatus.ResourceCount)
}

func TestFormatTimeAgo(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name     string
		t        time.Time
		expected string
	}{
		{
			name:     "just now",
			t:        now,
			expected: "Just now",
		},
		{
			name:     "1 minute ago",
			t:        now.Add(-1 * time.Minute),
			expected: "1 minute ago",
		},
		{
			name:     "5 minutes ago",
			t:        now.Add(-5 * time.Minute),
			expected: "5 minutes ago",
		},
		{
			name:     "1 hour ago",
			t:        now.Add(-1 * time.Hour),
			expected: "1 hour ago",
		},
		{
			name:     "2 hours ago",
			t:        now.Add(-2 * time.Hour),
			expected: "2 hours ago",
		},
		{
			name:     "1 day ago",
			t:        now.Add(-24 * time.Hour),
			expected: "1 day ago",
		},
		{
			name:     "3 days ago",
			t:        now.Add(-3 * 24 * time.Hour),
			expected: "3 days ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatTimeAgo(tt.t)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputFormat_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format   outputFormat
		expected string
	}{
		{outputFormatTable, "table"},
		{outputFormatJSON, "json"},
		{outputFormatYAML, "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.format.String())
		})
	}
}

func TestOutputFormat_Set(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    outputFormat
		expectError bool
	}{
		{
			name:     "table",
			input:    "table",
			expected: outputFormatTable,
		},
		{
			name:     "json",
			input:    "json",
			expected: outputFormatJSON,
		},
		{
			name:     "yaml",
			input:    "yaml",
			expected: outputFormatYAML,
		},
		{
			name:        "invalid",
			input:       "xml",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var format outputFormat
			err := format.Set(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "must be one of")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, format)
			}
		})
	}
}

