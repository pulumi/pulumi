.PHONY: build ensure ensure-proto ensure-python generate generate-proto watch

VENV = venv

ensure-python:: .ensure-python.phony
.ensure-python.phony: requirements.txt
	../scripts/retry python -m $(VENV) $(VENV)
	. $(VENV)/*/activate && ../scripts/retry python -m pip install --upgrade pip setuptools wheel
	. $(VENV)/*/activate && ../scripts/retry python -m pip install -r requirements.txt
	@touch .ensure-python.phony

ensure-proto:: .ensure-proto.phony
.ensure-proto.phony: ensure-python
	. $(VENV)/*/activate && python make.py ensure-proto
	@touch .ensure-proto.phony

ensure:: ensure-proto ensure-python

generate-proto: ensure
	. $(VENV)/*/activate && python make.py generate-proto

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
	. $(VENV)/*/activate && python -m sphinx \
		-T \
		-c sphinx \
		-b html \
		-d _build/doctrees \
		-D language=en \
		.. \
		$$READTHEDOCS_OUTPUT/html
	cp sphinx/index.html.template $$READTHEDOCS_OUTPUT/html/index.html

watch: ensure generate
	. $(VENV)/*/activate && sphinx-autobuild \
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
	. $(VENV)/*/activate && black .
