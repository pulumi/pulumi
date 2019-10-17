// Copyright 2016-2018, Pulumi Corporation.
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

package python

import (
	"strings"
	"unicode"
)

// PyName turns a variable or function name, normally using camelCase, to an underscore_case name.
func PyName(name string) string {
	// This method is a state machine with four states:
	//   stateFirst - the initial state.
	//   stateUpper - The last character we saw was an uppercase letter and the character before it
	//                was either a number or a lowercase letter.
	//   stateAcronym - The last character we saw was an uppercase letter and the character before it
	//                  was an uppercase letter.
	//   stateLowerOrNumber - The last character we saw was a lowercase letter or a number.
	//
	// The following are the state transitions of this state machine:
	//   stateFirst -> (uppercase letter) -> stateUpper
	//   stateFirst -> (lowercase letter or number) -> stateLowerOrNumber
	//      Append the lower-case form of the character to currentComponent.
	//
	//   stateUpper -> (uppercase letter) -> stateAcronym
	//   stateUpper -> (lowercase letter or number) -> stateLowerOrNumber
	//      Append the lower-case form of the character to currentComponent.
	//
	//   stateAcronym -> (uppercase letter) -> stateAcronym
	//		Append the lower-case form of the character to currentComponent.
	//   stateAcronym -> (number) -> stateLowerOrNumber
	//      Append the character to currentComponent.
	//   stateAcronym -> (lowercase letter) -> stateLowerOrNumber
	//      Take all but the last character in currentComponent, turn that into
	//      a string, and append that to components. Set currentComponent to the
	//      last two characters seen.
	//
	//   stateLowerOrNumber -> (uppercase letter) -> stateUpper
	//      Take all characters in currentComponent, turn that into a string,
	//      and append that to components. Set currentComponent to the last
	//      character seen.
	//	 stateLowerOrNumber -> (lowercase letter) -> stateLowerOrNumber
	//      Append the character to currentComponent.
	//
	// The Go libraries that convert camelCase to snake_case deviate subtly from
	// the semantics we're going for in this method, namely that they separate
	// numbers and lowercase letters. We don't want this in all cases (we want e.g. Sha256Hash to
	// be converted as sha256_hash). We also want SHA256Hash to be converted as sha256_hash, so
	// we must at least be aware of digits when in the stateAcronym state.
	//
	// As for why this is a state machine, the libraries that do this all pretty much use
	// either regular expressions or state machines, which I suppose are ultimately the same thing.
	const (
		stateFirst = iota
		stateUpper
		stateAcronym
		stateLowerOrNumber
	)

	var components []string     // The components that will be joined together with underscores
	var currentComponent []rune // The characters composing the current component being built
	state := stateFirst
	for _, char := range name {
		switch state {
		case stateFirst:
			if unicode.IsUpper(char) {
				// stateFirst -> stateUpper
				state = stateUpper
				currentComponent = append(currentComponent, unicode.ToLower(char))
				continue
			}

			// stateFirst -> stateLowerOrNumber
			state = stateLowerOrNumber
			currentComponent = append(currentComponent, char)
			continue

		case stateUpper:
			if unicode.IsUpper(char) {
				// stateUpper -> stateAcronym
				state = stateAcronym
				currentComponent = append(currentComponent, unicode.ToLower(char))
				continue
			}

			// stateUpper -> stateLowerOrNumber
			state = stateLowerOrNumber
			currentComponent = append(currentComponent, char)
			continue

		case stateAcronym:
			if unicode.IsUpper(char) {
				// stateAcronym -> stateAcronym
				currentComponent = append(currentComponent, unicode.ToLower(char))
				continue
			}

			// We want to fold digits immediately following an acronym into the same
			// component as the acronym.
			if unicode.IsDigit(char) {
				// stateAcronym -> stateLowerOrNumber
				currentComponent = append(currentComponent, char)
				state = stateLowerOrNumber
				continue
			}

			// stateAcronym -> stateLowerOrNumber
			last, rest := currentComponent[len(currentComponent)-1], currentComponent[:len(currentComponent)-1]
			components = append(components, string(rest))
			currentComponent = []rune{last, char}
			state = stateLowerOrNumber
			continue

		case stateLowerOrNumber:
			if unicode.IsUpper(char) {
				// stateLowerOrNumber -> stateUpper
				components = append(components, string(currentComponent))
				currentComponent = []rune{unicode.ToLower(char)}
				state = stateUpper
				continue
			}

			// stateLowerOrNumber -> stateLowerOrNumber
			currentComponent = append(currentComponent, char)
			continue
		}
	}

	components = append(components, string(currentComponent))
	result := strings.Join(components, "_")
	return EnsureKeywordSafe(result)
}

// Keywords is a map of reserved keywords used by Python 2 and 3.  We use this to avoid generating unspeakable
// names in the resulting code.  This map was sourced by merging the following reference material:
//
//     * Python 2: https://docs.python.org/2.5/ref/keywords.html
//     * Python 3: https://docs.python.org/3/reference/lexical_analysis.html#keywords
//
var Keywords = map[string]bool{
	"False":    true,
	"None":     true,
	"True":     true,
	"and":      true,
	"as":       true,
	"assert":   true,
	"async":    true,
	"await":    true,
	"break":    true,
	"class":    true,
	"continue": true,
	"def":      true,
	"del":      true,
	"elif":     true,
	"else":     true,
	"except":   true,
	"exec":     true,
	"finally":  true,
	"for":      true,
	"from":     true,
	"global":   true,
	"if":       true,
	"import":   true,
	"in":       true,
	"is":       true,
	"lambda":   true,
	"nonlocal": true,
	"not":      true,
	"or":       true,
	"pass":     true,
	"print":    true,
	"raise":    true,
	"return":   true,
	"try":      true,
	"while":    true,
	"with":     true,
	"yield":    true,
}

// EnsureKeywordSafe adds a trailing underscore if the generated name clashes with a Python 2 or 3 keyword, per
// PEP 8: https://www.python.org/dev/peps/pep-0008/?#function-and-method-arguments
func EnsureKeywordSafe(name string) string {
	if _, isKeyword := Keywords[name]; isKeyword {
		return name + "_"
	}
	return name
}
