// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/stretchr/testify/assert"
)

func TestNoRootNoMain(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	context, main, err := getContextAndMain(&pack.Package{}, dir)
	assert.NoError(t, err)
	assert.Equal(t, dir, context)
	assert.Equal(t, "", main)
}

func TestNoRootMain(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	testPkg := pack.Package{Main: "foo/bar/baz/"}

	context, main, err := getContextAndMain(&testPkg, dir)
	assert.NoError(t, err)
	assert.Equal(t, dir, context)
	assert.Equal(t, testPkg.Main, main)
}

func TestRootNoMain(t *testing.T) {
	dir, _ := ioutil.TempDir("", "archive-test")
	sub := filepath.Join(dir, "sub1", "sub2", "sub3")
	defer func() {
		contract.IgnoreError(os.RemoveAll(dir))
	}()

	err := os.MkdirAll(sub, 0700)
	assert.NoError(t, err, "error creating test directory")

	testPkg := pack.Package{
		Context: "../../../",
	}

	context, main, err := getContextAndMain(&testPkg, sub)
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

	testPkg := pack.Package{
		Context: "../../../",
		Main:    "sub4/",
	}

	context, main, err := getContextAndMain(&testPkg, filepath.Dir(sub))
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

	testPkg := pack.Package{
		Context: bad,
	}

	_, _, err := getContextAndMain(&testPkg, dir)

	assert.Error(t, err)
}
