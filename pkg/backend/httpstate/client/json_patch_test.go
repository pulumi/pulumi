package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	jpatch "github.com/evanphx/json-patch/v5"
	jpatch2 "github.com/mattbaird/jsonpatch"
	"github.com/sergi/go-diff/diffmatchpatch"
	"pgregory.net/rapid"
)

type jsonPatchSystem struct {
	diff                  func(original, modified []byte) []byte
	patch                 func(original, patch []byte) []byte
	canonicalize          func(json json.RawMessage) json.RawMessage
	noNullValuesInObjects bool
	noNullValuesInArrays  bool
}

func canonicalizeJson(jsonData json.RawMessage) (json.RawMessage, error) {
	var m interface{}
	if err := json.Unmarshal(jsonData, &m); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	canonical := buf.Bytes()
	return canonical, nil
}

func canonicalize(json json.RawMessage) json.RawMessage {
	c, err := canonicalizeJson(json)
	if err != nil {
		panic(err)
	}
	return c
}

func TestCanonicalizePreservesDeepEqual(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxHeight := 3
		g := &rapidJsonGen{rapidJsonOpts{}}
		x := g.genJsonValue(maxHeight).Draw(t, "x").(json.RawMessage)
		var a, b interface{}
		if err := json.Unmarshal(canonicalize(x), &a); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if err := json.Unmarshal(x, &b); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("Does not turnaround")
		}
	})
}

func TestTextDiffTurnaround(t *testing.T) {
	diff := func(original, modified []byte) []byte {
		dmp := diffmatchpatch.New()
		patches := dmp.PatchMake(
			string(canonicalize(original)),
			string(canonicalize(modified)),
		)
		return []byte(dmp.PatchToText(patches))
	}
	patch := func(original, patch []byte) []byte {
		dmp := diffmatchpatch.New()
		patches, err := dmp.PatchFromText(string(patch))
		if err != nil {
			panic(err)
		}
		patched, applies := dmp.PatchApply(patches,
			string(canonicalize(original)))
		for i, a := range applies {
			if !a {
				panic(fmt.Errorf("Patch %d failed", i))
			}
		}
		return []byte(patched)
	}
	sys := jsonPatchSystem{
		diff:         diff,
		patch:        patch,
		canonicalize: canonicalize,
	}
	checkTurnaroundThoroughly(t, sys)
}

func TestRFC7396PatchTurnaround(t *testing.T) {
	// RFC7396 JSON merge patches. Requires canonicalization
	// as fields may be reordered. Requires eliminating nil keys
	// from JSON objects.

	// Note that another weird issue with this patch format is
	// that the patch JSON format itself requires meaningful nil
	// keys in JSON objects to indicate deletion, these should not
	// drop during transport.

	sys := jsonPatchSystem{
		noNullValuesInObjects: true,
		diff: func(original, modified []byte) []byte {
			p, err := jpatch.CreateMergePatch(original, modified)
			if err != nil {
				panic(err)
			}
			return p
		},
		patch: func(original, patch []byte) []byte {
			r, err := jpatch.MergePatch(original, patch)
			if err != nil {
				panic(err)
			}
			return canonicalize(r)
		},
		canonicalize: canonicalize,
	}
	checkTurnaroundThoroughly(t, sys)
}

func TestRFC6902PatchTurnaround(t *testing.T) {
	t.Skip("Failures detected")

	// With RFC6902, evanphx/json-patch does not support the diff operation,
	// so this system tries to use mattbaird/jsonpatch for the diff.
	sys := jsonPatchSystem{
		noNullValuesInObjects: true,
		noNullValuesInArrays:  true,
		diff: func(original, modified []byte) []byte {
			operations, err := jpatch2.CreatePatch(original, modified)
			if err != nil {
				panic(err)
			}
			var jsonPatch []json.RawMessage
			for _, op := range operations {
				jsonPatch = append(jsonPatch, json.RawMessage(op.Json()))
			}
			p, err := json.Marshal(jsonPatch)
			if err != nil {
				panic(err)
			}
			return p
		},
		patch: func(original, patch []byte) []byte {
			p, err := jpatch.DecodePatch(patch)
			if err != nil {
				panic(err)
			}
			r, err := p.Apply(original)
			if err != nil {
				panic(err)
			}
			return canonicalize(r)
		},
		canonicalize: canonicalize,
	}
	checkTurnaroundThoroughly(t, sys)
}

func checkTurnaroundThoroughly(t *testing.T, sys jsonPatchSystem) {
	t.Run("general-3", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			maxHeight := 3
			g := &rapidJsonGen{rapidJsonOpts{
				noNullValuesInObjects: sys.noNullValuesInObjects,
				noNullValuesInArrays:  sys.noNullValuesInArrays,
			}}
			checkTurnaround(t, g.genJsonObject(maxHeight), sys)
		})
	})

	t.Run("restricted-3", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			maxHeight := 3
			g := &rapidJsonGen{rapidJsonOpts{
				noNullValuesInObjects: sys.noNullValuesInObjects,
				noNullValuesInArrays:  sys.noNullValuesInArrays,
				stringGen:             rapid.StringMatching("a|b"),
				intGen:                rapid.IntRange(1, 1),
				float64Gen:            rapid.Float64Range(1.0, 1.0),
				boolGen: rapid.Bool().
					Map(func(x interface{}) bool { return x.(bool) }),
			}}
			checkTurnaround(t, g.genJsonObject(maxHeight), sys)
		})
	})
}

func checkTurnaround(t *rapid.T, j *rapid.Generator, sys jsonPatchSystem) {
	original := j.Draw(t, "original").(json.RawMessage)
	modified := j.Draw(t, "modified").(json.RawMessage)

	patch := sys.diff(original, modified)
	reconstructed := sys.patch(original, patch)

	t.Logf("original               = %v", string(original))
	t.Logf("modified               = %v", string(modified))
	t.Logf("canonicalize(modified) = %v", string(sys.canonicalize(modified)))
	t.Logf("patch                  = %v", string(patch))
	t.Logf("reconstructed          = %v", string(reconstructed))

	if string(reconstructed) != string(sys.canonicalize(modified)) {
		t.Fatalf("patch.Apply() did not match")
	}
}
