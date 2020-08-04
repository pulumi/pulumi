package python

import "testing"

var pyNameTests = []struct {
	input    string
	expected string
}{
	{"kubeletConfigKey", "kubelet_config_key"},
	{"podCIDR", "pod_cidr"},
	{"podCidr", "pod_cidr"},
	{"podCIDRs", "pod_cidrs"},
	{"podCIDRSet", "pod_cidr_set"},
	{"Sha256Hash", "sha256_hash"},
	{"SHA256Hash", "sha256_hash"},
}

func TestPyName(t *testing.T) {
	for _, tt := range pyNameTests {
		t.Run(tt.input, func(t *testing.T) {
			result := PyName(tt.input)
			if result != tt.expected {
				t.Errorf("expected \"%s\"; got \"%s\"", tt.expected, result)
			}
		})
	}
}
