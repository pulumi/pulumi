package python

import (
	"strings"
	"testing"

	"github.com/blang/semver"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

func parseAndBindProgram(t *testing.T, text, name string, options ...pcl.BindOption) (*pcl.Program, hcl.Diagnostics) {
	parser := syntax.NewParser()
	err := parser.ParseFile(strings.NewReader(text), name)
	if err != nil {
		t.Fatalf("could not read %v: %v", name, err)
	}
	if parser.Diagnostics.HasErrors() {
		t.Fatalf("failed to parse files: %v", parser.Diagnostics)
	}

	options = append(options, pcl.PluginHost(utils.NewHost(testdataPath)))

	program, diags, err := pcl.BindProgram(parser.Files, options...)
	if err != nil {
		t.Fatalf("could not bind program: %v", err)
	}
	return program, diags
}

func TestMakeSafeEnumName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"red", "RED", false},
		{"snake_cased_name", "SNAKE_CASED_NAME", false},
		{"+", "", true},
		{"*", "ASTERISK", false},
		{"0", "ZERO", false},
		{"8.3", "TYPE_NAME_8_3", false},
		{"11", "TYPE_NAME_11", false},
		{"Microsoft-Windows-Shell-Startup", "MICROSOFT_WINDOWS_SHELL_STARTUP", false},
		{"Microsoft.Batch", "MICROSOFT_BATCH", false},
		{"readonly", "READONLY", false},
		{"SystemAssigned, UserAssigned", "SYSTEM_ASSIGNED_USER_ASSIGNED", false},
		{"Dev(NoSLA)_Standard_D11_v2", "DEV_NO_SL_A_STANDARD_D11_V2", false},
		{"Standard_E8as_v4+1TB_PS", "STANDARD_E8AS_V4_1_T_B_PS", false},
		{"Plants'R'Us", "PLANTS_R_US", false},
		{"Pulumi Planters Inc.", "PULUMI_PLANTERS_INC_", false},
		{"ZeroPointOne", "ZERO_POINT_ONE", false},
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

func TestMakePyPiVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"1.2.3", "1.2.3"},
		{"1.2.3+dirty", "1.2.3+dirty"},
		{"1.2.3-alpha123+beta123", "1.2.3a123+beta123"},
		{"1.2.3-rc789", "1.2.3rc789"},
		{"1.2.3-dev321", "1.2.3.dev321"},
		{"1.2.3-post456", "1.2.3.post456"},
		{"1.2.3-posttt456", "1.2.3+posttt456"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			v := semver.MustParse(tt.input)
			actual := pypiVersion(v)
			if tt.expected != actual {
				t.Errorf("expected %q != actual %q", tt.expected, actual)
			}
		})
	}
}
