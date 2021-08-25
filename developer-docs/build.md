# Building the Docs

In order to build the devloper documentation:

1. Install [PlantUML](https://plantuml.com). On macOS, this can be done via `brew install plantuml`.
2. Install the requirements for Sphinx:

	```bash
	$ pip install requirements.txt
	```

3. Run `make` to build the HTML documentation:

	```bash
	$ make
	```

This will regenerate any out-of-date SVGs and build the a local version of the HTML documentation. The documentation
can also be built from the repository root by running `make developer_docs`.
