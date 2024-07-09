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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

const (
	beginCodeBlock = "<!--Start PulumiCodeChooser -->"
	endCodeBlock   = "<!--End PulumiCodeChooser -->"
)

type codeLocation struct {
	open  int
	close int
}

func getCodeSection(doc string) []codeLocation {
	var fences []codeLocation

	startIndex := 0
	for {
		open := strings.Index(doc[startIndex:], beginCodeBlock)
		if open == -1 {
			break
		}
		var fence codeLocation
		fence.open = startIndex + open
		startIndex += open + len(beginCodeBlock)
		closing := strings.Index(doc[startIndex:], endCodeBlock)

		contract.Assertf(closing != -1, "this should never happen: "+
			"there should be equal amounts of opening and closing code block markers")

		fence.close = startIndex + closing

		startIndex += closing + len(endCodeBlock)
		fences = append(fences, fence)
	}
	return fences
}

func markupBlock(block, supportedSnippetLanguages string) string {
	languages := []struct{ tag, choosable string }{
		{"typescript", "<div>\n<pulumi-choosable type=\"language\" values=\"javascript,typescript\">\n\n"},
		{"python", "<div>\n<pulumi-choosable type=\"language\" values=\"python\">\n\n"},
		{"go", "<div>\n<pulumi-choosable type=\"language\" values=\"go\">\n\n"},
		{"csharp", "<div>\n<pulumi-choosable type=\"language\" values=\"csharp\">\n\n"},
		{"java", "<div>\n<pulumi-choosable type=\"language\" values=\"java\">\n\n"},
		{"yaml", "<div>\n<pulumi-choosable type=\"language\" values=\"yaml\">\n\n"},
	}
	const (
		//nolint:lll
		chooserStartFmt = "<div>\n<pulumi-chooser type=\"language\" options=\"%s\"></pulumi-chooser>\n</div>\n"
		choosableEnd    = "</pulumi-choosable>\n</div>\n"
	)

	var markedUpBlock strings.Builder
	// first, append the start chooser
	markedUpBlock.WriteString(fmt.Sprintf(chooserStartFmt, supportedSnippetLanguages))

	for _, lang := range languages {
		// Add language specific open choosable
		markedUpBlock.WriteString(lang.choosable)
		// find our language - because we have no guarantee of order from our input, we need to find
		// both code fences and then append the content in the order that docsgen expects.
		start := strings.Index(block, "```"+lang.tag)
		if start == -1 {
			markedUpBlock.WriteString("```\n")
			markedUpBlock.WriteString(defaultMissingExampleSnippetPlaceholder)
			markedUpBlock.WriteString("\n```\n")
		} else {
			// find end index - this is the next code fence.
			endLangBlock := start + len("```"+lang.tag) + strings.Index(block[start+len("```"+lang.tag):], "```")
			// append code to block, and include code fences
			markedUpBlock.WriteString(block[start : endLangBlock+len("```")])
			markedUpBlock.WriteRune('\n')
		}
		// add closing choosable
		markedUpBlock.WriteString(choosableEnd)
	}
	return markedUpBlock.String()
}

func (dctx *docGenContext) processDescription(description, supportedSnippetLanguages string) docInfo {
	importDetails := ""
	parts := strings.Split(description, "\n\n## Import")
	if len(parts) > 1 {
		importDetails = parts[1]
		description = parts[0]
	}

	codeBlocks := getCodeSection(description)

	startIndex := 0
	var markedUpDescription string
	for _, block := range codeBlocks {
		// append text
		markedUpDescription += description[startIndex:block.open]
		codeBlock := description[block.open:block.close]
		// append marked up block
		markedUpDescription += markupBlock(codeBlock, supportedSnippetLanguages)
		startIndex = block.close + len(endCodeBlock)
	}
	// append remainder of description, if any
	markedUpDescription += description[startIndex:]

	return docInfo{
		description:   markedUpDescription,
		importDetails: importDetails,
	}
}
