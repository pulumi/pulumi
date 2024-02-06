// Copyright 2024, Pulumi Corporation.
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

	"github.com/pulumi/esc/syntax"
	"github.com/stretchr/testify/assert"
)

func TestExprError(t *testing.T) {
	type args struct {
		expr    Expr
		summary string
	}
	var ss *StringExpr
	tests := []struct {
		name string
		args args
		want *syntax.Diagnostic
	}{
		{"nil *StringExpr",
			args{
				ss, "",
			}, syntax.Error(nil, "", "")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, ExprError(tt.args.expr, tt.args.summary), "ExprError(%v, %v)", tt.args.expr, tt.args.summary)
		})
	}
}
