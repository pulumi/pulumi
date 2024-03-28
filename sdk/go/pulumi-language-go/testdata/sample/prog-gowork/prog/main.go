package main

import (
	"example.com/dep"
	"example.com/plugin"
)

func main() {
	plugin.Foo()
	dep.Bar()
}
