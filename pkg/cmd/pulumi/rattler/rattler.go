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

// Package rattler patches a Cobra command tree so that invalid invocations
// fail with a non-zero exit code and an "unknown command" error that suggests
// closely-matching commands from the whole tree.
//
// Cobra's own handling falls short in two ways: its "Did you mean this?"
// suggestions fire only at the root and only consider the root's direct
// children, so a nested command like `stack export` can never be suggested
// for `pulumi export`; and non-runnable group commands return flag.ErrHelp
// before args are ever validated, which Execute turns into "print help,
// exit 0".
//
// The name continues the snake theme set by cobra and constrictor: a rattler
// rattles a warning when you are headed down a bad path.
package rattler

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/texttheater/golang-levenshtein/levenshtein"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// Install walks the command tree rooted at c and patches commands so that
// invalid invocations fail with a non-zero exit code and an "unknown command"
// error that suggests closely-matching commands from the whole tree. See the
// package documentation for what this fixes over Cobra's own handling.
func Install(c *cobra.Command) {
	for _, child := range c.Commands() {
		Install(child)
	}
	// Commands that parse their own args (`pulumi do`) manage their own errors.
	if !c.HasSubCommands() || c.DisableFlagParsing {
		return
	}

	if !c.Runnable() {
		// A group command, including the root: a positional arg can only be an
		// attempted subcommand, so an unknown-command error beats whatever
		// arg-count validator the command may have declared. A bare group
		// invocation shows help but still exits non-zero; the bail error sets
		// the exit code without printing anything after the help.
		c.Args = func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}
			return unknownCommandError(cmd, args)
		}
		c.RunE = func(cmd *cobra.Command, args []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return result.BailErrorf("%q requires a subcommand", cmd.CommandPath())
		}
	} else {
		// A runnable command with subcommands (`stack`, `config`, ...):
		// positional args may be legitimate, so only treat them as an
		// attempted subcommand when the command's own argument specification
		// cannot accept that many, and blame the first arg past the spec.
		orig := c.Args
		c.Args = func(cmd *cobra.Command, args []string) error {
			if extra := argsPastSpec(cmd, args); len(extra) > 0 {
				return unknownCommandError(cmd, extra)
			}
			if orig != nil {
				return orig(cmd, args)
			}
			return nil
		}
	}
}

// argsPastSpec returns the positional arguments beyond what the command's
// constrictor argument specification can accept; the first of these must be
// an attempted subcommand. Commands without a spec never escalate, so their
// own validators still apply.
func argsPastSpec(cmd *cobra.Command, args []string) []string {
	spec, err := constrictor.ExtractArgs(cmd)
	if err != nil || spec.Variadic || len(args) <= len(spec.Arguments) {
		return nil
	}
	return args[len(spec.Arguments):]
}

const maxSuggestions = 3

func unknownCommandError(cmd *cobra.Command, args []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "unknown command %q for %q", args[0], cmd.CommandPath())
	if suggestions := suggestCommands(cmd, args); len(suggestions) > 0 {
		b.WriteString("\n\nDid you mean this?\n")
		for _, s := range suggestions {
			fmt.Fprintf(&b, "\t%s\n", s)
		}
	}
	fmt.Fprintf(&b, "\nRun '%s --help' for usage.", cmd.CommandPath())
	return errors.New(b.String())
}

// A word of user input to match against candidate commands. Words derived
// from the unrecognized argument are marked so that candidates which match
// only the already-resolved part of the command line can be rejected.
type typedPart struct {
	word        string
	fromFailing bool
}

// suggestCommands returns up to maxSuggestions full command paths that
// closely match what the user typed. Commands under the failing command are
// preferred; the whole tree is searched only when nothing nearby qualifies.
func suggestCommands(cmd *cobra.Command, args []string) []string {
	var typed []typedPart
	for _, x := range pathFromRoot(cmd) {
		for _, w := range normalize(x.Name()) {
			typed = append(typed, typedPart{word: w})
		}
	}
	for i, a := range args {
		for _, w := range normalize(a) {
			typed = append(typed, typedPart{word: w, fromFailing: i == 0})
		}
	}
	if len(typed) == 0 {
		return nil
	}

	if cmd.HasParent() {
		if s := rankCandidates(typed, collectCandidates(cmd, nil)); len(s) > 0 {
			return s
		}
	}
	return rankCandidates(typed, collectCandidates(cmd.Root(), nil))
}

// A command reachable in the tree, with the canonical words its path answers
// to. Each slot is the set of canonical names for one word of the path.
type suggestionCandidate struct {
	path  string // full display path, e.g. "pulumi stack export"
	depth int
	slots [][]string
}

// collectCandidates gathers every visible command under c (excluding c
// itself), pruning hidden, deprecated, and internal subtrees.
func collectCandidates(c *cobra.Command, out []suggestionCandidate) []suggestionCandidate {
	for _, child := range c.Commands() {
		if child.Hidden || child.Deprecated != "" ||
			strings.HasPrefix(child.Name(), "__") || child.Name() == "help" {
			continue
		}
		out = append(out, makeCandidate(child))
		out = collectCandidates(child, out)
	}
	return out
}

