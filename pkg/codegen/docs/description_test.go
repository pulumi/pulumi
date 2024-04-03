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
	inputBytes, err := os.ReadFile(filepath.Join("testdata", "lambda-description-in.md"))
	require.NoError(t, err)
	input := string(inputBytes)
	dctx := newDocGenContext()
	docInfo := dctx.processDescription(input)
	actual := docInfo.description

	expectedBytes, err := os.ReadFile(filepath.Join("testdata", "lambda-description-out.md"))
	require.NoError(t, err)
	expected := string(expectedBytes)
	assert.Equal(t, expected, actual)
}

func TestDecomposeDocstringRendersCodeChoosers(t *testing.T) {
	t.Parallel()
	inputBytes, err := os.ReadFile(filepath.Join("testdata", "lambda-description-in.md"))
	require.NoError(t, err)
	input := string(inputBytes)
	dctx := newDocGenContext()
	docInfo := dctx.decomposeDocstring(input)
	actual := docInfo.description

	expectedBytes, err := os.ReadFile(filepath.Join("testdata", "lambda-description-out.md"))
	require.NoError(t, err)
	expected := string(expectedBytes)
	assert.Equal(t, expected, actual)
}

func TestDecomposeDocstringRendersLegacyShortcodeExamples(t *testing.T) {
	t.Parallel()
	inputBytes, err := os.ReadFile(filepath.Join("testdata", "certificate-validation-description-in.md"))
	require.NoError(t, err)
	input := string(inputBytes)
	dctx := newDocGenContext()
	docInfo := dctx.decomposeDocstring(input)
	actual := docInfo.description

	expectedBytes, err := os.ReadFile(filepath.Join("testdata", "certificate-validation-description-out.md"))
	require.NoError(t, err)
	expected := string(expectedBytes)
	assert.Equal(t, expected, actual)
}
