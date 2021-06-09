package python

import (
	"crypto/md5" //nolint: gosec
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Regress a problem of non-deterministic codegen (due to reordering).
// The schema is taken from `pulumi-aws` and minified to the smallest
// example that still reproduced the issue.
func TestGenResourceMappingsIsDeterministic(t *testing.T) {
	minimalSchema := `
        {
            "name": "aws",
            "meta": {
                "moduleFormat": "(.*)(?:/[^/]*)"
            },
            "resources": {
                "aws:acm/certificateValidation:CertificateValidation": {},
                "aws:acm/certificate:Certificate": {}
            },
            "language": {
                "python": {
                    "readme": ".."
                }
            }
        }`

	var pkgSpec schema.PackageSpec
	err := json.Unmarshal([]byte(minimalSchema), &pkgSpec)
	if err != nil {
		t.Error(err)
		return
	}

	generateInitHash := func() string {
		pkg, err := schema.ImportSpec(pkgSpec, nil)
		if err != nil {
			t.Error(err)
			return ""
		}

		files, err := GeneratePackage("tool", pkg, nil)
		if err != nil {
			t.Error(err)
			return ""
		}

		file, haveFile := files["pulumi_aws/__init__.py"]
		if !haveFile {
			t.Error("Cannot find pulumi_aws/__init__.py in the generated files")
			return ""
		}

		return fmt.Sprintf("%x", md5.Sum(file)) //nolint: gosec
	}

	h1 := generateInitHash()
	for i := 0; i < 16; i++ {
		assert.Equal(t, h1, generateInitHash())
	}
}
