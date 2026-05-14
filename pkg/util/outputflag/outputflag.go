// Copyright 2026, Pulumi Corporation.
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

package outputflag

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/pflag"
)

// Var registers the OutputFlag on flags as --output with no shorthand.
func Var[R any](flags *pflag.FlagSet, output *OutputFlag[R]) {
	flags.Var(output, "output", usage(*output))
}

// VarP registers the OutputFlag on flags as --output with the conventional
// -o shorthand.
func VarP[R any](flags *pflag.FlagSet, output *OutputFlag[R]) {
	flags.VarP(output, "output", "o", usage(*output))
}

func usage[R any](output OutputFlag[R]) string {
	return "Output format. Supported values are: " + supportedValues(output, "and")
}

var _ pflag.Value = (*OutputFlag[struct{}])(nil)

type OutputFlag[R any] struct {
	RenderForTerminal R // Default behavior
	RenderJSON        R
	RenderMarkdown    R
	RenderCSV         R
	RenderYAML        R

	// set value
	value  string
	render R
}

func (f OutputFlag[R]) Get() R {
	if reflect.ValueOf(f.render).IsZero() {
		return f.RenderForTerminal
	}
	return f.render
}

func (f OutputFlag[R]) String() string {
	if f.value != "" {
		return f.value
	}
	return "default"
}

func (f *OutputFlag[R]) Set(s string) error {
	switch s {
	case "", "default":
		f.render = f.RenderForTerminal
	case "json":
		if reflect.ValueOf(f.RenderJSON).IsZero() {
			return f.unsupported(s)
		}
		f.render = f.RenderJSON
	case "markdown", "md":
		if reflect.ValueOf(f.RenderMarkdown).IsZero() {
			return f.unsupported(s)
		}
		f.render = f.RenderMarkdown
		s = "markdown" // normalize s
	case "yaml":
		if reflect.ValueOf(f.RenderYAML).IsZero() {
			return f.unsupported(s)
		}
		f.render = f.RenderYAML
	case "csv":
		if reflect.ValueOf(f.RenderCSV).IsZero() {
			return f.unsupported(s)
		}
		f.render = f.RenderCSV
	default:
		return f.unsupported(s)
	}
	f.value = s
	return nil
}

func (f OutputFlag[R]) Type() string { return "string" }

type unsuportedArgError[R any] struct {
	f     OutputFlag[R]
	given string
}

func (f OutputFlag[R]) unsupported(s string) error {
	return unsuportedArgError[R]{f, s}
}

func (err unsuportedArgError[R]) Error() string {
	return fmt.Sprintf("output %q not supported, valid values are: %s",
		err.given, supportedValues(err.f, "or"))
}

func supportedValues[R any](f OutputFlag[R], lastJoin string) string {
	supported := []string{"default"}
	if !reflect.ValueOf(f.RenderJSON).IsZero() {
		supported = append(supported, "json")
	}
	if !reflect.ValueOf(f.RenderMarkdown).IsZero() {
		supported = append(supported, "markdown")
	}
	if !reflect.ValueOf(f.RenderYAML).IsZero() {
		supported = append(supported, "yaml")
	}
	if !reflect.ValueOf(f.RenderCSV).IsZero() {
		supported = append(supported, "csv")
	}

	if len(supported) == 1 {
		return supported[0]
	}

	return strings.Join(supported[:len(supported)-1], ", ") + " " +
		lastJoin + " " + supported[len(supported)-1]
}
