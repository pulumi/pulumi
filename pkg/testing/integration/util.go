// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package integration

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// ReplaceInFile does a find and replace for a given string within a file.
func ReplaceInFile(old, new, path string) error {
	rawContents, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	newContents := strings.Replace(string(rawContents), old, new, -1)
	return ioutil.WriteFile(path, []byte(newContents), os.ModePerm)
}

// getCmdBin returns the binary named bin in location loc or, if it hasn't yet been initialized, will lazily
// populate it by either using the default def or, if empty, looking on the current $PATH.
func getCmdBin(loc *string, bin, def string) (string, error) {
	if *loc == "" {
		*loc = def
		if *loc == "" {
			var err error
			*loc, err = exec.LookPath(bin)
			if err != nil {
				return "", errors.Wrapf(err, "Expected to find `%s` binary on $PATH", bin)
			}
		}
	}
	return *loc, nil
}

func uniqueSuffix() string {
	// .<timestamp>.<five random hex characters>
	timestamp := time.Now().Format("20060102-150405")
	suffix, err := resource.NewUniqueHex("."+timestamp+".", 5, -1)
	contract.AssertNoError(err)
	return suffix
}

func writeCommandOutput(commandName, runDir string, output []byte) (string, error) {
	logFileDir := filepath.Join(runDir, "command-output")
	if err := os.MkdirAll(logFileDir, 0700); err != nil {
		return "", errors.Wrapf(err, "Failed to create '%s'", logFileDir)
	}

	logFile := filepath.Join(logFileDir, commandName+uniqueSuffix()+".log")

	if err := ioutil.WriteFile(logFile, output, 0644); err != nil {
		return "", errors.Wrapf(err, "Failed to write '%s'", logFile)
	}

	return logFile, nil
}

type prefixer struct {
	writer    io.Writer
	prefix    []byte
	anyOutput bool
}

// newPrefixer wraps an io.Writer, prepending a fixed prefix after each \n emitting on the wrapped writer
func newPrefixer(writer io.Writer, prefix string) *prefixer {
	return &prefixer{writer, []byte(prefix), false}
}

var _ io.Writer = (*prefixer)(nil)

func (prefixer *prefixer) Write(p []byte) (int, error) {
	n := 0
	lines := bytes.SplitAfter(p, []byte{'\n'})
	for _, line := range lines {
		if len(line) > 0 {
			_, err := prefixer.writer.Write(prefixer.prefix)
			if err != nil {
				return n, err
			}
		}
		m, err := prefixer.writer.Write(line)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
