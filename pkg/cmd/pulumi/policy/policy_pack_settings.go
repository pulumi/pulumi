// Copyright 2026, Pulumi Corporation.
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

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/pflag"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// allPoliciesConfigKey is the configuration key that holds the pack-wide enforcement level.
const allPoliciesConfigKey = "all"

const enforcementLevelConfigKey = "enforcementLevel"

const enforcementLevelValues = "advisory, mandatory, remediate, disabled"

const remediateWarning = "warning: the remediate enforcement level only takes effect for organizations " +
	"entitled to it; for other organizations it is saved but not applied\n"

// packSettingsFlags holds the raw values of the flags that change a Policy Pack's
// enforcement levels and ESC environments within a Policy Group.
type packSettingsFlags struct {
	enforcementLevel   string
	policyEnforcement  []string
	environments       []string
	removeEnvironments []string
}

func (f *packSettingsFlags) register(flags *pflag.FlagSet) {
	flags.StringVar(&f.enforcementLevel, "enforcement-level", "",
		"Set the enforcement level of every policy in the Policy Pack, clearing per-policy enforcement levels. "+
			"One of: "+enforcementLevelValues)
	flags.StringArrayVar(&f.policyEnforcement, "policy-enforcement", nil,
		"Set the enforcement level of one policy (repeatable), applied after --enforcement-level. "+
			"Format: '<policy>=<level>', where <level> is one of: "+enforcementLevelValues)
	flags.StringArrayVar(&f.environments, "environment", nil,
		"Attach an ESC environment to the Policy Pack (repeatable). Format: '<project>/<env>[@<version|tag>]'. "+
			"Attaching an environment that is already attached does nothing")
	flags.StringArrayVar(&f.removeEnvironments, "remove-environment", nil,
		"Detach an ESC environment from the Policy Pack (repeatable). Format: '<project>/<env>[@<version|tag>]'; "+
			"without a version, any attached version of the environment is detached. "+
			"Detaching an environment that is not attached does nothing")
}

// policyLevel is a single --policy-enforcement value.
type policyLevel struct {
	policy string
	level  apitype.EnforcementLevel
}

// packSettingChanges is the parsed, validated form of packSettingsFlags.
type packSettingChanges struct {
	packLevel          apitype.EnforcementLevel
	policyLevels       []policyLevel
	addEnvironments    []string
	removeEnvironments []string
}

func (c packSettingChanges) changesConfig() bool {
	return c.packLevel != "" || len(c.policyLevels) > 0
}

func (c packSettingChanges) any() bool {
	return c.changesConfig() || len(c.addEnvironments) > 0 || len(c.removeEnvironments) > 0
}

func (c packSettingChanges) usesRemediate() bool {
	if c.packLevel == apitype.Remediate {
		return true
	}
	return slices.ContainsFunc(c.policyLevels, func(p policyLevel) bool { return p.level == apitype.Remediate })
}

func parseEnforcementLevel(flag, s string) (apitype.EnforcementLevel, error) {
	level := apitype.EnforcementLevel(s)
	if !level.IsValid() {
		return "", cmdCmd.ConfigurationError{Message: fmt.Sprintf(
			"invalid %s %q: must be one of %s", flag, s, enforcementLevelValues)}
	}
	return level, nil
}

