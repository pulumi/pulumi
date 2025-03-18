// Copyright 2016-2024, Pulumi Corporation.
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

package display

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// summarizeInternal handles the actual summarization logic and returns proper errors
func summarizeInternal(lines []string, orgID string, atlasResp apitype.AtlasUpdateSummaryResponse) (string, error) {
	// Look for the first summarizeUpdate message
	for _, msg := range atlasResp.ThreadMessages {
		if msg.Kind == "summarizeUpdate" {
			var content apitype.SummarizeUpdate
			if err := json.Unmarshal(msg.Content, &content); err != nil {
				return "", fmt.Errorf("parsing summary content: %w", err)
			}
			if content.Summary == "" {
				return "", errors.New("no summary generated")
			}
			return content.Summary, nil
		}
	}

	return "", errors.New("no summarizeUpdate message found in response")
}

// addPrefixToLines adds the given prefix to each line of the input text
func addPrefixToLines(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// summarize generates a summary of the update output
func summarize(orgID string, lines []string, atlasResp apitype.AtlasUpdateSummaryResponse, outputPrefix string) string {
	if len(lines) == 0 {
		return ""
	}

	summary, err := summarizeInternal(lines, orgID, atlasResp)
	if err != nil {
		// TODO: Use proper logging once we have it
		fmt.Fprintf(os.Stderr, "Error generating summary: %v\n", err)
		return ""
	}

	return addPrefixToLines(summary, outputPrefix)
}
