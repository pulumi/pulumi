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
// structure the guided `pulumi new` flow walks through. The structure (which providers exist and
// which languages each supports) is derived from the live template names, so new upstream templates
// appear automatically; only the presentation (display names, featured set, ordering) is curated.
package catalog

import (
	"sort"
	"strings"
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

// noneProvider is the pseudo-provider for the bare, cloudless templates whose name is just a
// language id (e.g. "typescript", "java-gradle").
const noneProvider = "none"

// languageDisplayNames is the set of known language ids and their display names. Its keys double as
// the vocabulary used to split "<provider>-<language>" template names, so an unrecognized language
// suffix leaves a template unparsed (and thus out of the guided flow, still reachable via the flat list).
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

// languageDisplayOverrides names a language differently under a specific provider. The bare Java
// templates split by build system, so "None" disambiguates the plain "java" template as Maven while
// every cloud provider (one Java template each) keeps the plain "Java".
var languageDisplayOverrides = map[string]map[string]string{
	noneProvider: {"java": "Java (Maven)"},
}

// providerDisplayNames curates the human labels. Providers absent here still appear (using their raw
// id) rather than being hidden, so a newly published provider is reachable before anyone curates it.
var providerDisplayNames = map[string]string{
	"aws":          "AWS",
	"azure":        "Azure",
	"gcp":          "GCP",
	noneProvider:   "None",
	"alicloud":     "Alibaba Cloud",
	"aiven":        "Aiven",
	"auth0":        "Auth0",
	"azuredevops":  "Azure DevOps",
	"digitalocean": "DigitalOcean",
	"github":       "GitHub",
	"kubernetes":   "Kubernetes",
	"linode":       "Linode",
	"oci":          "Oracle Cloud",
	"ovh":          "OVH",
	"pinecone":     "Pinecone",
	"random":       "Random",
	"rediscloud":   "Redis Cloud",
}

// featuredOrder promotes these providers into the primary cloud prompt, in this order. "None" is not
// a cloud but is featured so the cloudless starters sit alongside AWS/Azure/GCP.
var featuredOrder = []string{"aws", "azure", "gcp", noneProvider}

// languageRank orders the language prompt by observed `pulumi new` usage share, most-used first.
// Ids absent here sort last, alphabetically.
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

// Catalog is the provider/language structure derived from a set of template names.
type Catalog struct {
	providers map[string]Provider
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
		if names[providerID] == nil {
			names[providerID] = map[string]string{}
		}
		names[providerID][languageID] = name
	}

	providers := make(map[string]Provider, len(names))
	for providerID, byLanguage := range names {
		languageIDs := make([]string, 0, len(byLanguage))
		for languageID := range byLanguage {
			languageIDs = append(languageIDs, languageID)
		}
		providers[providerID] = Provider{
			ID:          providerID,
			DisplayName: providerDisplayName(providerID),
			Featured:    featuredRank(providerID) >= 0,
			Languages:   buildLanguages(providerID, languageIDs),
		}
	}
	return &Catalog{providers: providers, templateNames: names}
}

// Empty reports whether no provider could be derived, in which case the caller should fall back to
// the flat template list.
func (c *Catalog) Empty() bool {
	return len(c.providers) == 0
}

func (c *Catalog) Get(id string) (Provider, bool) {
	p, ok := c.providers[id]
	return p, ok
}

func (c *Catalog) Featured() []Provider {
	providers := make([]Provider, 0, len(featuredOrder))
	for _, id := range featuredOrder {
		if p, ok := c.providers[id]; ok {
			providers = append(providers, p)
		}
	}
	return providers
}

func (c *Catalog) Others() []Provider {
	providers := make([]Provider, 0, len(c.providers))
	for _, p := range c.providers {
		if !p.Featured {
			providers = append(providers, p)
		}
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].DisplayName < providers[j].DisplayName })
	return providers
}

// Resolve returns the template name backing a provider/language pair.
func (c *Catalog) Resolve(providerID, languageID string) (string, bool) {
	name, ok := c.templateNames[providerID][languageID]
	return name, ok
}

// splitTemplateName decomposes a template name into its provider and language. A name that is exactly
// a known language id is a bare (None) template; otherwise the longest known language suffix wins,
// which keeps "java-gradle" whole instead of reading it as provider "java".
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

func buildLanguages(providerID string, languageIDs []string) []Language {
	langs := make([]Language, 0, len(languageIDs))
	for _, id := range languageIDs {
		langs = append(langs, Language{ID: id, DisplayName: languageDisplayName(providerID, id)})
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

func providerDisplayName(id string) string {
	if name, ok := providerDisplayNames[id]; ok {
		return name
	}
	return id
}

func languageDisplayName(providerID, languageID string) string {
	if override, ok := languageDisplayOverrides[providerID][languageID]; ok {
		return override
	}
	if name, ok := languageDisplayNames[languageID]; ok {
		return name
	}
	return languageID
}

func languageOrder(id string) int {
	if r, ok := languageRank[id]; ok {
		return r
	}
	return len(languageRank)
}

func featuredRank(id string) int {
	for i, featured := range featuredOrder {
		if featured == id {
			return i
		}
	}
	return -1
}
