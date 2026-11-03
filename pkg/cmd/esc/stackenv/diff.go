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

package stackenv

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/eval"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// ChangeKind is the kind of a DefinitionChange.
type ChangeKind int

const (
	// ChangeAdd is a value present only in the local definition.
	ChangeAdd ChangeKind = iota
	// ChangeRemove is a value present only in the published definition.
	ChangeRemove
	// ChangeUpdate is a value present in both definitions with different contents.
	ChangeUpdate
)

// DefinitionChange is one changed value between a published and a local environment definition.
type DefinitionChange struct {
	Kind ChangeKind
	// Path is the value's path in the definition, with keys as written there.
	Path resource.PropertyPath
	// Old and New are the rendered published and local values. Old is empty for ChangeAdd and New is empty for
	// ChangeRemove. A secret renders as [secret], or as the JSON string "[secret]" inside a list or mapping. An
	// update that contains a secret on either side prints no values: each side renders as [secret] if it contains a
	// secret and as [plaintext] otherwise. A rendered value longer than 256 bytes is cut there and followed by its
	// full length, as in "abc... (1048576 bytes)".
	Old, New string
}

const (
	redacted          = "[secret]"
	redactedPlaintext = "[plaintext]"
	maxRenderedBytes  = 256
)

type secretValue struct {
	argument any
}

// DiffDefinitions lists the values that differ between the published and local definitions of an environment, sorted
// by key within each mapping. Formatting, comments and key order are not compared. Secrets are compared by their
// ciphertext, so a re-encrypted secret is reported as changed.
func DiffDefinitions(published, local []byte) ([]DefinitionChange, error) {
	old, err := decodeDefinition(published)
	if err != nil {
		return nil, fmt.Errorf("published definition: %w", err)
	}
	new, err := decodeDefinition(local)
	if err != nil {
		return nil, fmt.Errorf("local definition: %w", err)
	}
	var changes []DefinitionChange
	diffValues(nil, old, true, new, true, &changes)
	return changes, nil
}

// WriteDefinitionDiff writes a header naming env and the revision the local changes are based on, followed by one
// line per change.
func WriteDefinitionDiff(
	w io.Writer,
	color colors.Colorization,
	env string,
	baseRevision int,
	changes []DefinitionChange,
) {
	var noChanges string
	if len(changes) == 0 {
		noChanges = " (no value changes)"
	}
	fmt.Fprintf(w,
		"Environment %s: local changes based on revision %d%s; an update publishes them, a preview does not\n",
		env, baseRevision, noChanges)
	for _, c := range changes {
		var spec, text string
		switch c.Kind {
		case ChangeAdd:
			spec, text = colors.SpecCreate, fmt.Sprintf("+ %s: %s", c.Path, c.New)
		case ChangeRemove:
			spec, text = colors.SpecDelete, fmt.Sprintf("- %s: %s", c.Path, c.Old)
		case ChangeUpdate:
			spec, text = colors.SpecUpdate, fmt.Sprintf("~ %s: %s -> %s", c.Path, c.Old, c.New)
		}
		fmt.Fprintf(w, "    %s\n", color.Colorize(spec+terminalSafe(text)+colors.Reset))
	}
}

func decodeDefinition(definition []byte) (map[string]any, error) {
	node, diags := encoding.DecodeYAMLBytes("", definition, eval.TagDecoder)
	if len(diags) != 0 {
		return nil, diags
	}
	switch root := plainValue(node).(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return root, nil
	default:
		return nil, fmt.Errorf("the top level is %s, not a mapping", renderValue(root))
	}
}

func plainValue(node syntax.Node) any {
	switch node := node.(type) {
	case *syntax.ObjectNode:
		if isSecretCall(node) {
			return secretValue{plainValue(node.Index(0).Value)}
		}
		m := make(map[string]any, node.Len())
		for i := range node.Len() {
			m[node.Index(i).Key.Value()] = plainValue(node.Index(i).Value)
		}
		return m
	case *syntax.ArrayNode:
		a := make([]any, node.Len())
		for i := range node.Len() {
			a[i] = plainValue(node.Index(i))
		}
		return a
	case *syntax.StringNode:
		return node.Value()
	case *syntax.NumberNode:
		return node.Value()
	case *syntax.BooleanNode:
		return node.Value()
	default:
		return nil
	}
}

