package main

import (
	"github.com/pulumi/go-dependency-testdata/dep"
	"github.com/pulumi/go-dependency-testdata/plugin"
)

func main() {
	plugin.Foo()
	dep.Bar()
}
