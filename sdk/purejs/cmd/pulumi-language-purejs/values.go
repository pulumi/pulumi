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

package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"google.golang.org/protobuf/types/known/structpb"
)

// The engine's property wire format (see sdk/go/common/resource/sig). This host reads it directly instead of through
// the SDK's property packages, which would bring in most of the SDK's dependencies.
const (
	sigKey            = "4dabf18193072939515e22adb298388d"
	secretSig         = "1b47061264138c4ac30d75fd1eb44270"
	resourceReference = "5cf8f73096256a8f31e491e813e4eb8e"
	outputValueSig    = "d0e6a833031e9bbcd3f4e8bde6ca49a4"
	byteStringSig     = "803fd3297a5875dc03ca845dda5d2a98"
	assetSig          = "c44067f5952c0a294b673a41bacd8c17"
	archiveSig        = "0def7320c3a5731c473e5ecbe6d01bc7"
)

var unknownValues = map[string]struct{}{
	"1c4a061d-8072-4f0a-a4cb-0ff528b18fe7": {}, // bool
	"3eeb2bf0-c639-47a8-9e75-3b44932eb421": {}, // number
	"04da6b54-80e4-46f7-96ec-b56ff0331ba9": {}, // string
	"6a19a0b0-7e62-4c92-b797-7f8e31da9cc2": {}, // array
	"030794c1-ac77-496b-92df-f27374a8bd58": {}, // asset
	"e48ece36-62e2-4504-bad9-02848725956a": {}, // archive
	"dd056dcd-154b-4c76-9bd3-c8f88648b5ff": {}, // object
}

func installHostFuncs(rt *goja.Runtime) error {
	if err := rt.Set("__log", func(level, msg string) {
		if verbose() {
			fmt.Fprintf(os.Stderr, "%s: %s\n", level, msg)
		}
	}); err != nil {
		return err
	}
	if err := rt.Set("__b64dec", func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.NewReplacer("-", "+", "_", "/").Replace(s)
		if b, err := base64.StdEncoding.DecodeString(s); err == nil {
			return string(b)
		}
		b, _ := base64.RawStdEncoding.DecodeString(strings.TrimRight(s, "="))
		return string(b)
	}); err != nil {
		return err
	}
	if err := rt.Set("__b64enc", func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}); err != nil {
		return err
	}
	return rt.Set("__parseURL", parseURL)
}

// parseURL returns the fields of a WHATWG URL object that policies read, or nil if input isn't an absolute URL.
func parseURL(input, base string) any {
	u, err := url.Parse(input)
	if err != nil {
		return nil
	}
	if base != "" {
		b, err := url.Parse(base)
		if err != nil {
			return nil
		}
		u = b.ResolveReference(u)
	}
	if u.Scheme == "" {
		return nil
	}
	search, hash := "", ""
	if u.RawQuery != "" {
		search = "?" + u.RawQuery
	}
	if u.Fragment != "" {
		hash = "#" + u.Fragment
	}
	path := u.EscapedPath()
	if path == "" && u.Host != "" {
		path = "/"
	}
	return map[string]any{
		"protocol": u.Scheme + ":",
		"hostname": u.Hostname(),
		"host":     u.Host,
		"port":     u.Port(),
		"pathname": path,
		"search":   search,
		"hash":     hash,
		"href":     u.String(),
		"origin":   u.Scheme + "://" + u.Host,
		"username": u.User.Username(),
	}
}

// propsToJS converts a resource's properties to the plain JS object a policy reads, the way the Node.js policy SDK
// presents them: secrets are unwrapped, resource references become their ID (or URN), and reading an unknown value
// throws. Internal keys ("__defaults" and the like) are dropped from the top level. Objects are built as native JS
// objects with sorted keys, so property order is stable and reads don't go through reflection.
func (pk *pack) propsToJS(props *structpb.Struct) *goja.Object {
	obj := pk.rt.NewObject()
	for _, k := range sortedKeys(props.GetFields()) {
		if strings.HasPrefix(k, "__") {
			continue
		}
		pk.setField(obj, k, props.GetFields()[k], k)
	}
	return obj
}

