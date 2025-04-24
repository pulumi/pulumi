module example.com/prog

go 1.18

require (
	github.com/pulumi/go-dependency-testdata/dep v1.6.0
	github.com/pulumi/go-dependency-testdata/plugin v1.2.3
)

require github.com/pulumi/go-dependency-testdata/indirect-dep/v2 v2.1.0 // indirect
