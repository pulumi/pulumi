// Copyright 2020-2024, Pulumi Corporation.
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

package dotnet

import (
	"testing"
)

func TestMakeSafeEnumName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"+", "", true},
		{"*", "Asterisk", false},
		{"0", "Zero", false},
		{"8.3", "TypeName_8_3", false},
		{"11", "TypeName_11", false},
		{"Microsoft-Windows-Shell-Startup", "Microsoft_Windows_Shell_Startup", false},
		{"Microsoft.Batch", "Microsoft_Batch", false},
		{"readonly", "@Readonly", false},
		{"SystemAssigned, UserAssigned", "SystemAssigned_UserAssigned", false},
		{"Dev(NoSLA)_Standard_D11_v2", "Dev_NoSLA_Standard_D11_v2", false},
		{"Standard_E8as_v4+1TB_PS", "Standard_E8as_v4_1TB_PS", false},
		{"Equals", "EqualsValue", false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got, err := makeSafeEnumName(tt.input, "TypeName")
			if (err != nil) != tt.wantErr {
				t.Errorf("makeSafeEnumName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("makeSafeEnumName() got = %v, want %v", got, tt.expected)
			}
		})
	}
}

func Test_makeValidIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"@default", "@default"},
		{"8", "_8"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := makeValidIdentifier(tt.input); got != tt.expected {
				t.Errorf("makeValidIdentifier() = %v, want %v", got, tt.expected)
			}
		})
	}
}
