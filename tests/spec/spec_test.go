package spec

import (
	"testing"

	ptesting "github.com/pulumi/pulumi/pkg/v3/testing"
)

func TestDotnet(t *testing.T) {
	ptesting.Datatest(t, "dotnet", ".")
}

func TestGo(t *testing.T) {
	ptesting.Datatest(t, "go", ".")
}