// setField sets obj[key] to v, or to a throwing getter if v is unknown.
func (pk *pack) setField(obj *goja.Object, key string, v *structpb.Value, path string) {
	val, known := pk.valueToJS(v, path)
	if known {
		_ = obj.Set(key, val)
		return
	}
	pk.defineUnknown(obj, key, path)
}

func (pk *pack) defineUnknown(obj *goja.Object, key, path string) {
	getter, err := pk.unknown(goja.Undefined(), pk.rt.ToValue(path))
	if err != nil {
		_ = obj.Set(key, goja.Undefined())
		return
	}
	_ = obj.DefineAccessorProperty(key, getter, nil, goja.FLAG_TRUE, goja.FLAG_TRUE)
}

// valueToJS converts one wire value. It returns false if the value is unknown.
func (pk *pack) valueToJS(v *structpb.Value, path string) (goja.Value, bool) {
	switch k := v.GetKind().(type) {
	case *structpb.Value_NullValue:
		return goja.Null(), true
	case *structpb.Value_BoolValue:
		return pk.rt.ToValue(k.BoolValue), true
	case *structpb.Value_NumberValue:
		return pk.rt.ToValue(k.NumberValue), true
	case *structpb.Value_StringValue:
		if _, unknown := unknownValues[k.StringValue]; unknown {
			return nil, false
		}
		return pk.rt.ToValue(k.StringValue), true
	case *structpb.Value_ListValue:
		elems := k.ListValue.GetValues()
		vals := make([]any, len(elems))
		var unknown []int
		for i, e := range elems {
			v, known := pk.valueToJS(e, path+"."+strconv.Itoa(i))
			if !known {
				unknown = append(unknown, i)
				v = goja.Undefined()
			}
			vals[i] = v
		}
		arr := pk.rt.NewArray(vals...)
		for _, i := range unknown {
			pk.defineUnknown(arr, strconv.Itoa(i), path+"."+strconv.Itoa(i))
		}
		return arr, true
	case *structpb.Value_StructValue:
		return pk.structToJS(k.StructValue, path)
	default:
		return goja.Undefined(), true
	}
}

func (pk *pack) structToJS(s *structpb.Struct, path string) (goja.Value, bool) {
	fields := s.GetFields()
	switch fields[sigKey].GetStringValue() {
	case "":
		obj := pk.rt.NewObject()
		for _, k := range sortedKeys(fields) {
			pk.setField(obj, k, fields[k], path+"."+k)
		}
		return obj, true
	case secretSig:
		if inner, ok := fields["value"]; ok {
			return pk.valueToJS(inner, path)
		}
		return goja.Undefined(), true
	case outputValueSig:
		inner, ok := fields["value"]
		if !ok {
			return nil, false
		}
		return pk.valueToJS(inner, path)
	case resourceReference:
		if id, ok := fields["id"]; ok {
			if _, unknown := unknownValues[id.GetStringValue()]; unknown || id.GetStringValue() == "" {
				return nil, false
			}
			return pk.rt.ToValue(id.GetStringValue()), true
		}
		return pk.rt.ToValue(fields["urn"].GetStringValue()), true
	case byteStringSig:
		b, err := base64.StdEncoding.DecodeString(fields["value"].GetStringValue())
		if err != nil {
			return goja.Undefined(), true
		}
		return pk.rt.ToValue(string(b)), true
	case assetSig, archiveSig:
		// Assets and archives keep their signature, so policies can tell them apart, plus whichever of hash, text,
		// path, uri and (for archives) assets they have.
		obj := pk.rt.NewObject()
		_ = obj.Set(sigKey, fields[sigKey].GetStringValue())
		for _, k := range []string{"hash", "text", "path", "uri"} {
			if s := fields[k].GetStringValue(); s != "" {
				_ = obj.Set(k, s)
			}
		}
		if assets, ok := fields["assets"]; ok && len(assets.GetStructValue().GetFields()) > 0 {
			_ = obj.Set("assets", pk.rt.ToValue(assets.GetStructValue().AsMap()))
		}
		return obj, true
	default:
		obj := pk.rt.NewObject()
		for _, k := range sortedKeys(fields) {
			pk.setField(obj, k, fields[k], path+"."+k)
		}
		return obj, true
	}
}

func sortedKeys(m map[string]*structpb.Value) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
