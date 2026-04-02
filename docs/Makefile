.PHONY: build ensure ensure-proto format generate generate-metaschema generate-deployment-schema generate-proto watch

ensure-proto:: .ensure-proto.phony
.ensure-proto.phony:
	uv run python make.py ensure-proto
	@touch .ensure-proto.phony

ensure:: ensure-proto

generate-proto: ensure
	uv run python make.py generate-proto

generate-metaschema:
	cd utils && go run ./jsonschema2md.go \
		< ../../pkg/codegen/schema/pulumi.json \
		> ../references/metaschema.md

generate-deployment-schema:
	cd utils && go run ./jsonschema2md.go \
		-title "Deployment Schema" \
		-ids "https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/deployments.json=-,https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/resources.json=../../sdk/go/common/apitype/resources.json,https://github.com/pulumi/pulumi/blob/master/sdk/go/common/apitype/property-values.json=../../sdk/go/common/apitype/property-values.json" \
		< ../../sdk/go/common/apitype/deployments.json \
		> ../references/deployment-schema.md

generate:: generate-proto generate-metaschema generate-deployment-schema

# This target is intended to be called by Read the Docs as part of deployment,
# whereby the READTHEDOCS_OUTPUT environment variable will be set to a
# destination that Read the Docs expects to be populated with the generated
# documentation.
# generate-metaschema and generate-deployment-schema require Go, which is not
# available on Read the Docs; their outputs are committed to the repository
# instead and updated locally when the underlying schemas change.
build: ensure generate-proto
	mkdir -p $$READTHEDOCS_OUTPUT/html
	uv run python -m sphinx \
		-T \
		-c sphinx \
		-b html \
		-d _build/doctrees \
		-D language=en \
		.. \
		$$READTHEDOCS_OUTPUT/html
	cp sphinx/index.html.template $$READTHEDOCS_OUTPUT/html/index.html

watch: ensure generate
	uv run sphinx-autobuild \
		-c sphinx \
		-b html \
		--ignore "*~" \
		--ignore "*.swp" \
		--ignore ".direnv/*" \
		--ignore "*/.git/*" \
		--ignore "*/.idea/*" \
		--ignore "*/.direnv/*" \
		--ignore "_build/*" \
		.. \
		_build

format:
	uv run black .
