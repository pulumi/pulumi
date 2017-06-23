// Copyright 2016-2017, Pulumi Corporation
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

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/tokens"
)

func TestURNRoundTripping(t *testing.T) {
	ns := tokens.QName("namespace")
	alloc := tokens.Module("foo:bar/baz")
	typ := tokens.Type("bang:boom/fizzle:MajorResource")
	name := tokens.QName("a-swell-resource")
	urn := NewURN(ns, alloc, typ, name)
	assert.Equal(t, ns, urn.Namespace())
	assert.Equal(t, alloc, urn.Alloc())
	assert.Equal(t, typ, urn.Type())
	assert.Equal(t, name, urn.Name())
}
