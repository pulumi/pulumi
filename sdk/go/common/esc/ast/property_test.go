// Copyright 2023, Pulumi Corporation.
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

package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyAccess_String(t *testing.T) {
	tests := []struct {
		name      string
		accessors []PropertyAccessor
		want      string
	}{
		{
			name:      "empty",
			accessors: []PropertyAccessor{},
			want:      "",
		},
		{
			name: "single",
			accessors: []PropertyAccessor{
				&PropertyName{Name: "foo"},
			},
			want: "foo",
		},
		{
			name: "multiple",
			accessors: []PropertyAccessor{
				&PropertyName{Name: "foo"},
				&PropertyName{Name: "bar"},
			},
			want: "foo.bar",
		},
		{
			name: "subscript",
			accessors: []PropertyAccessor{
				&PropertyName{Name: "foo"},
				&PropertySubscript{Index: "bar"},
			},
			want: "foo[\"bar\"]",
		},
		{
			name: "int-subscript",
			accessors: []PropertyAccessor{
				&PropertyName{Name: "foo"},
				&PropertySubscript{Index: 42},
			},
			want: "foo[42]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PropertyAccess{
				Accessors: tt.accessors,
			}
			assert.Equalf(t, tt.want, p.String(), "String()")
		})
	}
}
