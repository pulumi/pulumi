package yaml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func decodeFile(path string) (*Package, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pkg *Package
	if err := yaml.NewDecoder(f).Decode(&pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

func TestSchemas(t *testing.T) {
	files, err := os.ReadDir("testdata")
	require.NoError(t, err)

	for _, f := range files {
		name := f.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			syntax, err := decodeFile(filepath.Join("testdata", name))
			require.NoError(t, err)

			spec, err := syntax.Spec()
			require.NoError(t, err)

			_, err = schema.ImportSpec(*spec, nil)
			assert.NoError(t, err)

			bytes, err := json.MarshalIndent(spec, "", "    ")
			require.NoError(t, err)

			jsonPath := filepath.Join("testdata", name[:len(name)-len(".yaml")]+".json")
			if os.Getenv("PULUMI_ACCEPT") != "" {
				err = os.WriteFile(jsonPath, bytes, 0600)
				require.NoError(t, err)
			} else {
				expected, err := os.ReadFile(jsonPath)
				require.NoError(t, err)

				assert.Equal(t, expected, bytes)
			}
		})
	}
}
