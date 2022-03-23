package httpstate

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStackString(t *testing.T) {
	t.Run("PrintsFullNames", func(t *testing.T) {
		defer func(prev bool) {
			PrintFullStackNames = prev
		}(PrintFullStackNames)
		PrintFullStackNames = true
		ref := cloudBackendReference{
			name:    "stackName",
			project: "projectName",
			owner:   "ownerName",
			b:       nil,
		}

		assert.Equal(t, fmt.Sprintf("%s/%s/%s", ref.owner, ref.project, ref.name), ref.String())
	})
}
