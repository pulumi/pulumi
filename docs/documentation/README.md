# Documentation

Documentation is built with [Sphinx](https://www.sphinx-doc.org) and authored in
Markdown as much as possible. Markdown support for Sphinx is provided by
[MyST](https://myst-parser.readthedocs.io), which provides a number of
extensions to the Markdown dialect you may be used to in order to take advantage
of various Sphinx features.

## Workflow

### Building

The `docs` directory contains a `Makefile` (<gh-file:pulumi#docs/Makefile>) for
working with the documentation. Generally you'll just want:

```sh
make -C docs watch
```

which will watch for changes and rebuild the documentation as needed, serving
the results at <http://localhost:8000/docs/README.html>.

### Deployment

This documentation is deployed to [Read the Docs](https://readthedocs.org) as
part of our CI/CD pipelines. The `build` target in the `Makefile` is used to
build an HTML version of the documentation that Read the Docs then deploys. See
the <gh-file:pulumi#.readthedocs.yaml> configuration in the root of the
repository for more information.

#### Previewing changes

If you want to preview the documentation as it will appear on Read the Docs,
simply raise a PR with your changes. Read the Docs will build your PR and
generate a preview site for you to review. The link to the preview site will
appear in the list of checks on your PR. See [Read the Docs' documentation on
preview builds](https://docs.readthedocs.io/en/stable/pull-requests.html) for
more.

#### Local builds

If you want to build the documentation without watching for changes, you can use
the `build` target yourself locally, too. Set the `READTHEDOCS_OUTPUT`
environment variable to the location you'd like the documentation to be built to
and then run the build:

```sh
export READTHEDOCS_OUTPUT=/path/to/output
make -C docs build
```

`build` will compile documentation to a static HTML website in the
`/path/to/output/html` directory; you can open the `index.html` file in your
browser to view the built files from there:

```sh
open /path/to/output/html/index.html
```

:::{toctree}
:maxdepth: 1
:titlesonly:

/docs/documentation/writing
/docs/documentation/diagrams
:::
