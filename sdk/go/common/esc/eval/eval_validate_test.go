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

package eval

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateNeverReportsDiagnostic ensures that validating a value against the `Never` schema fails *and* reports a
// diagnostic explaining why, rather than failing silently.
//
// See https://github.com/pulumi/pulumi/issues/24909.
func TestValidateNeverReportsDiagnostic(t *testing.T) {
	t.Parallel()

	t.Run("known value against Never", func(t *testing.T) {
		t.Parallel()

		x := newMissingExpr("", nil)
		v := &value{def: x, schema: schema.String().Schema(), repr: "hello"}

		var e validator
		ok := e.validateElement(v, schema.Never(), validationLoc{x: x})
		assert.False(t, ok)
		require.Len(t, e.diags, 1)
		assert.Equal(t, "value is never valid", e.diags[0].Summary)
	})

	t.Run("schema against Never", func(t *testing.T) {
		t.Parallel()

		x := newMissingExpr("", nil)

		var e validator
		ok := e.validateSchemaType(schema.String().Schema(), schema.Never(), validationLoc{x: x})
		assert.False(t, ok)
		require.Len(t, e.diags, 1)
		assert.Equal(t, "value is never valid", e.diags[0].Summary)
	})

	t.Run("Never schema against Never", func(t *testing.T) {
		t.Parallel()

		x := newMissingExpr("", nil)

		var e validator
		ok := e.validateSchemaType(schema.Never(), schema.Never(), validationLoc{x: x})
		assert.True(t, ok)
		assert.Empty(t, e.diags, "Never is trivially a subtype of itself, so this should validate without error")
	})
}

// TestValidateNeverIsSubtypeOfEverything ensures that the `Never` schema, which admits no values, validates against any
// schema: it is the bottom type. A tuple's schema has `Items: Never`, so a value that is shorter than the expected
// tuple compares its missing slots against `Never` and must not be reported as an error.
func TestValidateNeverIsSubtypeOfEverything(t *testing.T) {
	t.Parallel()

	t.Run("Never schema against a concrete schema", func(t *testing.T) {
		t.Parallel()

		x := newMissingExpr("", nil)

		var e validator
		ok := e.validateSchemaType(schema.Never(), schema.String().Schema(), validationLoc{x: x})
		assert.True(t, ok)
		assert.Empty(t, e.diags)
	})

	t.Run("shorter tuple against a longer tuple", func(t *testing.T) {
		t.Parallel()

		x := newMissingExpr("", nil)
		actual := schema.Tuple(schema.String(), schema.Number()).Schema()
		expected := schema.Tuple(schema.String(), schema.Number(), schema.Boolean()).Schema()

		var e validator
		ok := e.validateSchemaType(actual, expected, validationLoc{x: x})
		assert.True(t, ok)
		assert.Empty(t, e.diags)
	})
}
