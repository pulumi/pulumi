// Copyright 2017-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package workspace

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
)

type Repository struct {
	Owner string `json:"owner" yaml:"owner"` // the owner of this repository
	Name  string `json:"name" yaml:"name"`   // the name of the repository
	Root  string `json:"-" yaml:"-"`         // storage location
}

func (r *Repository) Save() error {
	b, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return err
	}

	// nolint: gas
	err = os.MkdirAll(r.Root, 0755)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(r.Root, RepoFile), b, 0644)
}

func NewRepository(root string) *Repository {
	return &Repository{Root: getDotPulumiDirectoryPath(root)}
}

var ErrNoRepository = errors.New("no repository detected; did you forget to run 'pulumi init'?")

func GetRepository(root string) (*Repository, error) {
	dotPulumiPath := getDotPulumiDirectoryPath(root)

	repofilePath := filepath.Join(dotPulumiPath, RepoFile)

	b, err := ioutil.ReadFile(repofilePath)
	if os.IsNotExist(err) {
		return nil, ErrNoRepository
	} else if err != nil {
		return nil, err
	}

	var repo Repository
	err = json.Unmarshal(b, &repo)
	if err != nil {
		return nil, err
	}

	if repo.Owner == "" {
		return nil, errors.New("invalid repo.json file, missing name property")
	}

	if repo.Name == "" {
		return nil, errors.New("invalid repo.json file, missing name property")
	}

	repo.Root = dotPulumiPath

	return &repo, nil
}

func getDotPulumiDirectoryPath(dir string) string {
	// First, let's look to see if there's an existing .pulumi folder
	dotpulumipath, _ := fsutil.WalkUp(dir, isRepositoryFolder, nil)
	if dotpulumipath != "" {
		return dotpulumipath
	}

	// If there's a .git folder, put .pulumi there
	dotgitpath, _ := fsutil.WalkUp(dir, isGitFolder, nil)
	if dotgitpath != "" {
		return filepath.Join(filepath.Dir(dotgitpath), ".pulumi")
	}

	return filepath.Join(dir, ".pulumi")
}
