// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ints

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
)

// TestProjectMain tests out the ability to override the main entrypoint.
func TestProjectMain(t *testing.T) {
	var test integration.ProgramTestOptions
	test = integration.ProgramTestOptions{
		Dir:          "project_main",
		Dependencies: []string{"pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.Equal(t, test.StackName(), checkpoint.Stack)
		},
	}
	integration.ProgramTest(t, &test)

	t.Run("Error_AbsolutePath", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")

		e.ImportDirectory("project_main_abs")
		e.RunCommand("pulumi", "stack", "init", "--local", "main-abs")
		stdout, stderr := e.RunCommandExpectError("pulumi", "update")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "project 'main' must be a relative path")
	})

	t.Run("Error_ParentFolder", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")
		e.RunCommand("pulumi", "init")

		e.ImportDirectory("project_main_parent")
		e.RunCommand("pulumi", "stack", "init", "--local", "main-parent")
		stdout, stderr := e.RunCommandExpectError("pulumi", "update")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "project 'main' must be a subfolder")
	})
}

// TestStackProjectName ensures we can read the Pulumi stack and project name from within the program.
func TestStackProjectName(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_project_name",
		Dependencies: []string{"pulumi"},
		Quick:        true,
	})
}

// TestStackOutputs ensures we can export variables from a stack and have them get recorded as outputs.
func TestStackOutputs(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_outputs",
		Dependencies: []string{"pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
			// Ensure the checkpoint contains a single resource, the Stack, with two outputs.
			assert.NotNil(t, checkpoint.Latest)
			if assert.Equal(t, 1, len(checkpoint.Latest.Resources)) {
				stackRes := checkpoint.Latest.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.URN.Type())
				assert.Equal(t, 0, len(stackRes.Inputs))
				assert.Equal(t, 2, len(stackRes.Outputs))
				assert.Equal(t, "ABC", stackRes.Outputs["xyz"])
				assert.Equal(t, float64(42), stackRes.Outputs["foo"])
			}
		},
	})
}

// TestStackParenting tests out that stacks and components are parented correctly.
func TestStackParenting(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "stack_parenting",
		Dependencies: []string{"pulumi"},
		Quick:        true,
		ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
			// Ensure the checkpoint contains resources parented correctly.  This should look like this:
			//
			//     A      F
			//    / \      \
			//   B   C      G
			//      / \
			//     D   E
			//
			// with the caveat, of course, that A and F will share a common parent, the implicit stack.

			assert.NotNil(t, checkpoint.Latest)
			if assert.Equal(t, 8, len(checkpoint.Latest.Resources)) {
				stackRes := checkpoint.Latest.Resources[0]
				assert.NotNil(t, stackRes)
				assert.Equal(t, resource.RootStackType, stackRes.Type)
				assert.Equal(t, "", string(stackRes.Parent))
				a := checkpoint.Latest.Resources[1]
				assert.NotNil(t, a)
				assert.Equal(t, "a", string(a.URN.Name()))
				assert.NotEqual(t, "", a.Parent)
				assert.Equal(t, stackRes.URN, a.Parent)
				b := checkpoint.Latest.Resources[2]
				assert.NotNil(t, b)
				assert.Equal(t, "b", string(b.URN.Name()))
				assert.Equal(t, a.URN, b.Parent)
				c := checkpoint.Latest.Resources[3]
				assert.NotNil(t, c)
				assert.Equal(t, "c", string(c.URN.Name()))
				assert.Equal(t, a.URN, c.Parent)
				d := checkpoint.Latest.Resources[4]
				assert.NotNil(t, d)
				assert.Equal(t, "d", string(d.URN.Name()))
				assert.Equal(t, c.URN, d.Parent)
				e := checkpoint.Latest.Resources[5]
				assert.NotNil(t, e)
				assert.Equal(t, "e", string(e.URN.Name()))
				assert.Equal(t, c.URN, e.Parent)
				f := checkpoint.Latest.Resources[6]
				assert.NotNil(t, f)
				assert.Equal(t, "f", string(f.URN.Name()))
				assert.Equal(t, stackRes.URN, f.Parent)
				g := checkpoint.Latest.Resources[7]
				assert.NotNil(t, g)
				assert.Equal(t, "g", string(g.URN.Name()))
				assert.Equal(t, f.URN, g.Parent)
			}
		},
	})
}
