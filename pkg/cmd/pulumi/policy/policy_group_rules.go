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
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
)

// matchRuleKind is the entity a match rule's tag conditions read: stacks or
// Insights accounts.
type matchRuleKind struct {
	flag    string
	context func() cloudapitype.PermissionContextExpression
	isOwn   func(cloudapitype.PermissionContextExpression) bool
}

var (
	stackMatchRule = matchRuleKind{
		flag: "--stack-rule",
		context: func() cloudapitype.PermissionContextExpression {
			return cloudapitype.PermissionExpressionStackBuilder{}.Build()
		},
		isOwn: func(c cloudapitype.PermissionContextExpression) bool {
			_, ok := c.(cloudapitype.PermissionExpressionStack)
			return ok
		},
	}
	accountMatchRule = matchRuleKind{
		flag: "--account-rule",
		context: func() cloudapitype.PermissionContextExpression {
			return cloudapitype.PermissionExpressionInsightsAccountBuilder{}.Build()
		},
		isOwn: func(c cloudapitype.PermissionContextExpression) bool {
			_, ok := c.(cloudapitype.PermissionExpressionInsightsAccount)
			return ok
		},
	}
)

// matchRuleHelp documents the --stack-rule and --account-rule syntax in the
// long help of the commands that accept them.
const matchRuleHelp = "Match rules (preview; requires the tag-based policy groups feature):\n" +
	"\n" +
	"--stack-rule and --account-rule set a rule over stack tags or Insights\n" +
	"account tags. Stacks or accounts whose tags satisfy the rule are members\n" +
	"of the group alongside those added individually. --account-rule is only\n" +
	"allowed on audit groups. Setting a rule replaces the group's current rule,\n" +
	"and setting it to '' removes it.\n" +
	"\n" +
	"A non-empty value is one of:\n" +
	"  - Tag conditions separated by commas, all of which must hold:\n" +
	"      key=value    the tag is set to value\n" +
	"      key!=value   the tag is not set to value (also true when the tag is absent)\n" +
	"      key          the tag exists\n" +
	"      !key         the tag does not exist\n" +
	"    Separate alternatives with \" OR \" to match entities that satisfy any\n" +
	"    of them, as in 'env=prod,region=us-west-2 OR team'. This is the syntax\n" +
	"    `pulumi policy group get` prints.\n" +
	"  - A JSON PermissionBooleanExpression, as returned in the stackMatchRule\n" +
	"    and accountMatchRule fields of `pulumi policy group get --output json`.\n" +
	"\n" +
	"Use JSON when a tag key or value contains ',', '=' or \" OR \", or a key\n" +
	"starts or ends with '!'."

// parseMatchRuleFlag parses the value of kind's flag, or returns nil when the
// flag was not passed or its value is empty.
func parseMatchRuleFlag(
	kind matchRuleKind, value string, changed map[string]bool,
) (cloudapitype.PermissionBooleanExpression, error) {
	if !changed[strings.TrimPrefix(kind.flag, "--")] {
		return nil, nil
	}
	return parseMatchRule(kind, value)
}

// parseMatchRule parses a rule flag value, returning nil for an empty value.
func parseMatchRule(kind matchRuleKind, value string) (cloudapitype.PermissionBooleanExpression, error) {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return nil, nil
	case strings.HasPrefix(trimmed, "{"):
		return parseMatchRuleJSON(kind, []byte(trimmed))
	default:
		return parseAlternatives(kind, trimmed)
	}
}

func parseMatchRuleJSON(kind matchRuleKind, data []byte) (cloudapitype.PermissionBooleanExpression, error) {
	var expr cloudapitype.PermissionBooleanExpression
	if err := cloudapitype.UnmarshalJSONPermissionBooleanExpression(data, &expr); err != nil {
		return nil, matchRuleError(kind, "invalid JSON expression: "+err.Error())
	}
	if expr == nil {
		return nil, matchRuleError(kind, "JSON expression must be an object with a \"__type\"")
	}
	return expr, nil
}

func parseAlternatives(kind matchRuleKind, value string) (cloudapitype.PermissionBooleanExpression, error) {
	var rule cloudapitype.PermissionBooleanExpression
	for alt := range strings.SplitSeq(value, " OR ") {
		alt = strings.TrimSpace(alt)
		if alt == "OR" || strings.HasPrefix(alt, "OR ") || strings.HasSuffix(alt, " OR") {
			return nil, matchRuleError(kind, `" OR " must separate two alternatives`)
		}
		expr, err := parseTagConditions(kind, alt)
		if err != nil {
			return nil, err
		}
		if rule == nil {
			rule = expr
			continue
		}
		rule = cloudapitype.PermissionExpressionOrBuilder{
			PermissionBooleanExpressionBinaryBuilder: cloudapitype.PermissionBooleanExpressionBinaryBuilder{
				Left:  rule,
				Right: expr,
			},
		}.Build()
	}
	return rule, nil
}

