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
	"strings"

	"gopkg.in/yaml.v3"
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
		if strings.HasPrefix(line, commentDelimiter+" Resource") {
			currentExample = ""
			readingResource = true
			readingInvoke = false
			commentParts := strings.Split(line, " ")
			currentToken = commentParts[len(commentParts)-1]
			continue
		}

		if strings.HasPrefix(line, commentDelimiter+" Invoke") {
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

func extractConstructorSyntaxExamplesFromYAML(programText string) *languageConstructorSyntax {
	resources := map[string]string{}
	invokes := map[string]string{}

	type resource struct {
		Type       string                 `yaml:"type"`
		Properties map[string]interface{} `yaml:"properties"`
	}

	type yamlProgram struct {
		Resources map[string]resource `yaml:"resources"`
	}

	var program yamlProgram
	err := yaml.Unmarshal([]byte(programText), &program)
	if err != nil {
		return &languageConstructorSyntax{
			resources: resources,
			invokes:   invokes,
		}
	}

	for _, resourceDefinition := range program.Resources {
		if serializedResource, err := yaml.Marshal(resourceDefinition); err == nil {
			resources[resourceDefinition.Type] = strings.TrimSuffix(string(serializedResource), "\n")
		}
	}

	return &languageConstructorSyntax{
		resources: resources,
		invokes:   invokes,
	}
}
