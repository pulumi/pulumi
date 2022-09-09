package client

import (
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

func TestPatchTurnaround(t *testing.T) {
	t.Run("general-3", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			maxHeight := 3
			g := &rapidJsonGen{rapidJsonOpts{
				noNullValuesInObjects: true,
			}}
			checkTurnaround(t, g.genJsonObject(maxHeight))
		})
	})
	t.Run("restricted-3", func(t *testing.T) {
		rapid.Check(t, func(t *rapid.T) {
			maxHeight := 3
			g := &rapidJsonGen{rapidJsonOpts{
				stringGen:             rapid.StringMatching("a|b"),
				intGen:                rapid.IntRange(1, 1),
				float64Gen:            rapid.Float64Range(2.0, 2.0),
				noNullValuesInObjects: true,
			}}
			checkTurnaround(t, g.genJsonObject(maxHeight))
		})
	})
}

func checkTurnaround(t *rapid.T, j *rapid.Generator) {
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
