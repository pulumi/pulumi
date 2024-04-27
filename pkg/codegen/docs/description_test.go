package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"
)

func TestProcessDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prefix string
	}{
		{"lambda-description"},
		{"scaleway-k8s-cluster-description"}, // Repro: https://github.com/pulumi/registry/issues/4202
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.prefix, func(t *testing.T) {
			t.Parallel()

			input := readFile(t, filepath.Join("testdata", tt.prefix+".md"))

			actual := newDocGenContext().processDescription(input).description

			autogold.ExpectFile(t, autogold.Raw(actual))
		})
	}
}

func TestDecomposeDocstringDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		prefix string
	}{
		{"lambda-description"},                 // renders code choosers
		{"certificate-validation-description"}, // renders legacy shortcode examples
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.prefix, func(t *testing.T) {
			t.Parallel()

			input := readFile(t, filepath.Join("testdata", tt.prefix+".md"))

			actual := newDocGenContext().decomposeDocstring(input).description

			autogold.ExpectFile(t, autogold.Raw(actual))
		})
	}
}

func readFile(t *testing.T, filepath string) string {
	inputBytes, err := os.ReadFile(filepath)
	require.NoError(t, err)
	return string(inputBytes)
}
