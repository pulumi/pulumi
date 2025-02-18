(dev)=
# Development

(devenv)=
## Tools and environment

This repository makes use of a number of tools. At a minimum, you'll want to
install the following on your machine:

- [Go](https://go.dev/dl/), for building and running the code in this
  repository, including the Go SDK.
- [NodeJS](https://nodejs.org/en/download/), for working with the NodeJS SDK.
- [Python](https://www.python.org/downloads/), for working with the Python SDK.
- [.NET](https://dotnet.microsoft.com/download), for working with the .Net SDK.
- [Golangci-lint](https://github.com/golangci/golangci-lint), for linting Go
  code.
- [gofumpt](https://github.com/mvdan/gofumpt) for formatting Go code. See
  [installation](https://github.com/mvdan/gofumpt#installation) for editor setup
  instructions.
- [Yarn](https://yarnpkg.com/), for building and working with the NodeJS SDK.
- [Pulumictl](https://github.com/pulumi/pulumictl)
- [jq](https://stedolan.github.io/jq/)

For consistency and ease of use, this repository provides a
[](gh-file:pulumi#.mise.toml) file for configuring [Mise](https://mise.jdx.dev),
a tool for managing development environments (similar in nature to `direnv` or
`asdf`). To use it, you only need to install Mise and activate the environment,
which you can do as follows:

1. [Install
   Mise](https://mise.jdx.dev/getting-started.html#installing-mise-cli).
2. Configure your shell to [activate
   Mise](https://mise.jdx.dev/getting-started.html#activate-mise). This is
   typically accomplished by adding an appropriate invocation of `mise activate`
   to your shell's configuration file (e.g. `.bashrc`, `.zshrc`, etc.).
3. Restart your shell session so that your configuration changes take effect.
4. `cd` into the root of this repository. You should find that the tools you
   need are now available in your `PATH`. If not, running `mise install` should
   sort things out.

Use of Mise is currently experimental and optional.