// isSecretCall mirrors ast.tryParseFunction's fn::secret match.
func isSecretCall(node *syntax.ObjectNode) bool {
	return node.Len() == 1 && node.Index(0).Key.Value() == "fn::secret"
}

func diffValues(path resource.PropertyPath, old any, inOld bool, new any, inNew bool, changes *[]DefinitionChange) {
	switch {
	case !inOld:
		*changes = append(*changes, DefinitionChange{Kind: ChangeAdd, Path: path, New: renderTruncated(new)})
		return
	case !inNew:
		*changes = append(*changes, DefinitionChange{Kind: ChangeRemove, Path: path, Old: renderTruncated(old)})
		return
	case reflect.DeepEqual(old, new):
		return
	}

	oldMap, oldIsMap := old.(map[string]any)
	newMap, newIsMap := new.(map[string]any)
	if oldIsMap && newIsMap {
		keys := slices.Concat(slices.Collect(maps.Keys(oldMap)), slices.Collect(maps.Keys(newMap)))
		slices.Sort(keys)
		for _, k := range slices.Compact(keys) {
			oldValue, oldHas := oldMap[k]
			newValue, newHas := newMap[k]
			diffValues(slices.Concat(path, resource.PropertyPath{k}), oldValue, oldHas, newValue, newHas, changes)
		}
		return
	}

	oldList, oldIsList := old.([]any)
	newList, newIsList := new.([]any)
	if oldIsList && newIsList {
		for i := range max(len(oldList), len(newList)) {
			oldValue, oldHas := element(oldList, i)
			newValue, newHas := element(newList, i)
			diffValues(slices.Concat(path, resource.PropertyPath{i}), oldValue, oldHas, newValue, newHas, changes)
		}
		return
	}

	change := DefinitionChange{Kind: ChangeUpdate, Path: path, Old: renderTruncated(old), New: renderTruncated(new)}
	if containsSecret(old) || containsSecret(new) {
		change.Old, change.New = redactedLabel(old), redactedLabel(new)
	}
	*changes = append(*changes, change)
}

func renderTruncated(v any) string {
	rendered := renderValue(v)
	if len(rendered) <= maxRenderedBytes {
		return rendered
	}
	cut := maxRenderedBytes
	for cut > 0 && !utf8.RuneStart(rendered[cut]) {
		cut--
	}
	return fmt.Sprintf("%s... (%d bytes)", rendered[:cut], len(rendered))
}

func redactedLabel(v any) string {
	if containsSecret(v) {
		return redacted
	}
	return redactedPlaintext
}

func containsSecret(v any) bool {
	switch v := v.(type) {
	case secretValue:
		return true
	case map[string]any:
		for _, e := range v {
			if containsSecret(e) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(v, containsSecret)
	}
	return false
}

func element(list []any, i int) (any, bool) {
	if i < len(list) {
		return list[i], true
	}
	return nil, false
}

// ESC decodes .inf and .nan to the json.Numbers +Inf and NaN, which json.Marshal rejects.
func renderValue(v any) string {
	switch v := v.(type) {
	case secretValue:
		return redacted
	case map[string]any:
		entries := make([]string, 0, len(v))
		for _, k := range slices.Sorted(maps.Keys(v)) {
			entries = append(entries, quote(k)+":"+renderElement(v[k]))
		}
		return "{" + strings.Join(entries, ",") + "}"
	case []any:
		elements := make([]string, len(v))
		for i, e := range v {
			elements[i] = renderElement(e)
		}
		return "[" + strings.Join(elements, ",") + "]"
	case string:
		return quote(v)
	case json.Number:
		return string(v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return "null"
	}
}

func renderElement(v any) string {
	if _, ok := v.(secretValue); ok {
		return quote(redacted)
	}
	return renderValue(v)
}

func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// Colorize reads <{% as the start of a directive, and a non-graphic rune anywhere in the line can forge, hide or
// reorder lines.
func terminalSafe(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case strings.HasPrefix(s[i:], "<{%"):
			b.WriteString(`\u003c`)
		case unicode.IsGraphic(r):
			b.WriteRune(r)
		case r > 0xFFFF:
			fmt.Fprintf(&b, `\U%08x`, r)
		default:
			fmt.Fprintf(&b, `\u%04x`, r)
		}
	}
	return b.String()
}
