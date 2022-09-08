package client

import (
	"bytes"
	"encoding/json"
	"testing"

	jpatch "github.com/evanphx/json-patch/v5"
	"pgregory.net/rapid"
)

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

func decodePatch(raw []byte) (jpatch.Patch, error) {
	if bytes.Equal(raw, []byte(`{}`)) {
		return []jpatch.Operation{}, nil
	}
	return jpatch.DecodePatch(marshal([]json.RawMessage{raw}))
}

func TestPatchTurnaround(t *testing.T) {
	t.Run("general-3", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			checkTurnaround(t, rapidJsonOpts{
				maxHeight:  3,
				objectOnly: true,
				allowNull:  false,
			})
		})
	})
	t.Run("restricted-3", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			checkTurnaround(t, rapidJsonOpts{
				maxHeight:  3,
				objectOnly: true,
				allowNull:  false,
				stringGen:  rapid.StringMatching("a|b"),
				intGen:     rapid.IntRange(1, 1),
				float64Gen: rapid.Float64Range(2.0, 2.0),
			})
		})
	})
}

func checkTurnaround(t *rapid.T, opts rapidJsonOpts) {
	j := rapidJson(opts)
	original := j.Draw(t, "original").(json.RawMessage)
	modified := j.Draw(t, "modified").(json.RawMessage)
	t.Logf("original=%v", string(original))
	t.Logf("modified=%v", string(modified))

	mergePatch, err := jpatch.CreateMergePatch(original, modified)
	if err != nil {
		t.Fatalf("CreateMergePatch failed: %v", err)
	}
	t.Logf("mergePatch=%v", string(mergePatch))

	reconstructed, err := jpatch.MergePatch(original, mergePatch)
	if err != nil {
		t.Fatalf("MergePatch failed: %v", err)
	}

	reconstructedNorm, err := canonicalizeJson(reconstructed)
	if err != nil {
		t.Fatalf("canonicalizeJson failed: %v", err)
	}
	t.Logf("reconstr=%v", string(reconstructedNorm))

	modifiedNorm, err := canonicalizeJson(modified)
	if err != nil {
		t.Fatalf("canonicalizeJson failed: %v", err)
	}

	if string(reconstructedNorm) != string(modifiedNorm) {
		t.Fatalf("patch.Apply() did not match")
	}
}

// Generates arbitrary JSON trees as json.RawMessage obtained by
// default json.Marshal settings from all possible map/slice
// possibilities.
//
// Excludes null values from maps.
func rapidJson(opts rapidJsonOpts) *rapid.Generator {
	strG := opts.stringGen
	if strG == nil {
		strG = rapid.String()
	}

	intG := opts.intGen
	if intG == nil {
		intG = rapid.Int()
	}

	f64G := opts.float64Gen
	if f64G == nil {
		f64G = rapid.Float64()
	}

	options := []*rapid.Generator{
		rapid.Just(json.RawMessage(`true`)),
		rapid.Just(json.RawMessage(`false`)),
		strG.Map(func(x string) json.RawMessage { return marshal(x) }),
		intG.Map(func(x int) json.RawMessage { return marshal(x) }),
		f64G.Map(func(x float64) json.RawMessage { return marshal(x) }),
	}

	if opts.allowNull {
		options = append(options, rapid.Just(json.RawMessage(`null`)))
	}

	if opts.maxHeight > 1 {
		object := rapid.MapOf(strG, rapidJson(rapidJsonOpts{
			maxHeight:  opts.maxHeight - 1,
			objectOnly: false,
			allowNull:  false,
			stringGen:  opts.stringGen,
			intGen:     opts.intGen,
			float64Gen: opts.float64Gen,
		})).Map(func(x map[string]interface{}) json.RawMessage { return marshal(x) })

		if opts.objectOnly {
			return object
		}

		array := rapid.SliceOf(rapidJson(rapidJsonOpts{
			maxHeight:  opts.maxHeight - 1,
			objectOnly: false,
			allowNull:  true,
			stringGen:  opts.stringGen,
			intGen:     opts.intGen,
			float64Gen: opts.float64Gen,
		})).Map(func(x []interface{}) json.RawMessage { return marshal(x) })

		options = append(options, array, object)
	}

	return rapid.OneOf(options...)
}

type rapidJsonOpts struct {
	maxHeight  int
	objectOnly bool
	allowNull  bool
	stringGen  *rapid.Generator
	intGen     *rapid.Generator
	float64Gen *rapid.Generator
}

func marshal(x interface{}) json.RawMessage {
	bytes, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return bytes
}
