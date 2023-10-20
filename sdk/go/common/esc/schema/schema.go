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

package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Builder interface {
	Schema() *Schema
}

func Never() *Schema {
	return &Schema{Never: true}
}

func Always() *Schema {
	return &Schema{Always: true}
}

func Ref(ref string) *Schema {
	return &Schema{Ref: ref}
}

func AnyOf(anyOf ...Builder) *Schema {
	s := &Schema{}
	return buildAnyOf(s, anyOf)
}

func OneOf(oneOf ...Builder) *Schema {
	s := &Schema{}
	return buildOneOf(s, oneOf)
}

type Schema struct {
	// Core vocabulary

	Never  bool `json:"-"`
	Always bool `json:"-"`

	Defs map[string]*Schema `json:"$defs,omitempty"`

	// Applicator vocabulary

	Ref                  string             `json:"$ref,omitempty"`
	AnyOf                []*Schema          `json:"anyOf,omitempty"`
	OneOf                []*Schema          `json:"oneOf,omitempty"`
	PrefixItems          []*Schema          `json:"prefixItems,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`

	// Validation vocabulary

	Type              string              `json:"type"`
	Const             any                 `json:"const,omitempty"`
	Enum              []any               `json:"enum,omitempty"`
	MultipleOf        json.Number         `json:"multipleOf,omitempty"`
	Maximum           json.Number         `json:"maximum,omitempty"`
	ExclusiveMaximum  json.Number         `json:"exclusiveMaximum,omitempty"`
	Minimum           json.Number         `json:"minimum,omitempty"`
	ExclusiveMinimum  json.Number         `json:"exclusiveMinimum,omitempty"`
	MaxLength         json.Number         `json:"maxLength,omitempty"`
	MinLength         json.Number         `json:"minLength,omitempty"`
	Pattern           string              `json:"pattern,omitempty"`
	MaxItems          json.Number         `json:"maxItems,omitempty"`
	MinItems          json.Number         `json:"minItems,omitempty"`
	UniqueItems       bool                `json:"uniqueItems,omitempty"`
	MaxProperties     json.Number         `json:"maxProperties,omitempty"`
	MinProperties     json.Number         `json:"minProperties,omitempty"`
	Required          []string            `json:"required,omitempty"`
	DependentRequired map[string][]string `json:"dependentRequired,omitempty"`

	// Metadata vocabulary

	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
	Deprecated  bool   `json:"deprecated,omitempty"`
	Examples    []any  `json:"examples,omitempty"`

	// Environments extensions
	Secret bool `json:"secret,omitempty"`

	ref              *Schema
	multipleOf       *big.Float
	maximum          *big.Float
	exclusiveMaximum *big.Float
	minimum          *big.Float
	exclusiveMinimum *big.Float
	maxLength        *uint
	minLength        *uint
	pattern          *regexp.Regexp
	maxItems         *uint
	minItems         *uint
	maxProperties    *uint
	minProperties    *uint

	compiled bool
}

func (s *Schema) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		if b {
			s.Always = true
			return nil
		}
		s.Never = true
		return nil
	}

	type rawSchema Schema
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode((*rawSchema)(s))
}

func (s *Schema) MarshalJSON() ([]byte, error) {
	switch {
	case s.Never:
		return []byte("false"), nil
	case s.Always:
		return []byte("true"), nil
	default:
		type rawSchema Schema
		return json.Marshal((*rawSchema)(s))
	}
}

func (s *Schema) Schema() *Schema {
	return s
}

func (s *Schema) arrayItem(index int) *Schema {
	if s.Type != "array" {
		return Never()
	}
	if index < len(s.PrefixItems) {
		return s.PrefixItems[index]
	}
	return s.Items
}

func (s *Schema) Item(index int) *Schema {
	var oneOf []*Schema
	for _, x := range s.AnyOf {
		oneOf = append(oneOf, x.Item(index))
	}
	for _, x := range s.OneOf {
		oneOf = append(oneOf, x.Item(index))
	}
	oneOf = append(oneOf, s.arrayItem(index))
	return union(oneOf)
}

func (s *Schema) objectProperty(name string) *Schema {
	if s.Type != "object" {
		return Never()
	}
	if p, ok := s.Properties[name]; ok {
		return p
	}
	return s.AdditionalProperties
}

func (s *Schema) Property(name string) *Schema {
	var oneOf []*Schema
	for _, x := range s.AnyOf {
		oneOf = append(oneOf, x.Property(name))
	}
	for _, x := range s.OneOf {
		oneOf = append(oneOf, x.Property(name))
	}
	oneOf = append(oneOf, s.objectProperty(name))
	return union(oneOf)
}

func (s *Schema) GetRef() *Schema                 { return s.ref }
func (s *Schema) GetMultipleOf() *big.Float       { return s.multipleOf }
func (s *Schema) GetMaximum() *big.Float          { return s.maximum }
func (s *Schema) GetExclusiveMaximum() *big.Float { return s.exclusiveMaximum }
func (s *Schema) GetMinimum() *big.Float          { return s.minimum }
func (s *Schema) GetExclusiveMinimum() *big.Float { return s.exclusiveMinimum }
func (s *Schema) GetMaxLength() *uint             { return s.maxLength }
func (s *Schema) GetMinLength() *uint             { return s.minLength }
func (s *Schema) GetPattern() *regexp.Regexp      { return s.pattern }
func (s *Schema) GetMaxItems() *uint              { return s.maxItems }
func (s *Schema) GetMinItems() *uint              { return s.minItems }
func (s *Schema) GetMaxProperties() *uint         { return s.maxProperties }
func (s *Schema) GetMinProperties() *uint         { return s.minProperties }

