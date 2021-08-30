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

package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func urnGen() *rapid.Generator {
	return rapid.StringMatching(`urn:pulumi:a::b::c:d:e::[abcd][123]`).
		Map(func(x string) resource.URN { return resource.URN(x) })
}

// Generates PropertyValue values.
func propertyValueGen() *rapid.Generator {
	return rapid.Just(resource.NewNullProperty())
}

// Generates Output values.
func outputGen() *rapid.Generator {
	propertyValueG := propertyValueGen()
	urnsG := rapid.SliceOf(urnGen())
	return rapid.Custom(func(t *rapid.T) resource.Output {
		element := propertyValueG.Draw(t, "element").(resource.PropertyValue)
		known := rapid.Bool().Draw(t, "known").(bool)
		secret := rapid.Bool().Draw(t, "secret").(bool)
		deps := urnsG.Draw(t, "dependencies").([]resource.URN)
		return resource.Output{
			Element:      element,
			Known:        known,
			Secret:       secret,
			Dependencies: deps,
		}
	})
}

func TestOutputValueTurnaround(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		out := outputGen().Draw(t, "output").(resource.Output)
		v := resource.NewOutputProperty(out)

		opts := MarshalOptions{KeepOutputValues: true}
		pb, err := MarshalPropertyValue(v, opts)
		assert.NoError(t, err)
		if err != nil {
			t.FailNow()
		}
		v2, err := UnmarshalPropertyValue(pb, opts)
		assert.NoError(t, err)
		if err != nil {
			t.FailNow()
		}
		assert.NotNil(t, v2)
		assert.Equal(t, v, *v2)
	})
}
