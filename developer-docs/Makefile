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
	architecture/update.svg \
	architecture/pulumi-import.svg

%.svg: %.uml
	plantuml -tsvg $<

graphics: Makefile $(SVG_FILES)

MD_FILES = \
	architecture/deployment-schema.md \
	providers/metaschema.md

generated_markdown: Makefile $(MD_FILES)

providers/metaschema.md: utils/jsonschema2md.go ../pkg/codegen/schema/pulumi.json
	go run ./utils/jsonschema2md.go < ../pkg/codegen/schema/pulumi.json >providers/metaschema.md

architecture/deployment-schema.md: utils/jsonschema2md.go ../sdk/go/common/apitype/*.json
	go run ./utils/jsonschema2md.go -title "Deployment Schema" -ids "https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/deployments.json=-,https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/resources.json=../sdk/go/common/apitype/resources.json,https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/property-values.json=../sdk/go/common/apitype/property-values.json" <../sdk/go/common/apitype/deployments.json >architecture/deployment-schema.md


clean:
	rm -rf $(BUILDDIR)/*