func (s *Schema) Compile() error {
	if s == nil || s.compiled {
		return nil
	}

	return s.compile(s)
}

func (s *Schema) compile(root *Schema) error {
	if s == nil || s.compiled {
		return nil
	}
	s.compiled = true

	var err error
	if s.Ref != "" {
		if s.ref, err = parseRef(root, s.Ref); err != nil {
			return err
		}
		if err = s.ref.compile(root); err != nil {
			return err
		}
	}

	for _, s := range s.AnyOf {
		if err := s.compile(root); err != nil {
			return err
		}
	}
	for _, s := range s.OneOf {
		if err := s.compile(root); err != nil {
			return err
		}
	}

	for _, s := range s.PrefixItems {
		if err := s.compile(root); err != nil {
			return err
		}
	}
	if err := s.Items.compile(root); err != nil {
		return err
	}
	if err := s.AdditionalProperties.compile(root); err != nil {
		return err
	}
	for _, v := range s.Properties {
		if err := v.compile(root); err != nil {
			return err
		}
	}

	if s.multipleOf, err = parseNumber(s.MultipleOf); err != nil {
		return err
	}
	if s.maximum, err = parseNumber(s.Maximum); err != nil {
		return err
	}
	if s.exclusiveMaximum, err = parseNumber(s.ExclusiveMaximum); err != nil {
		return err
	}
	if s.minimum, err = parseNumber(s.Minimum); err != nil {
		return err
	}
	if s.exclusiveMinimum, err = parseNumber(s.ExclusiveMinimum); err != nil {
		return err
	}
	if s.maxLength, err = parseUint(s.MaxLength); err != nil {
		return err
	}
	if s.minLength, err = parseUint(s.MinLength); err != nil {
		return err
	}
	if s.pattern, err = parseRegexp(s.Pattern); err != nil {
		return err
	}
	if s.maxItems, err = parseUint(s.MaxItems); err != nil {
		return err
	}
	if s.minItems, err = parseUint(s.MinItems); err != nil {
		return err
	}
	if s.maxProperties, err = parseUint(s.MaxProperties); err != nil {
		return err
	}
	if s.minProperties, err = parseUint(s.MinProperties); err != nil {
		return err
	}

	return nil
}

func parseRef(root *Schema, ref string) (*Schema, error) {
	refName, ok := strings.CutPrefix(ref, "#/$defs/")
	if !ok || strings.Contains(refName, "/") {
		return nil, errors.New("only fragment references of the form #/$defs/ref are supported")
	}

	refName, err := url.PathUnescape(refName)
	if err != nil {
		return nil, err
	}

	s, ok := root.Defs[refName]
	if !ok {
		return nil, fmt.Errorf("unknown subschema %v", ref)
	}
	return s, nil
}

func parseNumber(n json.Number) (*big.Float, error) {
	if n == "" {
		return nil, nil
	}
	f, _, err := big.ParseFloat(string(n), 10, 0, big.ToNearestEven)
	return f, err
}

func parseUint(n json.Number) (*uint, error) {
	if n == "" {
		return nil, nil
	}
	v64, err := strconv.ParseUint(string(n), 10, 0)
	if err != nil {
		return nil, err
	}
	v := uint(v64)
	return &v, nil
}

func parseRegexp(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	return regexp.Compile(pattern)
}

func buildDefs[T Builder](b T, defs map[string]Builder) T {
	s := b.Schema()
	s.Defs = make(map[string]*Schema, len(defs))
	for k, v := range defs {
		s.Defs[k] = v.Schema()
	}
	return b
}

func buildRef[T Builder](b T, ref string) T {
	b.Schema().Ref = ref
	return b
}

func buildAnyOf[T Builder](b T, anyOf []Builder) T {
	s := b.Schema()
	s.AnyOf = make([]*Schema, len(anyOf))
	for i, b := range anyOf {
		s.AnyOf[i] = b.Schema()
	}
	return b
}

func buildOneOf[T Builder](b T, oneOf []Builder) T {
	s := b.Schema()
	s.OneOf = make([]*Schema, len(oneOf))
	for i, b := range oneOf {
		s.OneOf[i] = b.Schema()
	}
	return b
}

func union(oneOf []*Schema) *Schema {
	// Filter out Never schemas.
	n := 0
	for _, s := range oneOf {
		if s != nil && !s.Never {
			oneOf[n] = s
			n++
		}
	}
	oneOf = oneOf[:n]

	switch len(oneOf) {
	case 0:
		// If there are no schemas left, return Never.
		return Never()
	case 1:
		// If there is one schema left, return is.
		return oneOf[0]
	default:
		// Otherwise, return a OneOf.
		return &Schema{OneOf: oneOf}
	}
}
