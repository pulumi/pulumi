// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ints

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/stack"
	ptesting "github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/testing/integration"
	"github.com/pulumi/pulumi/pkg/tokens"
)

// TestProjectMain tests out the ability to override the main entrypoint.
func TestProjectMain(t *testing.T) {
	var test integration.ProgramTestOptions
	test = integration.ProgramTestOptions{
		Dir:          "project_main",
		Dependencies: []string{"pulumi"},
		ExtraRuntimeValidation: func(t *testing.T, checkpoint stack.Checkpoint) {
			// Simple runtime validation that just ensures the checkpoint was written and read.
			assert.Equal(t, test.GetStackName(), checkpoint.Stack)
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

// TestConfigSave ensures that config commands in the Pulumi CLI work as expected.
func TestConfigSave(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	// Initialize an empty stack.
	pkgpath := filepath.Join(e.RootPath, "Pulumi.yaml")
	err := pack.Save(pkgpath, &pack.Package{
		Name:    "testing-config",
		Runtime: "nodejs",
	})
	assert.NoError(t, err)
	e.RunCommand("git", "init")
	e.RunCommand("pulumi", "init")
	e.RunCommand("pulumi", "stack", "init", "--local", "testing-2")
	e.RunCommand("pulumi", "stack", "init", "--local", "testing-1")

	// Now configure and save a few different things:
	//     1) do not save.
	e.RunCommand("pulumi", "config", "set", "configA", "value1", "--save=false")
	//     2) save to the project file, under the current stack.
	e.RunCommand("pulumi", "config", "set", "configB", "value2")
	//     3) save to the project file, underneath an entirely different stack.
	e.RunCommand("pulumi", "config", "set", "configC", "value3", "--stack", "testing-2")
	//     4) save to the project file, across all stacks.
	e.RunCommand("pulumi", "config", "set", "configD", "value4", "--all")
	//     5) save the same config key with a different value in the stack versus all stacks.
	e.RunCommand("pulumi", "config", "set", "configE", "value55")
	e.RunCommand("pulumi", "config", "set", "configE", "value66", "--all")

	// Now read back the config using the CLI:
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configA")
		assert.Equal(t, "value1\n", stdout)
	}
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configB")
		assert.Equal(t, "value2\n", stdout)
	}
	{
		// config is in a different stack, should yield a stderr:
		stdout, stderr := e.RunCommandExpectError("pulumi", "config", "get", "configC")
		assert.Equal(t, "", stdout)
		assert.NotEqual(t, "", stderr)
	}
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configC", "--stack", "testing-2")
		assert.Equal(t, "value3\n", stdout)
	}
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configD")
		assert.Equal(t, "value4\n", stdout)
	}
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configE")
		assert.Equal(t, "value55\n", stdout)
	}

	// Finally, check that the project file contains what we expected.
	cfgkey := func(k string) tokens.ModuleMember { return tokens.ModuleMember("testing-config:config:" + k) }
	pkg, err := pack.Load(pkgpath)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(pkg.Config)) // --all
	d, ok := pkg.Config[cfgkey("configD")]
	assert.True(t, ok)
	dv, err := d.Value(nil)
	assert.NoError(t, err)
	assert.Equal(t, "value4", dv)
	ee, ok := pkg.Config[cfgkey("configE")]
	assert.True(t, ok)
	ev, err := ee.Value(nil)
	assert.NoError(t, err)
	assert.Equal(t, "value66", ev)
	assert.Equal(t, 2, len(pkg.Stacks))
	assert.Equal(t, 2, len(pkg.Stacks["testing-1"].Config))
	b, ok := pkg.Stacks["testing-1"].Config[cfgkey("configB")]
	assert.True(t, ok)
	bv, err := b.Value(nil)
	assert.NoError(t, err)
	assert.Equal(t, "value2", bv)
	e2, ok := pkg.Stacks["testing-1"].Config[cfgkey("configE")]
	assert.True(t, ok)
	e2v, err := e2.Value(nil)
	assert.NoError(t, err)
	assert.Equal(t, "value55", e2v)
	assert.Equal(t, 1, len(pkg.Stacks["testing-2"].Config))
	c, ok := pkg.Stacks["testing-2"].Config[cfgkey("configC")]
	assert.True(t, ok)
	cv, err := c.Value(nil)
	assert.NoError(t, err)
	assert.Equal(t, "value3", cv)
}
