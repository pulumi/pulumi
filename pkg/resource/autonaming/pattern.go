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
	"crypto"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"lukechampine.com/frand"
)

// stackPatternEval is a helper struct for resolving stack-level expressions in autonaming patterns.
// It's used to resolve ${organization}, ${project}, ${stack}, and ${config.key} expressions in patterns.
// These are all expressions that can be resolved at startup time because they don't depend
// on the resource URN.
type stackPatternEval struct {
	organization   string
	project        string
	stack          string
	getConfigValue func(key string) (string, error)
}

// newStackPatternEval creates a new stack pattern evaluator based on the given stack and configuration.
func newStackPatternEval(s backend.Stack, cfg *backend.StackConfiguration, decrypter config.Decrypter,
) *stackPatternEval {
	organization := "organization"
	if cs, ok := s.(httpstate.Stack); ok {
		organization = cs.OrgName()
	}
	project := "project"
	if projName, ok := s.Ref().Project(); ok {
		project = projName.String()
	}
	stack := s.Ref().Name().String()
	getConfigValue := func(key string) (string, error) {
		c, ok, err := cfg.Config.Get(config.MustMakeKey(project, key), true)
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
		organization:   organization,
		project:        project,
		stack:          stack,
		getConfigValue: getConfigValue,
	}
}

// Regexes for resolving expressions in patterns.
var (
	configRegex = regexp.MustCompile(`\${config\.([^}]+)}`)
	hexRegex    = regexp.MustCompile(`\${hex\((\d+)\)}`)
	alphaRegex  = regexp.MustCompile(`\${alphanum\((\d+)\)}`)
	strRegex    = regexp.MustCompile(`\${string\((\d+)\)}`)
	numRegex    = regexp.MustCompile(`\${num\((\d+)\)}`)
)

// resolveStackExpressions resolves the organization, project, stack, and config expressions in the given pattern.
func (e *stackPatternEval) resolveStackExpressions(pattern string) (string, error) {
	// Replace ${organization}, ${project}, ${stack} with values from context
	pattern = strings.ReplaceAll(pattern, "${organization}", e.organization)
	pattern = strings.ReplaceAll(pattern, "${project}", e.project)
	pattern = strings.ReplaceAll(pattern, "${stack}", e.stack)

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

func replaceHex(pattern string, random *frand.RNG) string {
	return hexRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		n, _ := strconv.Atoi(hexRegex.FindStringSubmatch(match)[1])
		b := make([]byte, n/2+1)
		_, _ = random.Read(b)
		return hex.EncodeToString(b)[:n]
	})
}

func replaceAlphanum(pattern string, random *frand.RNG) string {
	return alphaRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		n, _ := strconv.Atoi(alphaRegex.FindStringSubmatch(match)[1])
		const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
		b := make([]byte, n)
		for i := range b {
			b[i] = chars[random.Intn(len(chars))]
		}
		return string(b)
	})
}

func replaceString(pattern string, random *frand.RNG) string {
	return strRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		n, _ := strconv.Atoi(strRegex.FindStringSubmatch(match)[1])
		const chars = "abcdefghijklmnopqrstuvwxyz"
		b := make([]byte, n)
		for i := range b {
			b[i] = chars[random.Intn(len(chars))]
		}
		return string(b)
	})
}

func replaceNum(pattern string, random *frand.RNG) string {
	return numRegex.ReplaceAllStringFunc(pattern, func(match string) string {
		n, _ := strconv.Atoi(numRegex.FindStringSubmatch(match)[1])
		const chars = "0123456789"
		b := make([]byte, n)
		for i := range b {
			b[i] = chars[random.Intn(len(chars))]
		}
		return string(b)
	})
}

func replaceUUID(pattern string, random *frand.RNG) string {
	if strings.Contains(pattern, "${uuid}") {
		uuidBytes := make([]byte, 16)
		_, _ = random.Read(uuidBytes)
		pattern = strings.ReplaceAll(pattern, "${uuid}", uuid.Must(uuid.FromBytes(uuidBytes)).String())
	}
	return pattern
}

var randomExpressionReplacers = []func(string, *frand.RNG) string{
	replaceHex,
	replaceAlphanum,
	replaceString,
	replaceNum,
	replaceUUID,
}

// generateName generates a final proposed name based on the configured pattern, the resource URN, and random seed.
// Note that the pattern is expected to have already had stack-level expressions like ${organization}, ${project}, ${stack},
// and ${config.key} resolved before passing to this function.
func generateName(pattern string, urn urn.URN, randomSeed []byte) (string, bool) {
	// Replace ${name} with the logical name
	result := strings.ReplaceAll(pattern, "${name}", urn.Name())
	hasRandom := false

	// Create a random number generator with the given seed. If no seed is provided, use a default
	// random number generator.
	var random *frand.RNG
	if len(randomSeed) == 0 {
		random = frand.New()
	} else {
		// frand.NewCustom needs a 32 byte seed. Take the SHA256 hash of whatever bytes we've been given as a
		// seed and pass the 32 byte result of that to frand.
		hash := crypto.SHA256.New()
		hash.Write(randomSeed)
		seed := hash.Sum(nil)
		bufsize := 1024 // Same bufsize as used by frand.New.
		rounds := 12    // Same rounds as used by frand.New.
		random = frand.NewCustom(seed, bufsize, rounds)
	}

	for _, replacer := range randomExpressionReplacers {
		newResult := replacer(result, random)
		hasRandom = hasRandom || newResult != result
		result = newResult
	}

	return result, hasRandom
}
