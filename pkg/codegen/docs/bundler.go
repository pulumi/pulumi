// Copyright 2016-2021, Pulumi Corporation.
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

package docs

import (
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
)

const (
	basePath          = "."
	docsTemplatesPath = basePath + "/templates"
)

// generateTemplatesBundle reads the templates from `../templates/` and returns a map
// containing byte-slices of the templates.
func generateTemplatesBundle() (map[string][]byte, error) {
	files, err := ioutil.ReadDir(docsTemplatesPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading the templates dir")
	}

	contents := make(map[string][]byte)
	for _, f := range files {
		if f.IsDir() {
			fmt.Printf("%q is a dir. Skipping...\n", f.Name())
		}
		b, err := ioutil.ReadFile(docsTemplatesPath + "/" + f.Name())
		if err != nil {
			return nil, errors.Wrapf(err, "reading file %s", f.Name())
		}
		if len(b) == 0 {
			fmt.Printf("%q is empty. Skipping...\n", f.Name())
			continue
		}
		contents[f.Name()] = b
	}

	return contents, nil
}
