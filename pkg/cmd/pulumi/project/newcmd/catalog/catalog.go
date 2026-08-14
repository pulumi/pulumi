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
// provider and language stays out of the catalog entirely.
package catalog

import (
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

// noneProvider is the pseudo-provider for templates whose name is just a language id.
const noneProvider = "none"

type vocab struct {
	id, displayName string
}

var languages = []vocab{
	{"typescript", "TypeScript"},
	{"python", "Python"},
	{"go", "Go"},
	{"csharp", "C#"},
	{"yaml", "YAML"},
	{"hcl", "HCL"},
	{"java", "Java"},
	{"java-gradle", "Java (Gradle)"},
	{"javascript", "JavaScript"},
	{"bun", "Bun"},
	{"fsharp", "F#"},
	{"scala", "Scala"},
	{"visualbasic", "Visual Basic"},
}

// The cloudless row sits last so the curated clouds stay adjacent.
var providers = []vocab{
	{"aws", "AWS"},
	{"azure", "Azure"},
	{"gcp", "Google Cloud"},
	{noneProvider, "Basic Pulumi Program"},
}

var (
	languageDisplayNames = displayNames(languages)
	providerDisplayNames = displayNames(providers)
)

func displayNames(entries []vocab) map[string]string {
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.id] = e.displayName
	}
	return m
}

type Catalog[T any] struct {
	// templates maps providerID -> languageID -> the template whose name produced the pair.
	templates map[string]map[string]T
}

func New[T any](templates []T, name func(T) string) *Catalog[T] {
	byProvider := map[string]map[string]T{}
	for _, template := range templates {
		providerID, languageID, ok := splitTemplateName(name(template))
		if !ok {
			continue
		}
		if _, curated := providerDisplayNames[providerID]; !curated {
			continue
		}
		if byProvider[providerID] == nil {
			byProvider[providerID] = map[string]T{}
		}
		byProvider[providerID][languageID] = template
	}
	return &Catalog[T]{templates: byProvider}
}

func (c *Catalog[T]) Providers() []Provider {
	found := make([]Provider, 0, len(providers))
	for _, p := range providers {
		if _, ok := c.templates[p.id]; ok {
			found = append(found, Provider{
				ID:          p.id,
				DisplayName: p.displayName,
				Languages:   buildLanguages(p.id, c.templates[p.id]),
			})
		}
	}
	return found
}

func (c *Catalog[T]) Resolve(providerID, languageID string) (T, bool) {
	template, ok := c.templates[providerID][languageID]
	return template, ok
}

// The longest known language suffix wins, which keeps "java-gradle" whole instead of reading it as
// provider "java".
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

func buildLanguages[T any](providerID string, byLanguage map[string]T) []Language {
	langs := make([]Language, 0, len(byLanguage))
	for _, l := range languages {
		if _, ok := byLanguage[l.id]; ok {
			langs = append(langs, Language{ID: l.id, DisplayName: languageDisplayName(providerID, l.id)})
		}
	}
	return langs
}

func languageDisplayName(providerID, languageID string) string {
	// The bare Java templates split by build system, so under the cloudless provider the plain
	// "java" template is disambiguated as Maven.
	if providerID == noneProvider && languageID == "java" {
		return "Java (Maven)"
	}
	if name, ok := languageDisplayNames[languageID]; ok {
		return name
	}
	return languageID
}
