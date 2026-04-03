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

package docs

import (
	"regexp"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/docsrender"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

var (
	chooserOpenRe  = regexp.MustCompile(`<!--\s*chooser:\s*(\w+)\s*-->`)
	optionOpenRe   = regexp.MustCompile(`<!--\s*option:\s*(\w+)\s*-->`)
	chooserCloseRe = regexp.MustCompile(`<!--\s*/chooser\s*-->`)
)

// chooserInfo describes a chooser found in the document, with its type and available options.
type chooserInfo struct {
	chooserType string
	options     []string
}

// scanChoosers scans raw markdown text for chooser blocks and returns
// their types and available option values.
func scanChoosers(text string) []chooserInfo {
	seen := map[string]bool{}
	var result []chooserInfo
	pos := 0

	for pos < len(text) {
		loc := chooserOpenRe.FindStringIndex(text[pos:])
		if loc == nil {
			break
		}
		absStart := pos + loc[0]
		absEnd := pos + loc[1]

		m := chooserOpenRe.FindStringSubmatch(text[absStart:absEnd])
		chooserType := m[1]

		closeLoc := chooserCloseRe.FindStringIndex(text[absEnd:])
		if closeLoc == nil {
			pos = absEnd
			continue
		}

		chooserBody := text[absEnd : absEnd+closeLoc[0]]
		pos = absEnd + closeLoc[1]

		if seen[chooserType] {
			continue
		}
		seen[chooserType] = true

		optMatches := optionOpenRe.FindAllStringSubmatch(chooserBody, -1)
		options := make([]string, 0, len(optMatches))
		for _, optMatch := range optMatches {
			options = append(options, optMatch[1])
		}
		result = append(result, chooserInfo{chooserType: chooserType, options: options})
	}

	return result
}

// buildChooserSelections builds a selections map for docsrender.ResolveChoosers.
// It resolves selections from flags, stored preferences, and interactive prompts.
func buildChooserSelections(
	body string, prefs *docsrender.Preferences,
	flagLang, flagOS string,
	session map[string]string,
) map[string]string {
	selections := map[string]string{}

	// Apply flag overrides
	if flagLang != "" {
		selections[docsrender.ChooserLanguage] = flagLang
	}
	if flagOS != "" {
		selections[docsrender.ChooserOS] = flagOS
	}

	// Apply session selections (reuse prior interactive choices)
	for k, v := range session {
		if _, ok := selections[k]; !ok {
			selections[k] = v
		}
	}

	// Apply stored preferences for types not yet resolved
	for _, t := range []string{docsrender.ChooserLanguage, docsrender.ChooserOS, docsrender.ChooserCloud} {
		if _, ok := selections[t]; !ok {
			if v := prefs.Get(t); v != "" {
				selections[t] = v
			}
		}
	}

	// For interactive mode, prompt for any chooser types still unresolved
	if cmdutil.Interactive() {
		choosers := scanChoosers(body)
		for _, ci := range choosers {
			if _, ok := selections[ci.chooserType]; ok {
				continue
			}
			if len(ci.options) == 0 {
				continue
			}
			defaultOpt := ci.options[0]
			if pref := prefs.Get(ci.chooserType); pref != "" {
				defaultOpt = pref
			}
			selected := ui.PromptUser(
				"Select "+ci.chooserType+":",
				ci.options,
				defaultOpt,
				cmdutil.GetGlobalColorization(),
			)
			if selected != "" {
				selections[ci.chooserType] = selected
			}
		}
	}

	// Persist selections
	for k, v := range selections {
		session[k] = v
		prefs.Set(k, v)
	}
	docsrender.SavePreferences(prefs)

	return selections
}
