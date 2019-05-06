# Pulumi Golang SDK

This directory contains support for writing Pulumi programs in the Go language.  There are two aspects to this:

* `pulumi/` contains the client language bindings Pulumi program's code directly against;
* `pulumi-language-go/` contains the language host plugin that the Pulumi engine uses to orchestrate updates.

To author a Pulumi program in Go, simply say so in your `Pulumi.yaml`

    name: <my-project>
    runtime: go

and ensure you have `pulumi-language-go` on your path (it is distributed in the Pulumi download automatically).

By default, the language plugin will use your project's name, `<my-project>`, as the executable that it loads.  This too
must be on your path for the language provider to load it when you run `pulumi preview` or `pulumi up`.
