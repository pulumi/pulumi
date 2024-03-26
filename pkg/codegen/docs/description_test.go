package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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

			inputFile := filepath.Join("test_data", tt.prefix+"-in.md")
			expectedFile := filepath.Join("test_data", tt.prefix+"-out.md")

			input, expected := readFile(t, inputFile), readFile(t, expectedFile)

			docInfo := newDocGenContext().processDescription(input)
			actual := docInfo.description

			assert.Equal(t, expected, actual)
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

			inputFile := filepath.Join("test_data", tt.prefix+"-in.md")
			expectedFile := filepath.Join("test_data", tt.prefix+"-out.md")

			input, expected := readFile(t, inputFile), readFile(t, expectedFile)

			docInfo := newDocGenContext().decomposeDocstring(input)
			actual := docInfo.description

			assert.Equal(t, expected, actual)
		})
	}
}

func readFile(t *testing.T, filepath string) string {
	inputBytes, err := os.ReadFile(filepath)
	require.NoError(t, err)
	return string(inputBytes)
}