func (f packSettingsFlags) parse() (packSettingChanges, error) {
	var c packSettingChanges
	if f.enforcementLevel != "" {
		level, err := parseEnforcementLevel("--enforcement-level", f.enforcementLevel)
		if err != nil {
			return packSettingChanges{}, err
		}
		c.packLevel = level
	}

	seen := map[string]apitype.EnforcementLevel{}
	for _, v := range f.policyEnforcement {
		policy, levelStr, ok := strings.Cut(v, "=")
		if !ok || policy == "" {
			return packSettingChanges{}, cmdCmd.ConfigurationError{Message: fmt.Sprintf(
				"invalid --policy-enforcement %q: expected <policy>=<level>", v)}
		}
		if policy == allPoliciesConfigKey {
			return packSettingChanges{}, cmdCmd.ConfigurationError{Message: fmt.Sprintf(
				"invalid --policy-enforcement %q: use --enforcement-level to set the level of every policy", v)}
		}
		level, err := parseEnforcementLevel("--policy-enforcement level", levelStr)
		if err != nil {
			return packSettingChanges{}, err
		}
		if prev, dup := seen[policy]; dup {
			if prev != level {
				return packSettingChanges{}, cmdCmd.ConfigurationError{Message: fmt.Sprintf(
					"conflicting --policy-enforcement values for policy %q: %q and %q", policy, prev, level)}
			}
			continue
		}
		seen[policy] = level
		c.policyLevels = append(c.policyLevels, policyLevel{policy: policy, level: level})
	}

	for _, ref := range f.environments {
		if err := validateEnvironmentRef("--environment", ref); err != nil {
			return packSettingChanges{}, err
		}
		if !slices.Contains(c.addEnvironments, ref) {
			c.addEnvironments = append(c.addEnvironments, ref)
		}
	}
	for _, ref := range f.removeEnvironments {
		if err := validateEnvironmentRef("--remove-environment", ref); err != nil {
			return packSettingChanges{}, err
		}
		for _, added := range c.addEnvironments {
			if environmentName(added) == environmentName(ref) {
				return packSettingChanges{}, cmdCmd.ConfigurationError{Message: fmt.Sprintf(
					"environment %q is both attached with --environment and detached with --remove-environment",
					environmentName(ref))}
			}
		}
		c.removeEnvironments = append(c.removeEnvironments, ref)
	}
	return c, nil
}

func validateEnvironmentRef(flag, ref string) error {
	name, version, hasVersion := strings.Cut(ref, "@")
	project, env, ok := strings.Cut(name, "/")
	if !ok || project == "" || env == "" || strings.Contains(env, "/") || (hasVersion && version == "") {
		return cmdCmd.ConfigurationError{Message: fmt.Sprintf(
			"invalid %s %q: expected <project>/<env>[@<version|tag>]", flag, ref)}
	}
	return nil
}

// environmentName strips any @version suffix from an environment reference.
func environmentName(ref string) string {
	name, _, _ := strings.Cut(ref, "@")
	return name
}

// applyEnvironmentChanges returns envs with removals applied, then additions. A reference
// without a version removes every attached version of that environment, and adding an
// environment that is attached at another version replaces it in place.
func applyEnvironmentChanges(envs []string, c packSettingChanges) []string {
	result := slices.Clone(envs)
	for _, ref := range c.removeEnvironments {
		result = slices.DeleteFunc(result, func(e string) bool {
			if strings.Contains(ref, "@") {
				return e == ref
			}
			return environmentName(e) == ref
		})
	}
	for _, ref := range c.addEnvironments {
		idx := slices.IndexFunc(result, func(e string) bool { return environmentName(e) == environmentName(ref) })
		if idx < 0 {
			result = append(result, ref)
		} else {
			result[idx] = ref
		}
	}
	return result
}

// policyConfigObject decodes one configuration value. Values are either an object or,
// in older configurations, a bare enforcement level string.
func policyConfigObject(raw *json.RawMessage) (map[string]any, error) {
	if raw == nil {
		return map[string]any{}, nil
	}
	var v any
	if err := json.Unmarshal(*raw, &v); err != nil {
		return nil, err
	}
	switch v := v.(type) {
	case nil:
		return map[string]any{}, nil
	case string:
		return map[string]any{enforcementLevelConfigKey: v}, nil
	case map[string]any:
		return v, nil
	default:
		return nil, fmt.Errorf("expected an object or string, got %s", string(*raw))
	}
}

func marshalPolicyConfigObject(obj map[string]any) (*json.RawMessage, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(b)
	return &raw, nil
}

