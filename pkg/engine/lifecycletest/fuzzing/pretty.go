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

package fuzzing

import (
	"hash/fnv"

	"github.com/fatih/color"
)

// PrettySpecs can be pretty-printed as human-readable strings for use in debugging output and error messages.
type PrettySpec interface {
	// Returns a pretty human-readable string representation of this spec.
	Pretty(indent string) string
}

// ColorFor accepts a string and hashes it to produce an RGB color. This is useful for making e.g. different URNs easy
// to identify by giving them unique colors when pretty-printing them.
func ColorFor(s string) *color.Color {
	hash := fnv.New32a()
	hash.Write([]byte(s))
	sum := hash.Sum32()

	r := color.Attribute((sum >> 16) & 0xFF)
	g := color.Attribute((sum >> 8) & 0xFF)
	b := color.Attribute(sum & 0xFF)

	c := color.New(38, 2, r, g, b)
	c.EnableColor()
	return c
}

// Colored generates a color from the given string and colors the string with it.
func Colored[T ~string](t T) string {
	return ColorFor(string(t)).Sprint(t)
}
