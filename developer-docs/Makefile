SPHINXOPTS    ?=
SPHINXBUILD   ?= sphinx-build
SOURCEDIR     = .
BUILDDIR      = _build

html: Makefile graphics generated_markdown
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

pdf: Makefile graphics generated_markdown
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

.PHONY: graphics generated_markdown clean Makefile

SVG_FILES = \
	providers/resource-lifecycle.svg \
	architecture/construct.svg \
	architecture/create.svg \
	architecture/delete-before-replace-graph.svg \
	architecture/delete-before-replace.svg \
	architecture/import.svg \
	architecture/replace.svg \
	architecture/same.svg \
	architecture/update.svg

%.svg: %.uml
	plantuml -tsvg $<

graphics: Makefile $(SVG_FILES)

MD_FILES = \
	providers/metaschema.md

generated_markdown: Makefile $(MD_FILES)

providers/metaschema.md: utils/jsonschema2md.go ../pkg/codegen/schema/pulumi.json
	go run ./utils/jsonschema2md.go < ../pkg/codegen/schema/pulumi.json >providers/metaschema.md

clean:
	rm -rf $(BUILDDIR)/*
