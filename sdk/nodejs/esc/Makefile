# Build and publish the @pulumi/esc-sdk package. The esc/raw client is generated
# from ../../esc/swagger.yaml via `make generate-esc-sdk-typescript` at the repo root.
.PHONY: build
build::
	yarn install && yarn run build

.PHONY: publish
publish:: build
	cd bin && npm publish --access public
