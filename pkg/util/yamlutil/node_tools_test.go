package yamlutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func assertYaml(t *testing.T, before, after string, mutation func(node *yaml.Node) error) {
	t.Helper()
	var beforeNode yaml.Node
	err := yaml.Unmarshal([]byte(before), &beforeNode)
	assert.NoError(t, err)
	err = mutation(&beforeNode)
	assert.NoError(t, err)
	afterBytes, err := yaml.Marshal(beforeNode.Content[0])
	assert.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(after), strings.TrimSpace(string(afterBytes)))
}

func TestInsertNode(t *testing.T) {
	t.Parallel()

	assertYaml(t, `
foo: baz
`, `
foo: bar
`, func(node *yaml.Node) error { return Insert(node, "foo", "bar") })
}

func TestInsertNodeNew(t *testing.T) {
	t.Parallel()

	assertYaml(t, `
# comment
existing: node # comment
`, `
# comment
existing: node # comment
foo: bar
`, func(node *yaml.Node) error { return Insert(node, "foo", "bar") })
}

func TestInsertNodeOverwrite(t *testing.T) {
	t.Parallel()

	assertYaml(t, `
foo: 1
# header
bar: 2 # this should become 42
# trailer
quux: 3
`, `
foo: 1
# header
bar: 42
# trailer
quux: 3
`, func(node *yaml.Node) error { return Insert(node, "bar", "42") })
}

func TestDeleteNode(t *testing.T) {
	t.Parallel()

	assertYaml(t, `
foo: 1
# header
bar: 2 # this should become 42
# trailer
quux: 3
`, `
foo: 1
# trailer
quux: 3
`, func(node *yaml.Node) error { return Delete(node, "bar") })
}
