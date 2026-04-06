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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/docsrender"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// buildChooserSelections builds a selections map for docsrender.ResolveChoosers.
// It resolves selections from flags, stored preferences, and interactive prompts.
func buildChooserSelections(
	choosers []docsrender.ChooserInfo, prefs *docsrender.Preferences,
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
		for _, ci := range choosers {
			if _, ok := selections[ci.Type]; ok {
				continue
			}
			if len(ci.Options) == 0 {
				continue
			}
			defaultOpt := ci.Options[0]
			if pref := prefs.Get(ci.Type); pref != "" {
				defaultOpt = pref
			}
			selected := ui.PromptUser(
				"Select "+ci.Type+":",
				ci.Options,
				defaultOpt,
				cmdutil.GetGlobalColorization(),
			)
			if selected != "" {
				selections[ci.Type] = selected
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
