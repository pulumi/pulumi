// Copyright 2023, Pulumi Corporation.  All rights reserved.

package schema

import (
	"bytes"
	"encoding/json"
	"math/big"
	"regexp"
	"strconv"
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

type Schema struct {
	// Core vocabulary

	Never  bool `json:"-"`
	Always bool `json:"-"`

	// Applicator volcabulary

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

func (s Schema) MarshalJSON() ([]byte, error) {
	switch {
	case s.Never:
		return []byte("false"), nil
	case s.Always:
		return []byte("true"), nil
	default:
		type rawSchema Schema
		return json.Marshal((rawSchema)(s))
	}
}

func (s *Schema) Schema() *Schema {
	return s
}

func (s *Schema) Item(index int) *Schema {
	if s.Type != "array" {
		return Never()
	}
	if index < len(s.PrefixItems) {
		return s.PrefixItems[index]
	}
	return s.Items
}

func (s *Schema) Property(name string) *Schema {
	if s.Type != "object" {
		return Never()
	}
	if p, ok := s.Properties[name]; ok {
		return p
	}
	return s.AdditionalProperties
}

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
	s.compiled = true

	for _, s := range s.PrefixItems {
		if err := s.Compile(); err != nil {
			return err
		}
	}
	if err := s.Items.Compile(); err != nil {
		return err
	}
	if err := s.AdditionalProperties.Compile(); err != nil {
		return err
	}
	for _, v := range s.Properties {
		if err := v.Compile(); err != nil {
			return err
		}
	}

	var err error
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
