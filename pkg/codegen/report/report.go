// Copyright 2016-2022, Pulumi Corporation.
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

package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/hcl/v2"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	hcl2 "github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

var ExportTargetDir = env.String("CODEGEN_REPORT_DIR",
	"The directory to generate a codegen report in")

type GenerateProgramFn func(*hcl2.Program) (map[string][]byte, hcl.Diagnostics, error)

type Reporter interface {
	io.Closer
	// Report a call to GenerateProgram.
	Report(title, language string, files []*syntax.File, diags hcl.Diagnostics, err error)
	Summary() Summary
}

func New(name, version string) Reporter {
	return &reporter{
		data: Summary{
			Name:    name,
			Version: version,
		},
	}
}

type Summary struct {
	Stats
	Name          string               `json:"name"`
	Version       string               `json:"version"`
	ReportVersion string               `json:"reportVersion"`
	Languages     map[string]*Language `json:"languages"`
}

type Stats struct {
	NumConversions int
	Successes      int
}

type Language struct {
	Stats

	// A mapping from Error:(title:occurrences)
	Warnings map[string]map[string]int `json:"warning,omitempty"`
	Errors   map[string]map[string]int `json:"errors,omitempty"`

	// A mapping from between titles and Go errors (as opposed to diag errors)
	GoErrors map[string]string `json:"goerrors,omitempty"`

	// A mapping from title:files
	Files map[string][]File `json:"files,omitempty"`
}

type File struct {
	Name string `json:"name,omitempty"`
	Body string `json:"body,omitempty"`
}

type reporter struct {
	data     Summary
	reported bool
	m        sync.Mutex
}

func (s *Stats) update(succeed bool) {
	s.NumConversions++
	if succeed {
		s.Successes++
	}
}

func (r *reporter) getLanguage(lang string) *Language {
	if r.data.Languages == nil {
		r.data.Languages = map[string]*Language{}
	}
	l, ok := r.data.Languages[lang]
	if !ok {
		l = new(Language)
		r.data.Languages[lang] = l
	}
	return l
}

func WrapGen(reporter Reporter, title, language string, files []*syntax.File, f GenerateProgramFn) GenerateProgramFn {
	return func(p *hcl2.Program) (m map[string][]byte, diags hcl.Diagnostics, err error) {
		defer func() {
			reporter.Report(title, language, files, diags, err)
		}()
		m, diags, err = f(p)
		return m, diags, err
	}
}

func (r *reporter) Report(title, language string, files []*syntax.File, diags hcl.Diagnostics, err error) {
	r.m.Lock()
	defer r.m.Unlock()
	if panicVal := recover(); panicVal != nil {
		if panicErr, ok := panicVal.(error); ok {
			err = fmt.Errorf("panic: %w", panicErr)
		} else {
			err = fmt.Errorf("panic: %v", panicVal)
		}
	}
	failed := diags.HasErrors() || err != nil
	r.data.Stats.update(!failed)
	lang := r.getLanguage(language)
	lang.Stats.update(!failed)

	if failed {
		var txts []File
		for _, file := range files {
			txts = append(txts, File{
				Name: file.Name,
				Body: string(file.Bytes),
			})
		}
		if lang.Files == nil {
			lang.Files = map[string][]File{}
		}
		lang.Files[title] = txts
	}
	if err != nil {
		err := fmt.Sprintf("error: %v", err)
		if lang.GoErrors == nil {
			lang.GoErrors = map[string]string{}
		}
		lang.GoErrors[title] = err
	}

	incr := func(m *map[string]map[string]int, key string) {
		if (*m) == nil {
			*m = map[string]map[string]int{}
		}
		if (*m)[key] == nil {
			(*m)[key] = map[string]int{}
		}
		(*m)[key][title]++
	}

	for _, diag := range diags {
		switch diag.Severity {
		case hcl.DiagError:
			incr(&lang.Errors, diag.Error())
		case hcl.DiagWarning:
			incr(&lang.Warnings, diag.Error())
		case hcl.DiagInvalid:
			msg := fmt.Sprintf("invalid diag: %v", diag)
			incr(&lang.Errors, msg)
		}
	}
}

// Fetch the summary to report on.
//
// Calling this function disables automatic reporting.
func (r *reporter) Summary() Summary {
	if r == nil {
		return Summary{ReportVersion: version.Version}
	}
	r.m.Lock()
	defer r.m.Unlock()
	r.reported = true
	return r.summary()
}

func (r *reporter) summary() Summary {
	r.data.ReportVersion = version.Version
	return r.data
}

// If an env var is set to specify where we should write our results to, and if no other
// program has looked at our results, we write out our results to a file.
func (r *reporter) Close() error {
	return r.DefaultExport()
}

// Run the default export behavior on the current report.
func (r *reporter) DefaultExport() error {
	r.m.Lock()
	defer r.m.Unlock()
	dir, ok := ExportTargetDir.Underlying()
	if !ok || r.reported {
		return nil
	}
	r.reported = true
	return r.defaultExport(dir)
}

func (r *reporter) defaultExport(dir string) error {
	if dir == "" {
		err := fmt.Errorf("%q set to the empty string", ExportTargetDir.Var().Name())
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}

	if info, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0o700)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !info.IsDir() {
		err := fmt.Errorf("expected %q to be a directory or empty, found a file", dir)
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}

	name := fmt.Sprintf("%s-%s.json", r.data.Name, time.Now().Format("2006-01-02-15:04:05"))
	path := filepath.Join(dir, name)
	data, err := json.MarshalIndent(r.summary(), "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
