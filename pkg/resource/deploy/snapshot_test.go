package deploy

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func createSnapshot() Snapshot {
	resourceUrns := []resource.URN{
		resource.NewURN("stack", "test", "typ", "aws:resource", "bar"),
		resource.NewURN("stack", "test", "typ", "aws:resource", "aname"),
		resource.NewURN("stack", "test", "typ", "azure:resource", "bar"),
	}
	resources := []*resource.State{}
	for _, u := range resourceUrns {
		resources = append(resources, &resource.State{URN: u})
	}
	return Snapshot{Resources: resources}
}

func TestGlobUrn(t *testing.T) {
	t.Parallel()

	snap := createSnapshot()

	globs := []struct {
		input    string
		expected []resource.URN
	}{
		{
			input: "**",
			expected: []resource.URN{
				"urn:pulumi:stack::test::typ$aws:resource::aname",
				"urn:pulumi:stack::test::typ$aws:resource::bar",
				"urn:pulumi:stack::test::typ$azure:resource::bar",
			},
		},
		{
			input: "urn:pulumi:stack::test::typ*:resource::bar",
			expected: []resource.URN{
				"urn:pulumi:stack::test::typ$aws:resource::bar",
				"urn:pulumi:stack::test::typ$azure:resource::bar",
			},
		},
		{
			input:    "**:aname",
			expected: []resource.URN{"urn:pulumi:stack::test::typ$aws:resource::aname"},
		},
		{
			input: "*:*:stack::test::typ$aws:resource::*",
			expected: []resource.URN{
				"urn:pulumi:stack::test::typ$aws:resource::aname",
				"urn:pulumi:stack::test::typ$aws:resource::bar",
			},
		},
		{
			input:    "stack::test::typ$aws:resource::none",
			expected: []resource.URN{"stack::test::typ$aws:resource::none"},
		},
	}
	for _, tt := range globs {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			actual := snap.GlobUrn(resource.URN(tt.input))
			assert.Equal(t, tt.expected, actual)
		})
	}
}
