// Copyright 2021-2024, Pulumi Corporation.
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

package gen

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

type spillFunc func(x model.Expression) (string, model.Expression, bool)

type spillTemp struct {
	Kind     string
	Variable *model.Variable
	Value    model.Expression
}

type spills struct {
	counts map[string]int
}

func (s *spills) newTemp(kind string, value model.Expression) *spillTemp {
	i := s.counts[kind]
	s.counts[kind] = i + 1

	v := &model.Variable{
		Name:         fmt.Sprintf("%s%d", kind, i),
		VariableType: value.Type(),
	}
	return &spillTemp{
		Variable: v,
		Value:    value,
	}
}

type spiller struct {
	spills *spills

	temps    []*spillTemp
	spill    spillFunc
	disabled bool
}

func (s *spiller) preVisit(x model.Expression) (model.Expression, hcl.Diagnostics) {
	_, isfn := x.(*model.AnonymousFunctionExpression)
	if isfn {
		s.disabled = true
	}
	return x, nil
}

func (s *spiller) postVisit(x model.Expression) (model.Expression, hcl.Diagnostics) {
	_, isfn := x.(*model.AnonymousFunctionExpression)
	if isfn {
		s.disabled = false
	} else if !s.disabled {
		if kind, value, ok := s.spill(x); ok {
			t := s.spills.newTemp(kind, value)
			s.temps = append(s.temps, t)
			return model.VariableReference(t.Variable), nil
		}
	}
	return x, nil
}

func (g *generator) rewriteSpills(
	x model.Expression, spill spillFunc,
) (model.Expression, []*spillTemp, hcl.Diagnostics) {
	spiller := &spiller{
		spills: g.spills,
		spill:  spill,
	}
	x, diags := model.VisitExpression(x, spiller.preVisit, spiller.postVisit)
	return x, spiller.temps, diags
}
