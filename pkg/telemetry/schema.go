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

package telemetry

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type Kind string

const (
	KindBool    Kind = "bool"
	KindInt     Kind = "int"
	KindEnum    Kind = "enum"
	KindNameSet Kind = "name-set"
)

const (
	maxNames      = 64
	maxNameLength = 128
)

type Fact struct {
	Kind      Kind     `json:"kind"`
	Values    []string `json:"values,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	MaxItems  int      `json:"maxItems,omitempty"`
	MaxLength int      `json:"maxLength,omitempty"`

	re *regexp.Regexp
}

var (
	osValues   = []string{"linux", "darwin", "windows"}
	archValues = []string{"amd64", "arm64"}
)

//go:embed schema.json
var schemaJSON []byte

var schema = loadSchema(schemaJSON)

func loadSchema(data []byte) map[string]Fact {
	var s struct {
		Facts map[string]Fact `json:"facts"`
	}
	contract.AssertNoErrorf(json.Unmarshal(data, &s), "parsing the embedded telemetry schema")
	for name, f := range s.Facts {
		if f.Pattern != "" {
			f.re = regexp.MustCompile(f.Pattern)
			s.Facts[name] = f
		}
	}
	return s.Facts
}

func schemaFor(commands []string) map[string]Fact {
	ciSystemValues := slices.Sorted(maps.Values(ciSystems))
	return map[string]Fact{
		"arch":           enumSchema(archValues, "other"),
		"automation-api": {Kind: KindBool},
		"backend":        enumSchema(append([]string{"pulumi"}, diySchemes...), "other"),
		"ci":             {Kind: KindBool},
		"ci-system":      enumSchema(ciSystemValues, "none", "other"),
		"command":        enumSchema(commands, "other"),
		"duration-ms":    {Kind: KindInt},
		"env-vars":       nameSetSchema(`^PULUMI_[A-Z0-9_]+$`),
		"exit-code":      {Kind: KindInt},
		"flags":          nameSetSchema(`^[a-z][a-z0-9-]*$`),
		"interactive":    {Kind: KindBool},
		"os":             enumSchema(osValues, "other"),
		"using-ai":       enumSchema(agents, "no", "other"),
	}
}

func enumSchema(values []string, extra ...string) Fact {
	return Fact{Kind: KindEnum, Values: append(slices.Clone(values), extra...)}
}

func nameSetSchema(pattern string) Fact {
	return Fact{
		Kind:      KindNameSet,
		Pattern:   pattern,
		MaxItems:  maxNames,
		MaxLength: maxNameLength,
		re:        regexp.MustCompile(pattern),
	}
}

func SchemaJSON(commands []string) ([]byte, error) {
	data, err := json.MarshalIndent(struct {
		Version int             `json:"version"`
		Facts   map[string]Fact `json:"facts"`
	}{Version: 1, Facts: schemaFor(commands)}, "", "  ")
	return append(data, '\n'), err
}

func ValidateFact(name string, value any) error {
	f, ok := schema[name]
	if !ok {
		return fmt.Errorf("unknown fact %q", name)
	}
	if err := f.validate(value); err != nil {
		return fmt.Errorf("fact %q: %w", name, err)
	}
	return nil
}

func (f Fact) validate(value any) error {
	switch f.Kind {
	case KindBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected a bool, got %T", value)
		}
	case KindInt:
		v, ok := value.(float64)
		if !ok || v != math.Trunc(v) || math.Abs(v) > 1<<53 {
			return fmt.Errorf("expected an integer, got %v", value)
		}
	case KindEnum:
		v, ok := value.(string)
		if !ok || !slices.Contains(f.Values, v) {
			return fmt.Errorf("expected one of the allowed values, got %v", value)
		}
	case KindNameSet:
		names, ok := value.([]string)
		if !ok {
			return fmt.Errorf("expected a list of names, got %T", value)
		}
		if len(names) > f.MaxItems {
			return fmt.Errorf("expected at most %d names, got %d", f.MaxItems, len(names))
		}
		for _, name := range names {
			if len(name) > f.MaxLength || !f.re.MatchString(name) {
				return fmt.Errorf("name %q does not match %s", name, f.Pattern)
			}
		}
	default:
		return fmt.Errorf("unknown kind %q", f.Kind)
	}
	return nil
}
