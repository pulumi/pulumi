package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBatches(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10} {
		n := n
		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			t.Parallel()

			var combined []ProgramTest
			for i := 1; i <= n; i++ {
				combined = append(combined, ProgramTestBatch(i, n)...)
			}

			assert.ElementsMatch(t, PulumiPulumiProgramTests, combined)
		})
	}
}