func parseTagConditions(kind matchRuleKind, value string) (cloudapitype.PermissionBooleanExpression, error) {
	var rule cloudapitype.PermissionBooleanExpression
	for raw := range strings.SplitSeq(value, ",") {
		cond, err := parseTagCondition(kind, strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		if rule == nil {
			rule = cond
			continue
		}
		rule = cloudapitype.PermissionExpressionAndBuilder{
			PermissionBooleanExpressionBinaryBuilder: cloudapitype.PermissionBooleanExpressionBinaryBuilder{
				Left:  rule,
				Right: cond,
			},
		}.Build()
	}
	return rule, nil
}

func parseTagCondition(kind matchRuleKind, cond string) (cloudapitype.PermissionBooleanExpression, error) {
	if cond == "" {
		return nil, matchRuleError(kind, "empty tag condition")
	}

	eq := strings.Index(cond, "=")
	if eq < 0 {
		negate := strings.HasPrefix(cond, "!")
		key := strings.TrimSpace(strings.TrimPrefix(cond, "!"))
		if key == "" {
			return nil, matchRuleError(kind, fmt.Sprintf("tag condition %q is missing a key", cond))
		}
		hasTag := cloudapitype.PermissionExpressionHasTagBuilder{Context: kind.context(), Key: key}.Build()
		if negate {
			return notExpression(hasTag), nil
		}
		return hasTag, nil
	}

	negate := eq > 0 && cond[eq-1] == '!'
	keyEnd := eq
	if negate {
		keyEnd--
	}
	key := strings.TrimSpace(cond[:keyEnd])
	value := strings.TrimSpace(cond[eq+1:])
	if strings.HasPrefix(key, "!") {
		return nil, matchRuleError(kind,
			fmt.Sprintf("tag condition %q: write key!=value to exclude a value", cond))
	}
	if key == "" {
		return nil, matchRuleError(kind, fmt.Sprintf("tag condition %q is missing a key", cond))
	}
	if value == "" {
		return nil, matchRuleError(kind,
			fmt.Sprintf("tag condition %q is missing a value; write %q to require only that the tag exists", cond, key))
	}

	equal := cloudapitype.PermissionExpressionEqualBuilder{
		Left:  cloudapitype.PermissionExpressionTagBuilder{Context: kind.context(), Key: key}.Build(),
		Right: cloudapitype.PermissionLiteralExpressionStringBuilder{Value: value}.Build(),
	}.Build()
	if negate {
		return notExpression(equal), nil
	}
	return equal, nil
}

func notExpression(node cloudapitype.PermissionBooleanExpression) cloudapitype.PermissionBooleanExpression {
	return cloudapitype.PermissionExpressionNotBuilder{
		PermissionBooleanExpressionUnaryBuilder: cloudapitype.PermissionBooleanExpressionUnaryBuilder{Node: node},
	}.Build()
}

func matchRuleError(kind matchRuleKind, msg string) error {
	return cmdCmd.ConfigurationError{Message: fmt.Sprintf("invalid %s: %s", kind.flag, msg)}
}

// formatMatchRule renders a rule in the tag-condition syntax the rule flags
// accept, or as compact JSON when the rule has a shape that syntax cannot
// express.
func formatMatchRule(kind matchRuleKind, rule cloudapitype.PermissionBooleanExpression) string {
	var alternatives []string
	for _, branch := range flattenBinary[cloudapitype.PermissionExpressionOr](rule) {
		var conds []string
		for _, leaf := range flattenBinary[cloudapitype.PermissionExpressionAnd](branch) {
			cond, ok := formatTagCondition(kind, leaf)
			if !ok {
				return formatMatchRuleJSON(rule)
			}
			conds = append(conds, cond)
		}
		alternatives = append(alternatives, strings.Join(conds, ","))
	}
	return strings.Join(alternatives, " OR ")
}

// flattenBinary returns the operands of a chain of T nodes (AND or OR), left
// to right, regardless of how the chain is nested.
func flattenBinary[T cloudapitype.PermissionBooleanExpressionBinary](
	expr cloudapitype.PermissionBooleanExpression,
) []cloudapitype.PermissionBooleanExpression {
	node, ok := expr.(T)
	if !ok {
		return []cloudapitype.PermissionBooleanExpression{expr}
	}
	return append(flattenBinary[T](node.Left()), flattenBinary[T](node.Right())...)
}

func formatTagCondition(kind matchRuleKind, expr cloudapitype.PermissionBooleanExpression) (string, bool) {
	prefix, op := "", "="
	if not, ok := expr.(cloudapitype.PermissionExpressionNot); ok {
		expr = not.Node()
		prefix, op = "!", "!="
	}

	switch e := expr.(type) {
	case cloudapitype.PermissionExpressionHasTag:
		if !kind.isOwn(e.Context()) || !isShorthandSafe(e.Key()) {
			return "", false
		}
		return prefix + e.Key(), true
	case cloudapitype.PermissionExpressionEqual:
		tag, ok := e.Left().(cloudapitype.PermissionExpressionTag)
		if !ok || !kind.isOwn(tag.Context()) || !isShorthandSafe(tag.Key()) {
			return "", false
		}
		lit, ok := e.Right().(cloudapitype.PermissionLiteralExpressionString)
		if !ok || !isShorthandSafe(lit.Value()) {
			return "", false
		}
		return tag.Key() + op + lit.Value(), true
	}
	return "", false
}

// isShorthandSafe reports whether s reads back as the same key or value when
// written in the tag-condition syntax.
func isShorthandSafe(s string) bool {
	return s != "" && s != "OR" && s == strings.TrimSpace(s) &&
		!strings.ContainsAny(s, ",=") && !strings.HasPrefix(s, "!") && !strings.HasSuffix(s, "!") &&
		!strings.HasPrefix(s, "{") &&
		!strings.Contains(s, " OR ") && !strings.HasPrefix(s, "OR ") && !strings.HasSuffix(s, " OR")
}

func formatMatchRuleJSON(rule cloudapitype.PermissionBooleanExpression) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rule); err != nil {
		return fmt.Sprintf("<unprintable rule: %v>", err)
	}
	return strings.TrimSpace(buf.String())
}