func makeCandidate(c *cobra.Command) suggestionCandidate {
	chain := pathFromRoot(c)
	var slots [][]string
	for _, x := range chain {
		parts := normalize(x.Name())
		if len(parts) == 1 {
			// Single-word names also answer to their aliases and SuggestFor
			// entries.
			names := parts
			for _, alt := range slices.Concat(x.Aliases, x.SuggestFor) {
				if ap := normalize(alt); len(ap) == 1 && !slices.Contains(names, ap[0]) {
					names = append(names, ap[0])
				}
			}
			slots = append(slots, names)
		} else {
			for _, p := range parts {
				slots = append(slots, []string{p})
			}
		}
	}
	return suggestionCandidate{path: c.CommandPath(), depth: len(chain), slots: slots}
}

// pathFromRoot returns the commands from just below the root down to c
// itself. For the root it returns nothing.
func pathFromRoot(c *cobra.Command) []*cobra.Command {
	var chain []*cobra.Command
	for x := c; x.HasParent(); x = x.Parent() {
		chain = append(chain, x)
	}
	slices.Reverse(chain)
	return chain
}

func rankCandidates(typed []typedPart, cands []suggestionCandidate) []string {
	type scored struct {
		cand     suggestionCandidate
		score    int
		matchSum int
	}
	var eligible []scored
	for _, cand := range cands {
		score, matchSum, failingMatched := scoreCandidate(typed, cand)
		if !failingMatched || score < 2 {
			continue
		}
		eligible = append(eligible, scored{cand, score, matchSum})
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.matchSum != b.matchSum {
			return a.matchSum > b.matchSum
		}
		if a.cand.depth != b.cand.depth {
			return a.cand.depth < b.cand.depth
		}
		return a.cand.path < b.cand.path
	})
	var out []string
	for _, e := range eligible {
		// Suggestions well below the best match are noise, not alternatives.
		if len(out) == maxSuggestions || e.score < eligible[0].score-2 {
			break
		}
		out = append(out, e.cand.path)
	}
	return out
}

// scoreCandidate greedily matches each typed word against the best unused
// candidate slot. The score rewards matched words and penalizes candidate
// words the user never typed, so `stack export` scores 3-1=2 for the input
// `export` while `import` scores its edit-distance match value of 2.
func scoreCandidate(typed []typedPart, cand suggestionCandidate) (score, matchSum int, failingMatched bool) {
	used := make([]bool, len(cand.slots))
	for _, tp := range typed {
		best, bestIdx := 0, -1
		for i, slot := range cand.slots {
			if used[i] {
				continue
			}
			if v := matchValue(tp.word, slot); v > best {
				best, bestIdx = v, i
			}
		}
		if bestIdx >= 0 {
			used[bestIdx] = true
			matchSum += best
			if tp.fromFailing {
				failingMatched = true
			}
		}
	}
	unmatched := 0
	for _, u := range used {
		if !u {
			unmatched++
		}
	}
	return matchSum - unmatched, matchSum, failingMatched
}

func matchValue(word string, slot []string) int {
	best := 0
	for _, name := range slot {
		v := 0
		switch {
		case word == name:
			v = 3
		case closeEnough(word, name):
			v = 2
		case len(word) >= 3 && strings.HasPrefix(name, word),
			len(name) >= 3 && strings.HasPrefix(word, name):
			v = 1
		}
		if v > best {
			best = v
		}
	}
	return best
}

// Common synonyms mapped to the canonical word used for matching, so that
// `pulumi webhook create` can rank `pulumi stack webhook new` highly. The map
// is many-to-one, but matching is symmetric: normalize runs over both the
// typed words and the candidate command names, so any two words with the same
// canonical form match no matter which of them the user typed.
var synonyms = map[string]string{
	"ls":      "list",
	"rm":      "remove",
	"del":     "remove",
	"delete":  "remove",
	"create":  "new",
	"add":     "new",
	"show":    "get",
	"display": "get",
	"upgrade": "update",
}

// normalize splits a command token into canonical words: lowercased,
// hyphen-separated, mapped through the synonym table, with a trivial plural
// 's' stripped so `list-members` compares equal to `member list`.
func normalize(token string) []string {
	var parts []string
	for p := range strings.SplitSeq(strings.ToLower(token), "-") {
		if p == "" {
			continue
		}
		if s, ok := synonyms[p]; ok {
			p = s
		} else {
			if len(p) > 3 && strings.HasSuffix(p, "s") {
				p = strings.TrimSuffix(p, "s")
			}
			if s, ok := synonyms[p]; ok {
				p = s
			}
		}
		parts = append(parts, p)
	}
	return parts
}

var levenshteinOptions = func() levenshtein.Options {
	op := levenshtein.DefaultOptionsWithSub
	op.Matches = func(r1, r2 rune) bool {
		return unicode.ToLower(r1) == unicode.ToLower(r2)
	}
	return op
}()

// closeEnough scales the allowed edit distance with the longer string.
func closeEnough(a, b string) bool {
	threshold := 2
	if max(len(a), len(b)) < 6 {
		threshold = 1
	}
	return levenshtein.DistanceForStrings([]rune(a), []rune(b), levenshteinOptions) <= threshold
}
