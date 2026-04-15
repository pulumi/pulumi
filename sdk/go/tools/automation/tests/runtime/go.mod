// This nested module keeps the generated runtime tests out of the parent
// sdk module so `go mod tidy` on sdk/go.mod doesn't need the (gitignored)
// generator output tree to exist. The outer generator regenerates the
// output tree and then runs `go test` here with the automation_runtime
// build tag.

module github.com/pulumi/pulumi/sdk/v3/go/tools/automation/tests/runtime

go 1.25.8

replace github.com/pulumi/pulumi/sdk/v3 => ../../../../..

require github.com/pulumi/pulumi/sdk/v3 v3.0.0-00010101000000-000000000000
