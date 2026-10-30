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

package policy

import (
	"encoding/json"
	"testing"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
)

const (
	stackCtxJSON   = `{"__type": "PermissionExpressionStack"}`
	accountCtxJSON = `{"__type": "PermissionExpressionInsightsAccount"}`
)

func tagEqualsJSON(ctx, key, value string) string {
	return `{"__type": "PermissionExpressionEqual",
		"left": {"__type": "PermissionExpressionTag", "context": ` + ctx + `, "key": "` + key + `"},
		"right": {"__type": "PermissionLiteralExpressionString", "value": "` + value + `"}}`
}

func hasTagJSON(ctx, key string) string {
	return `{"__type": "PermissionExpressionHasTag", "context": ` + ctx + `, "key": "` + key + `"}`
}

func notJSON(node string) string {
	return `{"__type": "PermissionExpressionNot", "node": ` + node + `}`
}

func binaryJSON(op, left, right string) string {
	return `{"__type": "PermissionExpression` + op + `", "left": ` + left + `, "right": ` + right + `}`
}

func marshalRule(t *testing.T, rule cloudapitype.PermissionBooleanExpression) string {
	t.Helper()
	b, err := json.Marshal(rule)
	require.NoError(t, err)
	return string(b)
}

func TestParseMatchRule_TagConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		kind  matchRuleKind
		value string
		want  string
	}{
		{
			name:  "equals",
			kind:  stackMatchRule,
			value: "env=prod",
			want:  tagEqualsJSON(stackCtxJSON, "env", "prod"),
		},
		{
			name:  "not equals",
			kind:  stackMatchRule,
			value: "env!=prod",
			want:  notJSON(tagEqualsJSON(stackCtxJSON, "env", "prod")),
		},
		{
			name:  "exists",
			kind:  stackMatchRule,
			value: "owner",
			want:  hasTagJSON(stackCtxJSON, "owner"),
		},
		{
			name:  "not exists",
			kind:  stackMatchRule,
			value: "!temp",
			want:  notJSON(hasTagJSON(stackCtxJSON, "temp")),
		},
		{
			name:  "conditions are ANDed left to right",
			kind:  stackMatchRule,
			value: "env=prod, region = us-west-2 ,owner",
			want: binaryJSON("And",
				binaryJSON("And",
					tagEqualsJSON(stackCtxJSON, "env", "prod"),
					tagEqualsJSON(stackCtxJSON, "region", "us-west-2")),
				hasTagJSON(stackCtxJSON, "owner")),
		},
		{
			name:  "alternatives are ORed left to right",
			kind:  stackMatchRule,
			value: "env=prod,owner OR team=platform OR !temp",
			want: binaryJSON("Or",
				binaryJSON("Or",
					binaryJSON("And", tagEqualsJSON(stackCtxJSON, "env", "prod"), hasTagJSON(stackCtxJSON, "owner")),
					tagEqualsJSON(stackCtxJSON, "team", "platform")),
				notJSON(hasTagJSON(stackCtxJSON, "temp"))),
		},
		{
			name:  "account rules read account tags",
			kind:  accountMatchRule,
			value: "env=prod,team!=sandbox",
			want: binaryJSON("And",
				tagEqualsJSON(accountCtxJSON, "env", "prod"),
				notJSON(tagEqualsJSON(accountCtxJSON, "team", "sandbox"))),
		},
		{
			name:  "value may start or end with an exclamation mark",
			kind:  stackMatchRule,
			value: "env=!prod!,team!=web!",
			want: binaryJSON("And",
				tagEqualsJSON(stackCtxJSON, "env", "!prod!"),
				notJSON(tagEqualsJSON(stackCtxJSON, "team", "web!"))),
		},
		{
			name:  "OR without surrounding spaces is part of a value",
			kind:  stackMatchRule,
			value: "env=prodORdev",
			want:  tagEqualsJSON(stackCtxJSON, "env", "prodORdev"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rule, err := parseMatchRule(tt.kind, tt.value)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, marshalRule(t, rule))
		})
	}
}

func TestParseMatchRuleFlag_NotPassed(t *testing.T) {
	t.Parallel()

	rule, err := parseMatchRuleFlag(stackMatchRule, "env=prod", map[string]bool{"account-rule": true})
	require.NoError(t, err)
	assert.Nil(t, rule)
}

func TestParseMatchRule_Empty(t *testing.T) {
	t.Parallel()

	rule, err := parseMatchRule(stackMatchRule, "")
	require.NoError(t, err)
	assert.Nil(t, rule)
}

func TestParseMatchRule_JSON(t *testing.T) {
	t.Parallel()

	expr := binaryJSON("Or", hasTagJSON(stackCtxJSON, "a"), tagEqualsJSON(stackCtxJSON, "b", "c"))

	rule, err := parseMatchRule(stackMatchRule, "  "+expr)
	require.NoError(t, err)
	assert.JSONEq(t, expr, marshalRule(t, rule))
}

