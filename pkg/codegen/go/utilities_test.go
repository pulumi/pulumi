package gen

import (
	"testing"
)

func TestMakeSafeEnumName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"+", "", true},
		{"*", "TypeNameAsterisk", false},
		{"0", "TypeNameZero", false},
		{"8.3", "TypeName_8_3", false},
		{"11", "TypeName_11", false},
		{"Microsoft-Windows-Shell-Startup", "TypeName_Microsoft_Windows_Shell_Startup", false},
		{"Microsoft.Batch", "TypeName_Microsoft_Batch", false},
		{"readonly", "TypeNameReadonly", false},
		{"SystemAssigned, UserAssigned", "TypeName_SystemAssigned_UserAssigned", false},
		{"Dev(NoSLA)_Standard_D11_v2", "TypeName_Dev_NoSLA_Standard_D11_v2", false},
		{"Standard_E8as_v4+1TB_PS", "TypeName_Standard_E8as_v4_1TB_PS", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
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
	tests := []struct {
		input    string
		expected string
	}{
		{"&opts0", "&opts0"},
		{"8", "_8"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := makeValidIdentifier(tt.input); got != tt.expected {
				t.Errorf("makeValidIdentifier() = %v, want %v", got, tt.expected)
			}
		})
	}
}
