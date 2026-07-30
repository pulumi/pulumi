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

// Package catalog turns the flat list of available template names into the provider/language
// structure the guided `pulumi new` flow walks through. A name that doesn't decompose into a known
// provider and language stays out of the guided flow and remains reachable through the "Browse all
// templates" fallback.
package catalog

import (
	"slices"
	"strings"
)

type Language struct {
	ID          string
	DisplayName string
}

type Provider struct {
	ID          string
	DisplayName string
	Languages   []Language
}

// noneProvider is the pseudo-provider for the bare, cloudless templates whose name is just a
// language id (e.g. "typescript", "java-gradle").
const noneProvider = "none"

type vocab struct {
	id, displayName string
}

// languages is the closed vocabulary of languages the guided flow recognizes, ordered by observed
// `pulumi new` usage share, most-used first.
var languages = []vocab{
	{"typescript", "TypeScript"},
	{"python", "Python"},
	{"go", "Go"},
	{"csharp", "C#"},
	{"yaml", "YAML"},
	{"java", "Java"},
	{"java-gradle", "Java (Gradle)"},
	{"javascript", "JavaScript"},
	{"bun", "Bun"},
	{"fsharp", "F#"},
	{"scala", "Scala"},
	{"visualbasic", "Visual Basic"},
	{"hcl", "HCL"},
}

// featuredProviders are promoted into the primary cloud prompt, in this order. Together with
// otherProviders and the None pseudo-provider they are the closed set of providers the guided
// flow offers.
var featuredProviders = []vocab{
	{"aws", "AWS"},
	{"azure", "Azure"},
	{"gcp", "GCP"},
}

var none = vocab{noneProvider, "None"}

// otherProviders appear under "Other" in this order (alphabetical by display name).
var otherProviders = []vocab{
	{"aiven", "Aiven"},
	{"alicloud", "Alibaba Cloud"},
	{"auth0", "Auth0"},
	{"azuredevops", "Azure DevOps"},
	{"digitalocean", "DigitalOcean"},
	{"github", "GitHub"},
	{"kubernetes", "Kubernetes"},
	{"linode", "Linode"},
	{"ovh", "OVH"},
	{"oci", "Oracle Cloud"},
	{"pinecone", "Pinecone"},
	{"random", "Random"},
	{"rediscloud", "Redis Cloud"},
}

var (
	languageDisplayNames = displayNames(languages)
	providerDisplayNames = displayNames(slices.Concat(featuredProviders, otherProviders, []vocab{none}))
)

func displayNames(entries []vocab) map[string]string {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.id] = e.displayName
	}
	return m
}

type Catalog struct {
	// templateNames maps providerID -> languageID -> the template name that produced it.
	templateNames map[string]map[string]string
}

// New derives a catalog from the available template names.
func New(templateNames []string) *Catalog {
	names := map[string]map[string]string{}
	for _, name := range templateNames {
		providerID, languageID, ok := splitTemplateName(name)
		if !ok {
			continue
		}
		if _, curated := providerDisplayNames[providerID]; !curated {
			continue
		}
		if names[providerID] == nil {
			names[providerID] = map[string]string{}
		}
		names[providerID][languageID] = name
	}
	return &Catalog{templateNames: names}
}

func (c *Catalog) Empty() bool {
	return len(c.templateNames) == 0
}

func (c *Catalog) provider(id string) Provider {
	return Provider{
		ID:          id,
		DisplayName: providerDisplayNames[id],
		Languages:   buildLanguages(id, c.templateNames[id]),
	}
}

func (c *Catalog) Featured() []Provider {
	providers := make([]Provider, 0, len(featuredProviders))
	for _, p := range featuredProviders {
		if _, ok := c.templateNames[p.id]; ok {
			providers = append(providers, c.provider(p.id))
		}
	}
	return providers
}

// None returns the pseudo-provider for bare, cloudless templates, if any are available.
func (c *Catalog) None() (Provider, bool) {
	if _, ok := c.templateNames[noneProvider]; !ok {
		return Provider{}, false
	}
	return c.provider(noneProvider), true
}

func (c *Catalog) Others() []Provider {
	providers := make([]Provider, 0, len(c.templateNames))
	for _, p := range otherProviders {
		if _, ok := c.templateNames[p.id]; ok {
			providers = append(providers, c.provider(p.id))
		}
	}
	return providers
}

func (c *Catalog) Resolve(providerID, languageID string) (string, bool) {
	name, ok := c.templateNames[providerID][languageID]
	return name, ok
}

// splitTemplateName decomposes a template name into its provider and language. The longest known
// language suffix wins, which keeps "java-gradle" whole instead of reading it as provider "java".
func splitTemplateName(name string) (providerID, languageID string, ok bool) {
	if _, isLanguage := languageDisplayNames[name]; isLanguage {
		return noneProvider, name, true
	}
	best := ""
	for languageID := range languageDisplayNames {
		suffix := "-" + languageID
		if len(name) > len(suffix) && strings.HasSuffix(name, suffix) && len(languageID) > len(best) {
			best = languageID
		}
	}
	if best == "" {
		return "", "", false
	}
	return name[:len(name)-len(best)-1], best, true
}

func buildLanguages(providerID string, byLanguage map[string]string) []Language {
	langs := make([]Language, 0, len(byLanguage))
	for _, l := range languages {
		if _, ok := byLanguage[l.id]; ok {
			langs = append(langs, Language{ID: l.id, DisplayName: languageDisplayName(providerID, l.id)})
		}
	}
	return langs
}

func languageDisplayName(providerID, languageID string) string {
	// The bare Java templates split by build system, so under "None" the plain "java" template is
	// disambiguated as Maven.
	if providerID == noneProvider && languageID == "java" {
		return "Java (Maven)"
	}
	if name, ok := languageDisplayNames[languageID]; ok {
		return name
	}
	return languageID
}
