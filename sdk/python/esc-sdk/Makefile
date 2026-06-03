# Build and publish the pulumi-esc-sdk distribution. The pulumi_esc_sdk package is
# generated from ../../esc/swagger.yaml via `make generate-esc-sdk-python` at the
# repo root.
PYTHON ?= python3

.PHONY: build
build::
	$(PYTHON) -m build

.PHONY: publish
publish:: build
	$(PYTHON) -m twine upload dist/*
