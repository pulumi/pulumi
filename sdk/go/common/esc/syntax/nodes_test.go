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

package syntax

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNilNodeSyntax(t *testing.T) {
	var n *node
	assert.Equal(t, NoSyntax, n.Syntax())
}

func TestNilSyntx(t *testing.T) {
	n := NullSyntax(nil)
	assert.Equal(t, NoSyntax, n.Syntax())
}

func TestBooleanString(t *testing.T) {
	assert.Equal(t, "false", Boolean(false).String())
	assert.Equal(t, "true", Boolean(true).String())
}

func TestGoString(t *testing.T) {
	assert.Equal(t, "syntax.Null()", Null().GoString())
	assert.Equal(t, "syntax.Boolean(true)", Boolean(true).GoString())
	assert.Equal(t, "syntax.Number(json.Number(\"42\"))", Number(42).GoString())
	assert.Equal(t, "syntax.String(\"esc\")", String("esc").GoString())
	assert.Equal(t, "syntax.Array(syntax.String(\"blue\"), syntax.Number(json.Number(\"42\")))",
		Array(String("blue"), Number(42)).GoString())
	assert.Equal(t,
		"syntax.Object(syntax.ObjectProperty(syntax.String(\"hello\"), syntax.String(\"world\")), syntax.ObjectProperty(syntax.String(\"adieu\"), syntax.String(\"monde cruel\")))",
		Object(ObjectProperty(String("hello"), String("world")), ObjectProperty(String("adieu"), String("monde cruel"))).GoString())
}

func TestAsNumber(t *testing.T) {
	assert.Equal(t, json.Number("-42"), AsNumber(int(-42)))
	assert.Equal(t, json.Number("-42"), AsNumber(int8(-42)))
	assert.Equal(t, json.Number("-42"), AsNumber(int16(-42)))
	assert.Equal(t, json.Number("-42"), AsNumber(int32(-42)))
	assert.Equal(t, json.Number("-42"), AsNumber(int64(-42)))
	assert.Equal(t, json.Number("42"), AsNumber(uint(42)))
	assert.Equal(t, json.Number("42"), AsNumber(uint8(42)))
	assert.Equal(t, json.Number("42"), AsNumber(uint16(42)))
	assert.Equal(t, json.Number("42"), AsNumber(uint32(42)))
	assert.Equal(t, json.Number("42"), AsNumber(uint64(42)))
	assert.Equal(t, json.Number("3.14"), AsNumber(float32(3.14)))
	assert.Equal(t, json.Number("3.14"), AsNumber(float64(3.14)))
	assert.Equal(t, json.Number("bad"), AsNumber(json.Number("bad")))
}

func TestArraySetIndex(t *testing.T) {
	a := Array(String("hello"))
	a.SetIndex(0, Number(42))
	assert.Equal(t, a.Index(0), Number(42))
}
