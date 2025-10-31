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
	"regexp"
	"strconv"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"lukechampine.com/frand"
)

// Autonamer resolves custom autonaming options for a given resource URN.
type Autonamer interface {
	// AutonamingForResource returns the autonaming options for a resource, and whether it
	// should be required to be deleted before creating.
	AutonamingForResource(urn urn.URN, randomSeed []byte) (opts *plugin.AutonamingOptions, deleteBeforeReplace bool)
}

type defaultAutonaming struct{}

// defaultAutonamingConfig is the default instance of defaultAutonaming.
var defaultAutonamingConfig = &defaultAutonaming{}

// Default is the default autonaming strategy, which is equivalent to
// no custom autonaming.
func Default() Autonamer {
	return defaultAutonamingConfig
}

func (a *defaultAutonaming) AutonamingForResource(urn.URN, []byte,
) (opts *plugin.AutonamingOptions, deleteBeforeReplace bool) {
	return nil, false
}

type verbatimAutonaming struct{}

// Verbatim is an autonaming config that enforces the use of a
// logical resource name as the physical resource name literally, with no transformations.
func Verbatim() Autonamer {
	return verbatimAutonaming{}
}

func (verbatimAutonaming) AutonamingForResource(urn urn.URN, _ []byte,
) (opts *plugin.AutonamingOptions, deleteBeforeReplace bool) {
	return &plugin.AutonamingOptions{
		ProposedName:	urn.Name(),
		Mode:		plugin.AutonamingModeEnforce,
	}, true
}

type disabledAutonaming struct{}

// Disabled is an autonaming config that disables autonaming altogether.
func Disabled() Autonamer {
	return disabledAutonaming{}
}

func (disabledAutonaming) AutonamingForResource(urn.URN, []byte,
) (opts *plugin.AutonamingOptions, deleteBeforeReplace bool) {
	return &plugin.AutonamingOptions{
		Mode: plugin.AutonamingModeDisabled,
	}, true
}

// Pattern is an autonaming config that uses a pattern to generate a name.
type Pattern struct {
	// Pattern is the pattern to use to generate the name.
	Pattern	string
	// Enforce, if true, will enforce the use of the generated name, as opposed to proposing it.
	// A proposed name can still be overridden by the provider, while an enforced name cannot.
	Enforce	bool
}

func (a *Pattern) AutonamingForResource(urn urn.URN, randomSeed []byte,
) (opts *plugin.AutonamingOptions, deleteBeforeReplace bool) {
	mode := plugin.AutonamingModePropose
	if a.Enforce {
		mode = plugin.AutonamingModeEnforce
	}
	proposedName, hasRandom := generateName(a.Pattern, urn, randomSeed)
	return &plugin.AutonamingOptions{
		ProposedName:		proposedName,
		Mode:			mode,
		WarnIfNoSupport:	a.Enforce,
	}, !hasRandom
}

// Provider represents the configuration for a provider
type Provider struct {
	// Default is the default autonaming config for the provider unless overridden by a more specific
	// resource config.
	Default	Autonamer

	// Resources maps resource types to their specific configurations
	// Key format: provider:module:type (e.g., "aws:s3/bucket:Bucket")
	Resources	map[string]Autonamer
}

// Global represents the root configuration object for Pulumi autonaming
type Global struct {
	// Default is the default autonaming config for all the providers unless overridden by a more specific
	// provider config.
	Default	Autonamer

	// Providers maps provider names to their configurations
	// Key format: provider name (e.g., "aws")
	Providers	map[string]Provider
}

func (o *Global) pluginOptionsForResourceType(resourceType tokens.Type) (Autonamer, bool) {
	token := resourceType.String()
	provider := resourceType.Package().Name().String()

	// Check type-specific config
	if pConfig, ok := o.Providers[provider]; ok {
		if rConfig, ok := pConfig.Resources[token]; ok {
			return rConfig, false
		}
		if pConfig.Default != nil {
			return pConfig.Default, false
		}
	}
	// Fall back to global config
	if o.Default != nil {
		return o.Default, true
	}
	return defaultAutonamingConfig, true
}

// AutonamingForResource returns the autonaming options for a resource, and whether it should be required
// to be deleted before creating. The proper configuration is resolved by looking at the resource type
// and its provider, and falling back to the global default if no specific configuration is found.
// If the strategy returns nil, it means the user hasn't overridden the default autonaming for this resource.
func (o *Global) AutonamingForResource(urn urn.URN, randomSeed []byte) (*plugin.AutonamingOptions, bool) {
	naming, isTopLevelOrDefault := o.pluginOptionsForResourceType(urn.Type())
	opts, deleteBeforeReplace := naming.AutonamingForResource(urn, randomSeed)
	if opts == nil {
		// If the strategy returns nil, it means the user hasn't overridden the default autonaming for this resource.
		return nil, false
	}

	if !isTopLevelOrDefault {
		// If the strategy comes from a provider- or resource-specific config, it's specific enough that the user
		// definitely intended it to apply to this resource. Therefore, we should always warn if it turns out
		// the provider doesn't actually support autonaming customization.
		opts.WarnIfNoSupport = true
	}
	return opts, deleteBeforeReplace
}

var (
	hexRegex	= regexp.MustCompile(`\${hex\((\d+)\)}`)
	alphaRegex	= regexp.MustCompile(`\${alphanum\((\d+)\)}`)
	strRegex	= regexp.MustCompile(`\${string\((\d+)\)}`)
	numRegex	= regexp.MustCompile(`\${num\((\d+)\)}`)
)

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

// Regexes for resolving expressions in patterns.
var randomExpressionReplacers = []func(string, *frand.RNG) string{
	replaceHex,
	replaceAlphanum,
	replaceString,
	replaceNum,
	replaceUUID,
}

// generateName generates a final proposed name based on the configured pattern, the resource URN, and random seed.
// Note that the pattern is expected to have already had stack-level expressions like ${organization}, ${project},
// ${stack}, and ${config.key} resolved before passing to this function.
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
		bufsize := 1024	// Same bufsize as used by frand.New.
		rounds := 12	// Same rounds as used by frand.New.
		random = frand.NewCustom(seed, bufsize, rounds)
	}

	for _, replacer := range randomExpressionReplacers {
		newResult := replacer(result, random)
		hasRandom = hasRandom || newResult != result
		result = newResult
	}

	return result, hasRandom
}
