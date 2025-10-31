// Copyright 2024, Pulumi Corporation.
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

package autonaming

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

type StackContext struct {
	Organization	string
	Project		string
	Stack		string
}

// stackPatternEval is a helper struct for resolving stack-level expressions in autonaming patterns.
// It's used to resolve ${organization}, ${project}, ${stack}, and ${config.key} expressions in patterns.
// These are all expressions that can be resolved at startup time because they don't depend
// on the resource URN.
type stackPatternEval struct {
	ctx		StackContext
	getConfigValue	func(key string) (string, error)
}

// newStackPatternEval creates a new stack pattern evaluator based on the given stack and configuration.
func newStackPatternEval(s StackContext, cfg config.Map, decrypter config.Decrypter,
) *stackPatternEval {
	getConfigValue := func(key string) (string, error) {
		c, ok, err := cfg.Get(config.MustMakeKey(s.Project, key), true)
		if err != nil {
			return "", fmt.Errorf("failed to get config value for key %q: %w", key, err)
		}
		if !ok {
			return "", fmt.Errorf("no value found for key %q", key)
		}
		v, err := c.Value(decrypter)
		if err != nil {
			return "", fmt.Errorf("failed to decrypt value for key %q: %w", key, err)
		}
		return v, nil
	}
	return &stackPatternEval{
		ctx:		s,
		getConfigValue:	getConfigValue,
	}
}

var configRegex = regexp.MustCompile(`\${config\.([^}]+)}`)

// resolveStackExpressions resolves the organization, project, stack, and config expressions in the given pattern.
func (e *stackPatternEval) resolveStackExpressions(pattern string) (string, error) {
	// Replace ${organization}, ${project}, ${stack} with values from context
	pattern = strings.ReplaceAll(pattern, "${organization}", e.ctx.Organization)
	pattern = strings.ReplaceAll(pattern, "${project}", e.ctx.Project)
	pattern = strings.ReplaceAll(pattern, "${stack}", e.ctx.Stack)

	// Replace ${config.key} with config values
	var configErr error
	pattern = configRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		key := configRegex.FindStringSubmatch(match)[1]
		v, err := e.getConfigValue(key)
		if err != nil {
			configErr = err
			return ""
		}
		return v
	})
	if configErr != nil {
		return "", configErr
	}
	return pattern, nil
}