func TestParseMatchRule_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		kind  matchRuleKind
		value string
		want  string
	}{
		{"empty condition", stackMatchRule, "env=prod,,owner", "invalid --stack-rule: empty tag condition"},
		{"empty alternative", stackMatchRule, "env=prod OR  OR owner", "invalid --stack-rule: empty tag condition"},
		{"leading OR", stackMatchRule, "OR env=prod", `invalid --stack-rule: " OR " must separate two alternatives`},
		{"trailing OR", stackMatchRule, "env=prod OR ", `invalid --stack-rule: " OR " must separate two alternatives`},
		{"doubled OR", stackMatchRule, "env=prod OR OR owner", `invalid --stack-rule: " OR " must separate two alternatives`},
		{"tripled OR", stackMatchRule, "env OR OR OR owner", `invalid --stack-rule: " OR " must separate two alternatives`},
		{
			"lowercase or", stackMatchRule, "env=prod or team=web",
			`invalid --stack-rule: "env=prod or team=web": separate alternatives with " OR " (uppercase, between spaces); ` +
				`use a JSON expression for a key or value containing the word "or"`,
		},
		{
			"OR next to a tab", stackMatchRule, "env=prod\tOR team=web",
			`invalid --stack-rule: "env=prod\tOR team=web": separate alternatives with " OR " (uppercase, between spaces); ` +
				`use a JSON expression for a key or value containing the word "or"`,
		},
		{
			"whitespace only", stackMatchRule, " \t",
			"invalid --stack-rule: rule contains only whitespace; pass '' for no rule",
		},
		{
			"key ending in exclamation mark", stackMatchRule, "env!",
			`invalid --stack-rule: tag condition "env!": key "env!" needs a JSON expression; see --help`,
		},
		{
			"negated key ending in exclamation mark", stackMatchRule, "env!!=prod",
			`invalid --stack-rule: tag condition "env!!=prod": key "env!" needs a JSON expression; see --help`,
		},
		{
			"key starting with a brace", stackMatchRule, "env OR {team",
			`invalid --stack-rule: tag condition "{team": key "{team" needs a JSON expression; see --help`,
		},
		{
			"value containing an equals sign", stackMatchRule, "url=a=b",
			`invalid --stack-rule: tag condition "url=a=b": value "a=b" needs a JSON expression; see --help`,
		},
		{"missing key", stackMatchRule, "=prod", `invalid --stack-rule: tag condition "=prod" is missing a key`},
		{"missing negated key", accountMatchRule, "!", `invalid --account-rule: tag condition "!" is missing a key`},
		{
			"missing value", stackMatchRule, "env=",
			`invalid --stack-rule: tag condition "env=" is missing a value; write "env" to require only that the tag exists`,
		},
		{
			"negated equality", stackMatchRule, "!env=prod",
			`invalid --stack-rule: tag condition "!env=prod": write key!=value to exclude a value`,
		},
		{
			"unknown type", stackMatchRule, `{"__type": "Nope"}`,
			"invalid --stack-rule: invalid JSON expression: type 'Nope' not recognized",
		},
		{
			"malformed JSON", stackMatchRule, `{"__type": `,
			"invalid --stack-rule: invalid JSON expression: unexpected end of JSON input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseMatchRule(tt.kind, tt.value)
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
			assert.IsType(t, cmdCmd.ConfigurationError{}, err)
		})
	}
}

func TestFormatMatchRule_RoundTripsTagConditions(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"env=prod",
		"env!=prod",
		"owner",
		"!temp",
		"env=prod,region=us-west-2,!temp",
		"env=prod,owner OR team=platform OR !temp",
		"env=!prod!,team!=web!",
		"env=prodORdev",
	} {
		rule, err := parseMatchRule(stackMatchRule, value)
		require.NoError(t, err)
		assert.Equal(t, value, formatMatchRule(stackMatchRule, rule))
	}
}

func TestFormatMatchRule_FallsBackToJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind matchRuleKind
		expr string
	}{
		{
			name: "unsupported expression",
			kind: stackMatchRule,
			expr: `{"__type": "PermissionExpressionIn",
				"target": {"__type": "PermissionExpressionTag", "context": ` + stackCtxJSON + `, "key": "env"},
				"values": {"__type": "PermissionSetExpressionString", "values": ["prod"]}}`,
		},
		{
			name: "value the syntax cannot carry",
			kind: stackMatchRule,
			expr: tagEqualsJSON(stackCtxJSON, "env", "a,b"),
		},
		{
			name: "value ending in OR",
			kind: stackMatchRule,
			expr: tagEqualsJSON(stackCtxJSON, "env", "prod OR"),
		},
		{
			name: "value containing a lowercase or",
			kind: stackMatchRule,
			expr: tagEqualsJSON(stackCtxJSON, "env", "rock or roll"),
		},
		{
			name: "key ending in exclamation mark",
			kind: stackMatchRule,
			expr: hasTagJSON(stackCtxJSON, "env!"),
		},
		{
			name: "tag named OR",
			kind: stackMatchRule,
			expr: hasTagJSON(stackCtxJSON, "OR"),
		},
		{
			name: "context of the other entity",
			kind: stackMatchRule,
			expr: hasTagJSON(accountCtxJSON, "env"),
		},
		{
			name: "double negation",
			kind: stackMatchRule,
			expr: notJSON(notJSON(hasTagJSON(stackCtxJSON, "env"))),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var rule cloudapitype.PermissionBooleanExpression
			require.NoError(t, cloudapitype.UnmarshalJSONPermissionBooleanExpression([]byte(tt.expr), &rule))
			assert.JSONEq(t, tt.expr, formatMatchRule(tt.kind, rule))
		})
	}
}
