// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

func TestNoRootNoMain(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	context, main, err := getContextAndMain(&workspace.Project{}, dir)
	assert.NoError(t, err)
	assert.Equal(t, dir, context)
	assert.Equal(t, "", main)
}

func TestNoRootMain(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	testProj := workspace.Project{Main: "foo/bar/baz/"}

	context, main, err := getContextAndMain(&testProj, dir)
	assert.NoError(t, err)
	assert.Equal(t, dir, context)
	assert.Equal(t, testProj.Main, main)
}

func TestRootNoMain(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	sub := filepath.Join(dir, "sub1", "sub2", "sub3")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	err := os.MkdirAll(sub, 0700)
	assert.NoError(t, err, "error creating test directory")

	testProj := workspace.Project{
		Context: "../../../",
	}

	context, main, err := getContextAndMain(&testProj, sub)
	assert.NoError(t, err)
	assert.Equal(t, dir, context)
	assert.Equal(t, "sub1/sub2/sub3/", main)
}

func TestRootMain(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	sub := filepath.Join(dir, "sub1", "sub2", "sub3", "sub4")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	err := os.MkdirAll(sub, 0700)
	assert.NoError(t, err, "error creating test directory")

	testProj := workspace.Project{
		Context: "../../../",
		Main:    "sub4/",
	}

	context, main, err := getContextAndMain(&testProj, filepath.Dir(sub))
	assert.NoError(t, err)
	assert.Equal(t, dir, context)
	assert.Equal(t, "sub1/sub2/sub3/sub4/", main)
}

func TestBadContext(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	bad, _ := ioutil.TempDir("", "archive-test")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
		contract.IgnoreError(os.RemoveAll(bad))
	}()

	testProj := workspace.Project{
		Context: bad,
	}

	_, _, err := getContextAndMain(&testProj, dir)

	assert.Error(t, err)
}
