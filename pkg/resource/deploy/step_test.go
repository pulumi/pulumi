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

package deploy

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/stretchr/testify/assert"
)

func TestRawPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		op   display.StepOp
		want string
	}{
		{name: "Same", op: OpSame, want: "  "},
		{name: "Create", op: OpCreate, want: "+ "},
		{name: "Delete", op: OpDelete, want: "- "},
		{name: "Update", op: OpUpdate, want: "~ "},
		{name: "Replace", op: OpReplace, want: "+-"},
		{name: "CreateReplacement", op: OpCreateReplacement, want: "++"},
		{name: "DeleteReplaced", op: OpDeleteReplaced, want: "--"},
		{name: "Read", op: OpRead, want: "> "},
		{name: "ReadReplacement", op: OpReadReplacement, want: ">>"},
		{name: "Refresh", op: OpRefresh, want: "~ "},
		{name: "ReadDiscard", op: OpReadDiscard, want: "< "},
		{name: "DiscardReplaced", op: OpDiscardReplaced, want: "<<"},
		{name: "Import", op: OpImport, want: "= "},
		{name: "ImportReplacement", op: OpImportReplacement, want: "=>"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RawPrefix(tt.op))
		})
	}
	t.Run("panics on unknown", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			RawPrefix("not-a-real-operation")
		})
	})
}

func TestPastTense(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		op   display.StepOp
		want string
	}{
		{"Same", OpSame, "samed"},
		{"Create", OpCreate, "created"},
		{"Replace", OpReplace, "replaced"},
		{"Update", OpUpdate, "updated"},

		// TODO(dixler) consider fixing this.
		{"CreateReplacement", OpCreateReplacement, "create-replacementd"},
		{"ReadReplacement", OpReadReplacement, "read-replacementd"},

		{"Refresh", OpRefresh, "refreshed"},
		{"Read", OpRead, "read"},
		{"ReadDiscard", OpReadDiscard, "discarded"},
		{"DiscardReplaced", OpDiscardReplaced, "discarded"},
		{"Delete", OpDelete, "deleted"},
		{"DeleteReplaced", OpDeleteReplaced, "deleted"},
		{"Import", OpImport, "imported"},
		{"ImportReplacement", OpImportReplacement, "imported"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, PastTense(tt.op))
		})
	}
	t.Run("panics on unknown", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			PastTense("not-a-real-operation")
		})
	})
}
