// Copyright 2024, Pulumi Corporation.
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

package main

import (
	"context"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-github/v55/github"
)

// This looks up all the repositories in the "pulumiverse" organization and generates a list of plugin names
// based on those repositories that start with "pulumi-". This is then written to "pulumiverse_list.go".
func main() {
	ctx := context.Background()
	client := github.NewClient(nil)

	// Query all repositories in the "pulumiverse" organization
	org := "pulumiverse"
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 50},
	}

	var allRepos []*github.Repository
	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			log.Fatalf("Error fetching repositories: %v", err)
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	names := []string{}
	for _, repo := range allRepos {
		n := repo.GetName()
		if strings.HasPrefix(n, "pulumi-") {
			names = append(names, strings.TrimPrefix(n, "pulumi-"))
		}
	}
	slices.Sort(names)

	templates, err := template.New("templates").ParseGlob("./generate/templates/*")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("code generation failed: %v", err)
	}

	template := templates.Lookup("pulumiverse_list.go.template")

	fullname := filepath.Join(pwd, "pulumiverse_list.go")
	f, err := os.Create(fullname)
	if err != nil {
		log.Fatalf("failed to create %v: %v", fullname, err)
	}
	data := map[string]interface{}{
		"Names": names,
	}
	if err := template.Execute(f, data); err != nil {
		log.Fatalf("failed to execute %v: %v", template.Name(), err)
	}
	f.Close()

	gofmt := exec.Command("gofmt", "-s", "-w", fullname)
	gofmt.Stdout = os.Stdout
	gofmt.Stderr = os.Stderr
	if err := gofmt.Run(); err != nil {
		log.Fatalf("failed to run gofmt on %v: %v", fullname, err)
	}
}
