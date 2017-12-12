// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ints

import (
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/stretchr/testify/assert"
)

type EditDirWithValidation struct {
	*integration.EditDir
	Expected string
}

func testPreviewUpdatesAndEdits(t *testing.T, opts *integration.ProgramTestOptions, dir string) string {
	return integration.TestPreviewAndUpdates(t, opts, dir, testEdits)
}

func testEdits(t *testing.T, opts *integration.ProgramTestOptions, dir string) string {

	var edits = []EditDirWithValidation{
		{
			&integration.EditDir{
				Dir:      "step2",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					assert.Equal(t, 5, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					b := checkpoint.Latest.Resources[2]
					assert.Equal(t, "b", string(b.URN.Name()))
					c := checkpoint.Latest.Resources[3]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := checkpoint.Latest.Resources[4]
					assert.Equal(t, "e", string(e.URN.Name()))
				},
			},
			"",
		},
		{
			&integration.EditDir{
				Dir:      "step3",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					assert.Equal(t, 4, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := checkpoint.Latest.Resources[2]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := checkpoint.Latest.Resources[3]
					assert.Equal(t, "e", string(e.URN.Name()))
				},
			},
			"",
		},
		{
			&integration.EditDir{
				Dir:      "step4",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					// assert.Equal(t, 5, len(checkpoint.Latest.Resources))
					assert.Equal(t, 4, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
					a := checkpoint.Latest.Resources[1]
					assert.Equal(t, "a", string(a.URN.Name()))
					c := checkpoint.Latest.Resources[2]
					assert.Equal(t, "c", string(c.URN.Name()))
					e := checkpoint.Latest.Resources[3]
					assert.Equal(t, "e", string(e.URN.Name()))
					// aPendingDelete := checkpoint.Latest.Resources[4]
					// assert.Equal(t, "a", string(aPendingDelete.URN.Name()))
					// assert.True(t, aPendingDelete.Delete)
				},
			},
			"",
		},
		{
			&integration.EditDir{
				Dir:      "step5",
				Additive: true,
				ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
					assert.NotNil(t, checkpoint.Latest)
					assert.Equal(t, 1, len(checkpoint.Latest.Resources))
					stackRes := checkpoint.Latest.Resources[0]
					assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				},
			},
			"",
		},
	}

	var buf bytes.Buffer
	optsCopy := *opts
	optsCopy.Stdout = &buf

	for i, edit := range edits {
		dir = integration.TestEdit(t, &optsCopy, dir, i, *edit.EditDir)

		actual := buf.String()
		assert.Equal(t, edit.Expected, actual)

		buf.Reset()
	}

	return dir
}

// TestSteps tests many combinations of creates, updates, deletes, replacements, and so on.
func TestSteps(t *testing.T) {
	t.Parallel()

	opts := integration.ProgramTestOptions{
		Dir:          "step1",
		Dependencies: []string{"pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
			assert.NotNil(t, checkpoint.Latest)
			assert.Equal(t, 5, len(checkpoint.Latest.Resources))
			stackRes := checkpoint.Latest.Resources[0]
			assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
			a := checkpoint.Latest.Resources[1]
			assert.Equal(t, "a", string(a.URN.Name()))
			b := checkpoint.Latest.Resources[2]
			assert.Equal(t, "b", string(b.URN.Name()))
			c := checkpoint.Latest.Resources[3]
			assert.Equal(t, "c", string(c.URN.Name()))
			d := checkpoint.Latest.Resources[4]
			assert.Equal(t, "d", string(d.URN.Name()))
		},
	}

	integration.TestLifeCycleInitAndDestroy(t, &opts, testPreviewUpdatesAndEdits)
}
