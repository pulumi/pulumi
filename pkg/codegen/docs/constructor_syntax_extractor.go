// Copyright 2016-2020, Pulumi Corporation.
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
	"fmt"
	"strings"
)

func extractConstructorSyntaxExamples(
	program string,
	indentToTrim string,
	commentDelimiter string,
	exampleEndPredicate func(string) bool,
) *languageConstructorSyntax {
	resources := map[string]string{}
	invokes := map[string]string{}
	currentExample := ""
	currentToken := ""
	readingResource := false
	readingInvoke := false

	for _, line := range strings.Split(program, "\n") {
		line = strings.TrimPrefix(line, indentToTrim)
		if strings.HasPrefix(line, fmt.Sprintf("%s Resource", commentDelimiter)) {
			currentExample = ""
			readingResource = true
			readingInvoke = false
			commentParts := strings.Split(line, " ")
			currentToken = commentParts[len(commentParts)-1]
			continue
		}

		if strings.HasPrefix(line, fmt.Sprintf("%s Invoke", commentDelimiter)) {
			currentExample = ""
			readingInvoke = true
			readingResource = false
			commentParts := strings.Split(line, " ")
			currentToken = commentParts[len(commentParts)-1]
			continue
		}

		if readingResource || readingInvoke {
			if currentExample != "" {
				currentExample = currentExample + "\n" + line
			} else {
				currentExample = line
			}
		}

		if exampleEndPredicate(line) && currentToken != "" {
			if readingResource {
				resources[currentToken] = currentExample
			} else if readingInvoke {
				invokes[currentToken] = currentExample
			}
			currentToken = ""
			readingResource = false
			readingInvoke = false
		}
	}

	return &languageConstructorSyntax{
		resources: resources,
		invokes:   invokes,
	}
}
