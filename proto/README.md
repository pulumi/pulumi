# Protobuf and gRPC interfaces

This package contains [Protobuf definitions](https://protobuf.dev) for the
various types, messages and interfaces that Pulumi components use to
communicate. These definitions serve as the source of truth for several parts of
the wider codebase.

## Code generation

:::{note}
Code generated from Protobuf files is committed to the repository. If you change
one or more `.proto` files, you should run `make build_proto` to regenerate the
necessary stubs and commit the changes as part of the same piece of work.
:::

The <gh-file:pulumi#proto/generate.sh> script in this directory (called by the
`build_proto` target in the top-level <gh-file:pulumi#Makefile>) generates types
and gRPC clients for the languages supported in this repository (Go,
NodeJS/TypeScript and Python). Generated code is committed to the repository,
typically in `proto` directories at the relevant use sites (e.g.
<gh-file:pulumi#sdk/nodejs/proto>).

## Documentation

We use the [`protoc-gen-doc`
plugin](https://github.com/pseudomuto/protoc-gen-doc) to `protoc` to generate
Markdown documentation from the Protobuf files. This process is handled by the
<gh-file:pulumi#docs/Makefile> in the `docs` directory and uses the
<gh-file:pulumi#docs/references/proto.md.tmpl> template. Generated documentation ends
up in the `docs/_generated` directory (which is `.gitignore`d), so e.g. this
index links to files in this folder.

## Index

:::{toctree}
:maxdepth: 1
:titlesonly:

/docs/_generated/proto/resource
/docs/_generated/proto/provider
/docs/_generated/proto/plugin
/docs/_generated/proto/language
/docs/_generated/proto/callback
/docs/_generated/proto/loader
/docs/_generated/proto/converter
/docs/_generated/proto/mapper
/docs/_generated/proto/analyzer
/docs/_generated/proto/alias
/docs/_generated/proto/engine
/docs/_generated/proto/errors
/docs/_generated/proto/source
:::
