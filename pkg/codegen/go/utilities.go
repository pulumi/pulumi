package gen

import "github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"

// TODO: either fill out rest of providers or find more scalable solution
var providerMajorVersions = map[string]string{
	"aws":   "2",
	"gcp":   "3",
	"azure": "3",
}

func getProviderMajorVersion(pkg string) string {
	v, ok := providerMajorVersions[pkg]

	if !ok {
		contract.Failf("could not find version for package: %s", pkg)
	}

	return v
}
