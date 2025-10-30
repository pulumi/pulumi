package python

import python "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/python"

// PypiVersion translates semver 2.0 into pypi's versioning scheme:
// Details can be found here: https://www.python.org/dev/peps/pep-0440/#version-scheme
// [N!]N(.N)*[{a|b|rc}N][.postN][.devN]
func PypiVersion(v semver.Version) string {
	return python.PypiVersion(v)
}

