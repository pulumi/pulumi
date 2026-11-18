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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Identifiable unknowns.
//
// During a scan the SDK sends each unknown input as an output value that says which resource output it is
// ({"source": {"urn", "property"}}), or how it was built from outputs ({"template": {"concat": parts}} or
// {"template": {"json": value}}). Policies get each such value as a stable text that names the output,
// "output-<hash of the URN>.<property>", so the same output compares equal wherever the program used it and a
// join such as `rule.securityGroupId === group.id` can be decided before anything is deployed. A string built from
// outputs is its text with each one named that way, and a JSON document its JSON. Stack policies also see each
// resource's `id`, and its `arn` when another resource reads it, as the same names.
//
// This is done here, before the resources are sent to the packs, so packs of any runtime see it and the analyzer
// protocol doesn't change. An unknown with no source (a value computed some other way) stays unknown, and a policy
// that reads it can't be decided.

// scanRefFor names an unknown output: the given property of the resource with the given URN.
func scanRefFor(urn, property string) string {
	sum := sha256.Sum256([]byte(urn))
	return "output-" + hex.EncodeToString(sum[:8]) + "." + property
}

// scanUnknowns rewrites the identifiable unknowns in the recorded inputs.
type scanUnknowns struct {
	byURN map[string]*scanResource
	// names maps each ref to "name.property", for messages.
	names map[string]string
	// read is the set of "urn\x00property" outputs that some input refers to.
	read map[string]bool
	// linked and unlinked count unknown inputs that are, or aren't, identifiable.
	linked, unlinked int
}

func newScanUnknowns(g *scanGraph) *scanUnknowns {
	u := &scanUnknowns{byURN: map[string]*scanResource{}, names: map[string]string{}, read: map[string]bool{}}
	for i := range g.Resources {
		u.byURN[g.Resources[i].URN] = &g.Resources[i]
	}
	for _, r := range g.Resources {
		for _, v := range r.Object {
			u.collectReads(v)
		}
	}
	return u
}

func (u *scanUnknowns) ref(urn, property string) string {
	ref := scanRefFor(urn, property)
	if _, ok := u.names[ref]; !ok {
		name := urn
		if r := u.byURN[urn]; r != nil {
			name = r.Name
		} else if resource.URN(urn).IsValid() {
			name = resource.URN(urn).Name()
		}
		u.names[ref] = name + "." + property
	}
	return ref
}

func sigOf(m map[string]any) string {
	s, _ := m[resource.SigKey].(string)
	return s
}

// collectReads records every output some input refers to, templates' parts included.
func (u *scanUnknowns) collectReads(v any) {
	switch v := v.(type) {
	case []any:
		for _, e := range v {
			u.collectReads(e)
		}
	case map[string]any:
		if sigOf(v) == resource.OutputValueSig {
			if s, ok := v["source"].(map[string]any); ok {
				urn, _ := s["urn"].(string)
				prop, _ := s["property"].(string)
				u.read[urn+"\x00"+prop] = true
			}
			if t, ok := v["template"].(map[string]any); ok {
				u.collectReads(t["concat"])
				u.collectReads(t["json"])
			}
			return
		}
		for _, e := range v {
			u.collectReads(e)
		}
	}
}

// rewrite returns v with each identifiable unknown output value replaced by its text. Other values are left as
// they are; unknowns without a source stay unknown.
func (u *scanUnknowns) rewrite(v any) any {
	switch v := v.(type) {
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = u.rewrite(e)
		}
		return out
	case map[string]any:
		switch sigOf(v) {
		case resource.OutputValueSig:
			if _, known := v["value"]; known {
				return v
			}
			if s, ok := u.resolve(v); ok {
				u.linked++
				return s
			}
			u.unlinked++
			// A plain unknown: a secret around it would hide that it's unknown from some pack hosts.
			return plugin.UnknownStringValue
		case "":
			out := make(map[string]any, len(v))
			for k, e := range v {
				out[k] = u.rewrite(e)
			}
			return out
		case resource.SecretSig:
			out := maps.Clone(v)
			out["value"] = u.rewrite(v["value"])
			return out
		case resource.ResourceReferenceSig:
			if s, ok := resourceRefText(v); ok {
				return s
			}
		}
	}
	return v
}

// resourceRefText is how policies see a resource passed as an input: its ID when known, else its URN, which names
// it as well.
func resourceRefText(v map[string]any) (string, bool) {
	if id, ok := v["id"].(string); ok && id != "" && id != plugin.UnknownStringValue {
		return id, true
	}
	if urn, ok := v["urn"].(string); ok && urn != "" {
		return urn, true
	}
	return "", false
}

