SPHINXOPTS    ?=
SPHINXBUILD   ?= sphinx-build
SOURCEDIR     = .
BUILDDIR      = _build

html: Makefile graphics
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

pdf: Makefile graphics
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

.PHONY: graphics clean Makefile

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

clean:
	rm -rf $(BUILDDIR)/*
