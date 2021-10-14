// Copyright 2016-2021, Pulumi Corporation.
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

package pcl

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

func prettyPrint(expr model.Expression) string {
	buf := &bytes.Buffer{}
	ctx := &prettyPrintContext{writer: buf}
	res := ctx.prettyPrintTo(expr)
	fmt.Fprintf(buf, "%s\n", res)
	return string(buf.Bytes())
}

type prettyPrintContext struct {
	idCounter int
	writer    io.Writer
}

func (ctx *prettyPrintContext) gensym() string {
	n := ctx.idCounter
	ctx.idCounter++
	return fmt.Sprintf("X%d", n)
}

func (ctx *prettyPrintContext) prettyPrintTo(expr model.Expression) string {
	switch e := expr.(type) {

	case *model.FunctionCallExpression:
		var args []string
		for _, arg := range e.Args {
			args = append(args, ctx.prettyPrintTo(arg))
		}
		res := ctx.gensym()
		fmt.Fprintf(ctx.writer, "(setq %s (%s %s))\n", res, e.Name, strings.Join(args, " "))
		return res

	case *model.ObjectConsExpression:

		var keys []string
		var values []string

		for _, item := range e.Items {
			keys = append(keys, ctx.prettyPrintTo(item.Key))
			values = append(values, ctx.prettyPrintTo(item.Value))
		}

		res := ctx.gensym()

		var args []string
		for i, k := range keys {
			v := values[i]
			args = append(args, fmt.Sprintf("(%s %s)", k, v))
		}

		fmt.Fprintf(ctx.writer, "(setq %s (new-object %s))\n", res, strings.Join(args, " "))
		return res

	case *model.LiteralValueExpression:
		res := ctx.gensym()
		fmt.Fprintf(ctx.writer, "(setq %s <lit:%v>)\n", res, expr)
		return res

	case *model.ScopeTraversalExpression:
		res := ctx.gensym()

		var travs []string
		for _, trav := range e.Traversal.SimpleSplit().Rel {
			travs = append(travs, spew.Sdump(trav))
		}

		dump := spew.Sdump(expr)
		fmt.Fprintf(ctx.writer, "traversal dump: %s\n", dump)
		fmt.Fprintf(ctx.writer, "(setq %s (traverse-scope [root: %s] %s))\n", res,
			e.RootName,
			strings.Join(travs, " "))
		return res

	case *model.TemplateExpression:
		res := ctx.gensym()
		fmt.Fprintf(ctx.writer, "(setq %s <template>)\n", res)
		return res

	default:
		panic(fmt.Sprintf("Unhandled: %v", reflect.TypeOf(e)))
	}
}
