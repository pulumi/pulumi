// Copyright 2026, Pulumi Corporation.
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

package catalog

import (
	"fmt"
	"sort"
)

type Language struct {
	ID          string
	DisplayName string
}

type Provider struct {
	ID          string
	DisplayName string
	Featured    bool
	Languages   []Language
}

var languageDisplayNames = map[string]string{
	"bun":         "Bun",
	"csharp":      "C#",
	"fsharp":      "F#",
	"go":          "Go",
	"java":        "Java",
	"java-gradle": "Java (Gradle)",
	"javascript":  "JavaScript",
	"python":      "Python",
	"scala":       "Scala",
	"typescript":  "TypeScript",
	"visualbasic": "Visual Basic",
	"yaml":        "YAML",
}

type providerDef struct {
	displayName string
	// namePrefix is the template name's provider segment; empty means the language id is the
	// whole template name (the bare, cloudless templates).
	namePrefix       string
	featured         bool
	languages        []string
	displayOverrides map[string]string
}

var providerDefs = map[string]providerDef{
	"aws": {displayName: "AWS", namePrefix: "aws", featured: true, languages: []string{
		"typescript", "python", "bun", "csharp", "fsharp", "go", "java", "scala", "visualbasic", "yaml",
	}},
	"azure": {displayName: "Azure", namePrefix: "azure", featured: true, languages: []string{
		"typescript", "python", "csharp", "fsharp", "go", "java", "yaml",
	}},
	"gcp": {displayName: "GCP", namePrefix: "gcp", featured: true, languages: []string{
		"typescript", "python", "csharp", "fsharp", "go", "java", "visualbasic", "yaml",
	}},
	// The bare Java templates split by build system and Gradle outpolls Maven, so unlike the
	// cloud providers (one Java template each) plain "Java" would silently pick a build system.
	// "none" is not a cloud provider; it's featured so this non-cloud pseudo-provider is promoted
	// into the primary cloud prompt alongside AWS/Azure/GCP.
	"none": {displayName: "None", namePrefix: "", featured: true, languages: []string{
		"typescript", "python", "go", "csharp", "fsharp", "java", "java-gradle",
		"javascript", "bun", "visualbasic", "yaml",
	}, displayOverrides: map[string]string{"java": "Java (Maven)"}},
	"alicloud": {displayName: "Alibaba Cloud", namePrefix: "alicloud", languages: []string{
		"typescript", "python", "csharp", "fsharp", "go", "visualbasic", "yaml",
	}},
	"aiven": {displayName: "Aiven", namePrefix: "aiven", languages: []string{"typescript", "python", "go"}},
	"auth0": {displayName: "Auth0", namePrefix: "auth0", languages: []string{
		"typescript", "python", "csharp", "go", "yaml",
	}},
	"azuredevops": {displayName: "Azure DevOps", namePrefix: "azuredevops", languages: []string{"python"}},
	"digitalocean": {displayName: "DigitalOcean", namePrefix: "digitalocean", languages: []string{
		"typescript", "python", "go", "yaml",
	}},
	"github": {displayName: "GitHub", namePrefix: "github", languages: []string{
		"typescript", "python", "csharp", "go", "yaml",
	}},
	"kubernetes": {displayName: "Kubernetes", namePrefix: "kubernetes", languages: []string{
		"typescript", "python", "csharp", "fsharp", "go", "java", "yaml",
	}},
	"linode": {displayName: "Linode", namePrefix: "linode", languages: []string{
		"typescript", "python", "go", "yaml",
	}},
	"oci": {displayName: "Oracle Cloud", namePrefix: "oci", languages: []string{
		"typescript", "python", "go", "java", "yaml",
	}},
	"ovh": {displayName: "OVH", namePrefix: "ovh", languages: []string{
		"typescript", "python", "csharp", "go", "java",
	}},
	"pinecone": {displayName: "Pinecone", namePrefix: "pinecone", languages: []string{
		"typescript", "python", "csharp", "go", "yaml",
	}},
	"random": {displayName: "Random", namePrefix: "random", languages: []string{
		"typescript", "python", "csharp", "go", "java", "yaml",
	}},
	"rediscloud": {displayName: "Redis Cloud", namePrefix: "rediscloud", languages: []string{"python", "go"}},
}

var featuredOrder = []string{"aws", "azure", "gcp", "none"}

// languageRank orders the language prompt by observed `pulumi new` usage share, most-used first,
// so the common choices sit at the top. Ids absent here sort last, alphabetically.
var languageRank = map[string]int{
	"typescript":  0,
	"python":      1,
	"go":          2,
	"csharp":      3,
	"yaml":        4,
	"java":        5,
	"java-gradle": 6,
	"javascript":  7,
	"bun":         8,
	"fsharp":      9,
	"scala":       10,
	"visualbasic": 11,
}

func languageOrder(id string) int {
	if r, ok := languageRank[id]; ok {
		return r
	}
	return len(languageRank)
}

func buildLanguages(ids []string, displayOverrides map[string]string) []Language {
	langs := make([]Language, 0, len(ids))
	for _, id := range ids {
		display, ok := displayOverrides[id]
		if !ok {
			display = languageDisplayNames[id]
		}
		langs = append(langs, Language{ID: id, DisplayName: display})
	}
	sort.SliceStable(langs, func(i, j int) bool {
		ri, rj := languageOrder(langs[i].ID), languageOrder(langs[j].ID)
		if ri != rj {
			return ri < rj
		}
		return langs[i].DisplayName < langs[j].DisplayName
	})
	return langs
}

func Get(id string) (Provider, bool) {
	def, ok := providerDefs[id]
	if !ok {
		return Provider{}, false
	}
	return Provider{
		ID:          id,
		DisplayName: def.displayName,
		Featured:    def.featured,
		Languages:   buildLanguages(def.languages, def.displayOverrides),
	}, true
}

func Featured() []Provider {
	providers := make([]Provider, 0, len(featuredOrder))
	for _, id := range featuredOrder {
		p, ok := Get(id)
		if ok {
			providers = append(providers, p)
		}
	}
	return providers
}

func Others() []Provider {
	providers := make([]Provider, 0, len(providerDefs))
	for id, def := range providerDefs {
		if def.featured {
			continue
		}
		p, _ := Get(id)
		providers = append(providers, p)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].DisplayName < providers[j].DisplayName })
	return providers
}

func Resolve(providerID, languageID string) (string, bool) {
	def, ok := providerDefs[providerID]
	if !ok {
		return "", false
	}
	for _, id := range def.languages {
		if id != languageID {
			continue
		}
		if def.namePrefix == "" {
			return languageID, true
		}
		return fmt.Sprintf("%s-%s", def.namePrefix, languageID), true
	}
	return "", false
}
