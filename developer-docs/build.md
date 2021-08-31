# Building the Docs

This documentation is generated using [Sphinx] and authored in Markdown. Markdown support for [Sphinx] is provided by
[MyST]. [MyST] provides a number of small syntax extensions to support declaring ReStructuredText directives; see
[the MyST syntax guide](https://myst-parser.readthedocs.io/en/latest/syntax/syntax.html) for details.

In order to build the devloper documentation:

1. Install [PlantUML]. On macOS, this can be done via `brew install plantuml`.
2. Install the requirements for [Sphinx]:

	```bash
	$ pip install requirements.txt
	```

3. Run `make` to build the HTML documentation:

	```bash
	$ make
	```

This will regenerate any out-of-date SVGs and build the a local version of the HTML documentation. The documentation
can also be built from the repository root by running `make developer_docs`.

Note that Sphinx doesn't do a great job of rebuilding output files if only the table-of-contents has changed. If you
change the table of contents, you may need to clean the output directory in order to see the effects of your changes:

	```bash
	$ make clean
	```

## Notes on Style

- Do use appropriate links wherever possible. Learn to use [header anchors](https://myst-parser.readthedocs.io/en/latest/syntax/optional.html#auto-generated-header-anchors).
- If a particular link destination is referenced multiple times, prefer [shortcut reference links](https://spec.commonmark.org/0.29/#shortcut-reference-link).

[Sphinx]: https://www.sphinx-doc.org/en/master/index.html
[MyST]: https://myst-parser.readthedocs.io/en/latest/index.html
[PlantUML]: https://plantuml.com