// applyConfigChanges returns config with the enforcement level changes applied. The
// pack-wide level is written to "all" and removes every per-policy enforcementLevel,
// keeping the policies' other settings; per-policy levels are applied afterwards.
func applyConfigChanges(
	config map[string]*json.RawMessage, c packSettingChanges,
) (map[string]*json.RawMessage, error) {
	if !c.changesConfig() {
		return config, nil
	}
	result := maps.Clone(config)
	if result == nil {
		result = map[string]*json.RawMessage{}
	}

	if c.packLevel != "" {
		for key, raw := range result {
			if key == allPoliciesConfigKey {
				continue
			}
			obj, err := policyConfigObject(raw)
			if err != nil {
				return nil, fmt.Errorf("reading configuration for policy %q: %w", key, err)
			}
			if _, has := obj[enforcementLevelConfigKey]; !has {
				continue
			}
			delete(obj, enforcementLevelConfigKey)
			if len(obj) == 0 {
				delete(result, key)
				continue
			}
			if result[key], err = marshalPolicyConfigObject(obj); err != nil {
				return nil, err
			}
		}
		all, err := policyConfigObject(result[allPoliciesConfigKey])
		if err != nil {
			return nil, fmt.Errorf("reading configuration for %q: %w", allPoliciesConfigKey, err)
		}
		all[enforcementLevelConfigKey] = string(c.packLevel)
		if result[allPoliciesConfigKey], err = marshalPolicyConfigObject(all); err != nil {
			return nil, err
		}
	}

	for _, p := range c.policyLevels {
		obj, err := policyConfigObject(result[p.policy])
		if err != nil {
			return nil, fmt.Errorf("reading configuration for policy %q: %w", p.policy, err)
		}
		obj[enforcementLevelConfigKey] = string(p.level)
		if result[p.policy], err = marshalPolicyConfigObject(obj); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// dropUnknownPolicyConfig removes configuration keys for policies that are not in
// policies, returning the remaining configuration and the sorted removed keys.
func dropUnknownPolicyConfig(
	config map[string]*json.RawMessage, policies []cloudapitype.AppPolicy,
) (map[string]*json.RawMessage, []string) {
	known := make(map[string]bool, len(policies))
	for _, p := range policies {
		known[p.Name] = true
	}
	var dropped []string
	result := make(map[string]*json.RawMessage, len(config))
	for key, raw := range config {
		if key == allPoliciesConfigKey || known[key] {
			result[key] = raw
			continue
		}
		dropped = append(dropped, key)
	}
	slices.Sort(dropped)
	return result, dropped
}

func hasPolicyConfigKeys(config map[string]*json.RawMessage) bool {
	for key := range config {
		if key != allPoliciesConfigKey {
			return true
		}
	}
	return false
}

// packVersionTag returns the version tag of an applied pack, falling back to its version
// number for packs published without a tag.
func packVersionTag(p cloudapitype.AppPolicyPackMetadata) string {
	if p.VersionTag != "" {
		return p.VersionTag
	}
	if p.Version != 0 {
		return strconv.Itoa(p.Version)
	}
	return ""
}

func findAppliedPack(group cloudapitype.PolicyGroup, name string) *cloudapitype.AppPolicyPackMetadata {
	idx := slices.IndexFunc(group.AppliedPolicyPacks, func(p cloudapitype.AppPolicyPackMetadata) bool {
		return p.Name == name
	})
	if idx < 0 {
		return nil
	}
	return &group.AppliedPolicyPacks[idx]
}

// packSettingsClient is the subset of cloud-API operations needed to look up the policies
// of the Policy Pack version being enabled.
type packSettingsClient interface {
	ListPolicyPacks(
		ctx context.Context, orgName string, inContToken *string,
	) (cloudapitype.AppListPolicyPacksResponse, *string, error)
	GetPolicyPack(
		ctx context.Context, orgName, name, versionTag string,
	) (cloudapitype.AppGetPolicyPackResponse, error)
}

func latestPolicyPackVersionTag(ctx context.Context, c packSettingsClient, org, name string) (string, error) {
	var packs []cloudapitype.AppPolicyPackWithVersions
	var token *string
	for {
		resp, next, err := c.ListPolicyPacks(ctx, org, token)
		if err != nil {
			return "", fmt.Errorf("listing policy packs: %w", err)
		}
		packs = append(packs, resp.PolicyPacks...)
		if next == nil || *next == "" {
			break
		}
		token = next
	}
	return resolveLatestVersionTag(packs, org, name)
}

// packMergeInput describes the write being made for one Policy Pack in a Policy Group.
type packMergeInput struct {
	// current is the pack as currently applied to the group, or nil if it is not applied.
	current *cloudapitype.AppPolicyPackMetadata
	// targetVersionTag is the version being applied; empty means the latest version.
	targetVersionTag string
	// replaceConfig, when non-nil, replaces the pack's configuration wholesale.
	replaceConfig map[string]*json.RawMessage
	changes       packSettingChanges
}

// packMergeResult is the configuration and environments to send for the pack.
type packMergeResult struct {
	config            map[string]*json.RawMessage
	environments      []string
	droppedConfigKeys []string
	// policies of the version being applied, when they were fetched, at policiesVersionTag.
	policies           []cloudapitype.AppPolicy
	policiesVersionTag string
}

// mergePackSettings builds the configuration and environments for applying a Policy Pack
// to a Policy Group. The service replaces both wholesale on every write, so this starts
// from what is currently applied and changes only what was asked for.
func mergePackSettings(
	ctx context.Context, c packSettingsClient, org, name string, in packMergeInput,
) (packMergeResult, error) {
	var res packMergeResult
	if in.replaceConfig != nil {
		res.config = in.replaceConfig
	} else if in.current != nil {
		res.config = maps.Clone(in.current.Config)
	}
	if in.current != nil {
		res.environments = slices.Clone(in.current.Environments)
	}
	res.environments = applyEnvironmentChanges(res.environments, in.changes)

	mayDropStale := in.current != nil && in.replaceConfig == nil && hasPolicyConfigKeys(res.config)
	target := in.targetVersionTag
	if target == "" && (mayDropStale || len(in.changes.policyLevels) > 0) {
		latest, err := latestPolicyPackVersionTag(ctx, c, org, name)
		if err != nil {
			return packMergeResult{}, err
		}
		target = latest
	}
	dropStale := mayDropStale && target != packVersionTag(*in.current)

	if dropStale || len(in.changes.policyLevels) > 0 {
		pack, err := c.GetPolicyPack(ctx, org, name, target)
		if err != nil {
			return packMergeResult{}, err
		}
		res.policies, res.policiesVersionTag = pack.Policies, target
	}

	if dropStale {
		res.config, res.droppedConfigKeys = dropUnknownPolicyConfig(res.config, res.policies)
	}

	for _, p := range in.changes.policyLevels {
		if !slices.ContainsFunc(res.policies, func(ap cloudapitype.AppPolicy) bool { return ap.Name == p.policy }) {
			return packMergeResult{}, cmdCmd.ConfigurationError{Message: fmt.Sprintf(
				"invalid --policy-enforcement: policy %q is not in version %s of policy pack %q",
				p.policy, target, name)}
		}
	}

	config, err := applyConfigChanges(res.config, in.changes)
	if err != nil {
		return packMergeResult{}, err
	}
	res.config = config
	return res, nil
}

// warnPackSettings writes warnings about the merge to w.
func warnPackSettings(w io.Writer, name string, changes packSettingChanges, res packMergeResult) {
	if changes.usesRemediate() {
		fmt.Fprint(w, remediateWarning)
	}
	if len(res.droppedConfigKeys) > 0 {
		fmt.Fprintf(w, "warning: removed configuration for policies not in version %s of policy pack %q: %s\n",
			res.policiesVersionTag, name, strings.Join(res.droppedConfigKeys, ", "))
	}
}
