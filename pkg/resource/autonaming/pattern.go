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
// It's used to resolve ${org}, ${project}, ${stack}, and ${config.key} expressions in patterns.
// These are all expressions that can be resolved at the startup time because they don't depend
// on the resource URN.
type stackPatternEval struct {
	org            string
	proj           string
	stack          string
	getConfigValue func(key string) (string, error)
}

// newStackPatternEval creates a new stack pattern evaluator based on the given stack and configuration.
func newStackPatternEval(s backend.Stack, cfg *backend.StackConfiguration, decrypter config.Decrypter,
) *stackPatternEval {
	org := "default"
	if cs, ok := s.(httpstate.Stack); ok {
		org = cs.OrgName()
	}
	projName, _ := s.Ref().Project()
	projStr := projName.String()
	stackStr := s.Ref().Name().String()
	getConfigValue := func(key string) (string, error) {
		c, ok, err := cfg.Config.Get(config.MustMakeKey(projStr, key), true)
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
		org:            org,
		proj:           projStr,
		stack:          stackStr,
		getConfigValue: getConfigValue,
	}
}

// resolveStackExpressions resolves the org, project, stack, and config expressions in the given pattern.
func (e *stackPatternEval) resolveStackExpressions(pattern string) (string, error) {
	// Replace ${org}, ${project}, ${stack} with values from context
	pattern = strings.ReplaceAll(pattern, "${org}", e.org)
	pattern = strings.ReplaceAll(pattern, "${project}", e.proj)
	pattern = strings.ReplaceAll(pattern, "${stack}", e.stack)

	// Replace ${config.key} with config values
	configRegex := regexp.MustCompile(`\${config\.([^}]+)}`)
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

// generateName generates a final proposed name based on the configured pattern, the resource URN, and random seed.
// Note that the pattern is expected to have already had stack-level expressions like ${org}, ${project}, ${stack},
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

	// Replace ${hex(n)} with random hex string of length n
	hexRegex := regexp.MustCompile(`\${hex\((\d+)\)}`)
	result = hexRegex.ReplaceAllStringFunc(result, func(match string) string {
		n, _ := strconv.Atoi(hexRegex.FindStringSubmatch(match)[1])
		b := make([]byte, n/2+1)
		_, _ = random.Read(b)
		hasRandom = true
		return hex.EncodeToString(b)[:n]
	})

	// Replace ${alphanum(n)} with random alphanumeric string of length n
	alphaRegex := regexp.MustCompile(`\${alphanum\((\d+)\)}`)
	result = alphaRegex.ReplaceAllStringFunc(result, func(match string) string {
		n, _ := strconv.Atoi(alphaRegex.FindStringSubmatch(match)[1])
		const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
		b := make([]byte, n)
		for i := range b {
			b[i] = chars[random.Intn(len(chars))]
		}
		hasRandom = true
		return string(b)
	})

	// Replace ${string(n)} with random letter string of length n
	strRegex := regexp.MustCompile(`\${string\((\d+)\)}`)
	result = strRegex.ReplaceAllStringFunc(result, func(match string) string {
		n, _ := strconv.Atoi(strRegex.FindStringSubmatch(match)[1])
		const chars = "abcdefghijklmnopqrstuvwxyz"
		b := make([]byte, n)
		for i := range b {
			b[i] = chars[random.Intn(len(chars))]
		}
		hasRandom = true
		return string(b)
	})

	// Replace ${num(n)} with random digit string of length n
	numRegex := regexp.MustCompile(`\${num\((\d+)\)}`)
	result = numRegex.ReplaceAllStringFunc(result, func(match string) string {
		n, _ := strconv.Atoi(numRegex.FindStringSubmatch(match)[1])
		const chars = "0123456789"
		b := make([]byte, n)
		for i := range b {
			b[i] = chars[random.Intn(len(chars))]
		}
		hasRandom = true
		return string(b)
	})

	// Replace ${uuid} with random UUID
	if strings.Contains(result, "${uuid}") {
		uuidBytes := make([]byte, 16)
		_, _ = random.Read(uuidBytes)
		result = strings.ReplaceAll(result, "${uuid}", uuid.Must(uuid.FromBytes(uuidBytes)).String())
		hasRandom = true
	}

	return result, hasRandom
}
