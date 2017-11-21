// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package archive

import (
	"io/ioutil"
	"path"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// newPulumiIgnorerIgnorer creates an ignorer based on the contents of a .pulumiignore file, which
// has the same semantics as a .gitignore file
func newPulumiIgnorerIgnorer(pathToPulumiIgnore string) (ignorer, error) {
	gitIgnorer, err := readIgnoreFile(pathToPulumiIgnore)
	if err != nil {
		return nil, err
	}

	return &pulumiIgnoreIgnorer{root: path.Dir(pathToPulumiIgnore), ignorer: gitIgnorer}, nil
}

func readIgnoreFile(path string) (*ignore.GitIgnore, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	patterns := []string{".git/"}
	patterns = append(patterns, strings.Split(string(buf), "\n")...)

	return ignore.CompileIgnoreLines(patterns...)
}

type pulumiIgnoreIgnorer struct {
	root    string
	ignorer *ignore.GitIgnore
}

func (g *pulumiIgnoreIgnorer) IsIgnored(f string) bool {
	return g.ignorer.MatchesPath(strings.TrimPrefix(f, g.root))
}