// resolve is the text of an unknown output value: its output's ref, or the text of the template it was built from.
// False if it, or a part of its template, is an unknown without a source in this program.
func (u *scanUnknowns) resolve(ov map[string]any) (string, bool) {
	if s, ok := ov["source"].(map[string]any); ok {
		urn, _ := s["urn"].(string)
		prop, _ := s["property"].(string)
		if prop != "" {
			if _, inStack := u.byURN[urn]; inStack {
				return u.ref(urn, prop), true
			}
			return "", false
		}
	}
	t, ok := ov["template"].(map[string]any)
	if !ok {
		return "", false
	}
	if parts, ok := t["concat"].([]any); ok {
		var sb strings.Builder
		for _, p := range parts {
			s, ok := u.partText(p)
			if !ok {
				return "", false
			}
			sb.WriteString(s)
		}
		return sb.String(), true
	}
	if j, ok := t["json"]; ok {
		tree, ok := u.jsonTree(j)
		if !ok {
			return "", false
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(tree); err != nil {
			return "", false
		}
		return strings.TrimSuffix(buf.String(), "\n"), true
	}
	return "", false
}

func isUnknownValue(v any) bool {
	s, ok := v.(string)
	return ok && s == plugin.UnknownStringValue
}

// partText is a concat part as JavaScript's String() gives it.
func (u *scanUnknowns) partText(p any) (string, bool) {
	if isUnknownValue(p) {
		return "", false
	}
	switch p := p.(type) {
	case nil:
		return "null", true
	case string:
		return p, true
	case bool:
		return strconv.FormatBool(p), true
	case float64:
		return jsNumber(p), true
	case []any:
		ss := make([]string, len(p))
		for i, e := range p {
			if e == nil {
				continue
			}
			s, ok := u.partText(e)
			if !ok {
				return "", false
			}
			ss[i] = s
		}
		return strings.Join(ss, ","), true
	case map[string]any:
		switch sigOf(p) {
		case resource.OutputValueSig:
			if val, known := p["value"]; known {
				return u.partText(val)
			}
			return u.resolve(p)
		case resource.SecretSig:
			return u.partText(p["value"])
		case resource.ResourceReferenceSig:
			return resourceRefText(p)
		}
		return "[object Object]", true
	}
	return "", false
}

// jsonTree is a JSON template's value with each unknown as its text.
func (u *scanUnknowns) jsonTree(v any) (any, bool) {
	if isUnknownValue(v) {
		return nil, false
	}
	switch v := v.(type) {
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			ev, ok := u.jsonTree(e)
			if !ok {
				return nil, false
			}
			out[i] = ev
		}
		return out, true
	case map[string]any:
		switch sigOf(v) {
		case resource.OutputValueSig:
			if val, known := v["value"]; known {
				return u.jsonTree(val)
			}
			return u.resolve(v)
		case resource.SecretSig:
			return u.jsonTree(v["value"])
		case resource.ResourceReferenceSig:
			return resourceRefText(v)
		}
		out := make(map[string]any, len(v))
		for k, e := range v {
			ev, ok := u.jsonTree(e)
			if !ok {
				return nil, false
			}
			out[k] = ev
		}
		return out, true
	}
	return v, true
}

func jsNumber(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// stackOutputs is what stack policies see of r beyond its inputs: its `id`, and its `arn` when another resource
// reads it. Nothing was deployed, so both are the outputs' refs.
func (u *scanUnknowns) stackOutputs(r *scanResource, inputs map[string]any) map[string]any {
	out := map[string]any{}
	if _, has := inputs["id"]; !has {
		out["id"] = u.ref(r.URN, "id")
	}
	if _, has := inputs["arn"]; !has && u.read[r.URN+"\x00arn"] {
		out["arn"] = u.ref(r.URN, "arn")
	}
	return out
}

var scanRefPattern = regexp.MustCompile(`output-[0-9a-f]{16}\.[A-Za-z0-9_.]+`)

// describe replaces the refs in a policy's message with the outputs they stand for: "<output group.id>".
func (u *scanUnknowns) describe(msg string) string {
	if len(u.names) == 0 || !strings.Contains(msg, "output-") {
		return msg
	}
	return scanRefPattern.ReplaceAllStringFunc(msg, func(ref string) string {
		// Text after a ref in a joined string ("output-….id.s3.amazonaws.com") also matches: use the longest known
		// ref it starts with.
		for end := len(ref); end > 0; end = strings.LastIndexByte(ref[:end], '.') {
			if n, ok := u.names[ref[:end]]; ok {
				return "<output " + n + ">" + ref[end:]
			}
		}
		return ref
	})
}
