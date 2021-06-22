package tests

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
)

func TestLogin(t *testing.T) {

	t.Run("RespectsEnvVar", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer deleteIfNotFailed(e)

		integration.CreateBasicPulumiRepo(e)

		// Running pulumi logout --all twice shouldn't result in an error
		e.RunCommand("pulumi", "logout", "--all")
		e.RunCommand("pulumi", "logout", "--all")
	})
}
