// Copyright 2016-2022, Pulumi Corporation.
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

// Supporting test-only code for generating random JSON values for
// property-based testing.
package client

import (
	"encoding/json"
	"pgregory.net/rapid"
)

// Configures random JSON generation.
type rapidJsonOpts struct {
	// If true, disallows objects with null values {"foo": null}.
	noNullValuesInObjects bool
	// Generator override for object keys and string values.
	stringGen *rapid.Generator
	// Generator override for int values.
	intGen *rapid.Generator
	// Generator override for double values.
	float64Gen *rapid.Generator
	// Generator override for boolean values.
	boolGen *rapid.Generator
}

func (opts rapidJsonOpts) StringGen() *rapid.Generator {
	strG := opts.stringGen
	if strG == nil {
		strG = rapid.String()
	}
	return strG
}

func (opts rapidJsonOpts) IntGen() *rapid.Generator {
	strG := opts.intGen
	if strG == nil {
		strG = rapid.Int()
	}
	return strG
}

func (opts rapidJsonOpts) F64Gen() *rapid.Generator {
	strG := opts.float64Gen
	if strG == nil {
		strG = rapid.Float64()
	}
	return strG
}

func (opts rapidJsonOpts) BoolGen() *rapid.Generator {
	strG := opts.boolGen
	if strG == nil {
		strG = rapid.Bool()
	}
	return strG
}

type rapidJsonGen struct {
	opts rapidJsonOpts
}

func (g *rapidJsonGen) genJsonValue(maxHeight int) *rapid.Generator {
	return rapid.OneOf(
		rapid.Just(json.RawMessage(`null`)),
		g.genJsonNonNullValue(maxHeight))
}

func (g *rapidJsonGen) genJsonNonNullValue(maxHeight int) *rapid.Generator {
	if maxHeight <= 0 {
		panic("maxHeight <= 0")
	}

	options := []*rapid.Generator{
		g.opts.BoolGen().Map(func(x bool) json.RawMessage { return g.marshal(x) }),
		g.opts.StringGen().Map(func(x string) json.RawMessage { return g.marshal(x) }),
		g.opts.IntGen().Map(func(x int) json.RawMessage { return g.marshal(x) }),
		g.opts.F64Gen().Map(func(x float64) json.RawMessage { return g.marshal(x) }),
	}

	if maxHeight > 1 {
		options = append(options,
			g.genJsonObject(maxHeight),
			g.genJsonArray(maxHeight))
	}

	return rapid.OneOf(options...)
}

func (g *rapidJsonGen) genJsonObject(maxHeight int) *rapid.Generator {
	if maxHeight <= 1 {
		panic("maxHeight <= 1")
	}

	keyGen := g.opts.StringGen()

	valGen := g.genJsonValue(maxHeight - 1)
	if g.opts.noNullValuesInObjects {
		valGen = g.genJsonNonNullValue(maxHeight - 1)
	}

	return rapid.MapOf(keyGen, valGen).
		Map(func(x map[string]json.RawMessage) json.RawMessage { return g.marshal(x) })
}

func (g *rapidJsonGen) genJsonArray(maxHeight int) *rapid.Generator {
	if maxHeight <= 1 {
		panic("maxHeight <= 1")
	}
	elemGen := g.genJsonValue(maxHeight - 1)
	return rapid.SliceOf(elemGen).
		Map(func(x []interface{}) json.RawMessage { return g.marshal(x) })
}

func (rapidJsonGen) marshal(x interface{}) json.RawMessage {
	bytes, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return bytes
}
