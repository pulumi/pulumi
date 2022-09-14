package client

import (
	"encoding/json"
	"testing"

	jpatch "github.com/evanphx/json-patch/v5"
	jpatch2 "github.com/mattbaird/jsonpatch"
	"pgregory.net/rapid"
)

type jsonPatchSystem struct {
	diff                  func(original, modified []byte) []byte
	patch                 func(original, patch []byte) []byte
	canonicalize          func(json []byte) []byte
	noNullValuesInObjects bool
	noNullValuesInArrays  bool
}

func canonicalizeJson(jsonData []byte) ([]byte, error) {
	var m interface{}
	if err := json.Unmarshal(jsonData, &m); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return canonical, nil
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
			return r
		},
		canonicalize: func(json []byte) []byte {
			c, err := canonicalizeJson(json)
			if err != nil {
				panic(err)
			}
			return c
		},
	}
	checkTurnaroundThoroughly(t, sys)
}

func TestRFC6902PatchTurnaround(t *testing.T) {

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
			return r
		},
		canonicalize: func(json []byte) []byte {
			c, err := canonicalizeJson(json)
			if err != nil {
				panic(err)
			}
			return c
		},
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

	t.Logf("original      = %v", string(original))
	t.Logf("modified      = %v", string(modified))

	patch := sys.diff(original, modified)

	t.Logf("patch         = %v", string(patch))

	reconstructed := sys.patch(original, patch)
	reconstructedNorm := sys.canonicalize(reconstructed)

	t.Logf("reconstructed = %v", string(reconstructedNorm))

	modifiedNorm := sys.canonicalize(modified)

	if string(reconstructedNorm) != string(modifiedNorm) {
		t.Fatalf("patch.Apply() did not match")
	}
}
