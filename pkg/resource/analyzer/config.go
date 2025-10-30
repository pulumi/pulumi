package analyzer

import analyzer "github.com/pulumi/pulumi/sdk/v3/pkg/resource/analyzer"

// LoadPolicyPackConfigFromFile loads the JSON config from a file.
func LoadPolicyPackConfigFromFile(file string) (map[string]plugin.AnalyzerPolicyConfig, error) {
	return analyzer.LoadPolicyPackConfigFromFile(file)
}

// ParsePolicyPackConfigFromAPI parses the config returned from the service.
func ParsePolicyPackConfigFromAPI(config map[string]*json.RawMessage) (map[string]plugin.AnalyzerPolicyConfig, error) {
	return analyzer.ParsePolicyPackConfigFromAPI(config)
}

// ValidatePolicyPackConfig validates a policy pack configuration against the specified config schema.
func ValidatePolicyPackConfig(schemaMap map[string]apitype.PolicyConfigSchema, config map[string]*json.RawMessage) (err error) {
	return analyzer.ValidatePolicyPackConfig(schemaMap, config)
}

// ReconcilePolicyPackConfig takes metadata about each policy containing default values and config schema, and
// reconciles this with the given config to produce a new config that has all default values filled-in and then sets
// configured values.
func ReconcilePolicyPackConfig(policies []plugin.AnalyzerPolicyInfo, initialConfig map[string]plugin.AnalyzerPolicyConfig, config map[string]plugin.AnalyzerPolicyConfig) (map[string]plugin.AnalyzerPolicyConfig, []string, error) {
	return analyzer.ReconcilePolicyPackConfig(policies, initialConfig, config)
}

