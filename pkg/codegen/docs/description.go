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

// Pulling out some of the repeated strings tokens into constants would harm readability, so we just ignore the
// goconst linter's warning.
//
//nolint:lll, goconst
package docs

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"strings"
)

var (
	beginCodeBlock = "<!--Begin Code Chooser -->"
	endCodeBlock   = "<!--End Code Chooser -->"
)

func getCodeBlockIndices(s, openStr, closeStr string) ([]int, []int) {
	var openIndices []int
	var closeIndices []int

	startIndex := 0
	for {
		open := strings.Index(s[startIndex:], openStr)
		if open == -1 {
			break
		}
		openIndices = append(openIndices, startIndex+open)
		startIndex += open + len(openStr)
		closing := strings.Index(s[startIndex:], closeStr)
		if closing == -1 {
			contract.Failf("this should never happen: there should be equal amounts of opening and closing code block markers")
		}
		closeIndices = append(closeIndices, startIndex+closing)
		startIndex += closing + len(closeStr)

	}
	return openIndices, closeIndices
}

func markupBlock(block string) string {
	languages := []string{
		"typescript",
		"python",
		"go",
		"csharp",
		"java",
		"yaml",
	}
	choosables := []string{
		"<div>\n<pulumi-choosable type=\"language\" values=\"javascript,typescript\">\n\n",
		"<div>\n<pulumi-choosable type=\"language\" values=\"python\">\n\n",
		"<div>\n<pulumi-choosable type=\"language\" values=\"go\">\n\n",
		"<div>\n<pulumi-choosable type=\"language\" values=\"csharp\">\n\n",
		"<div>\n<pulumi-choosable type=\"language\" values=\"java\">\n\n",
		"<div>\n<pulumi-choosable type=\"language\" values=\"yaml\">\n\n",
	}

	chooserStart := "<div>\n<pulumi-chooser type=\"language\" options=\"typescript,python,go,csharp,java,yaml\"></pulumi-chooser>\n</div>\n"
	choosableEnd := "</pulumi-choosable>\n</div>\n"

	var markedUpBlock string
	// first, append the start chooser
	markedUpBlock += chooserStart

	for lang, choosable := range choosables {
		// Add language specific open choosable
		markedUpBlock += choosable
		// find our language - because we have no guarantee of order from our input, we need to find both code fences
		// and then append the content in the order that docsgen expects.
		start := strings.Index(block, "```"+languages[lang])
		if start == -1 {
			markedUpBlock += "```\n" + defaultMissingExampleSnippetPlaceholder + "```\n"
		} else {
			// find end index - this is the next code fence. Unfortunately Go doesn't allow us to look for the second
			// instance of a substring.
			endLangBlock := start + len("```"+languages[lang]) + strings.Index(block[start+len("```"+languages[lang]):], "```")
			// append code to block, and include code fences
			markedUpBlock += block[start:endLangBlock+len("```")] + "\n"
		}
		// add close choosable
		markedUpBlock += choosableEnd
	}
	return markedUpBlock
}

func (dctx *docGenContext) processDescription(description string) docInfo {
	importDetails := ""
	parts := strings.Split(description, "\n\n## Import")
	if len(parts) > 1 {
		importDetails = parts[1]
		description = parts[0]
	}
	openIndices, closeIndices := getSubstringIndices(description, beginCodeBlock, endCodeBlock)

	startIndex := 0
	var markedUpDescription string
	for i := range openIndices {
		// append text
		markedUpDescription += description[startIndex:openIndices[i]]
		codeBlock := description[openIndices[i]:closeIndices[i]]
		// append marked up block
		markedUpDescription += markupBlock(codeBlock)
		startIndex = closeIndices[i] + len(endCodeBlock)
	}
	// append remainder of description, if any
	markedUpDescription += description[startIndex:]

	return docInfo{
		description:   markedUpDescription,
		importDetails: importDetails,
	}
}
