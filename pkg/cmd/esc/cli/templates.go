// Copyright 2023, Pulumi Corporation.
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

package cli

import (
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates
var templatesFS embed.FS

var (
	envGetTemplate  *template.Template
	envDiffTemplate *template.Template
)

func init() {
	mustLookup := func(t *template.Template, name string) *template.Template {
		tt := t.Lookup(name)
		if tt == nil {
			panic(fmt.Errorf("missing template %v", name))
		}
		return tt
	}

	root := template.New("templates")

	templates := template.Must(root.ParseFS(templatesFS, "templates/*.tmpl"))
	envGetTemplate = mustLookup(templates, "env-get.tmpl")
	envDiffTemplate = mustLookup(templates, "env-diff.tmpl")
}
