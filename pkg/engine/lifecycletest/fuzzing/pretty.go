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

func ColorFor(s string) *color.Color {
	// Use FNV-1a hash function
	hash := fnv.New32a()
	hash.Write([]byte(s))
	sum := hash.Sum32()

	// Extract RGB values from the hash
	r := color.Attribute((sum >> 16) & 0xFF) // Get bits 16-23
	g := color.Attribute((sum >> 8) & 0xFF)  // Get bits 8-15
	b := color.Attribute(sum & 0xFF)         // Get bits 0-7

	c := color.New(38, 2, r, g, b)
	c.EnableColor()
	return c
}

func Colored[T ~string](t T) string {
	return ColorFor(string(t)).Sprint(t)
}

type PrettySpec interface {
	Pretty(indent string) string
}
