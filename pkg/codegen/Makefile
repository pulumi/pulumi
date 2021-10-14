# Re-generates codegen baselines. Running `make accept` accepts what
# the current version of codegen produces on current test
# `schema.json` files as correct expected outputs.
accept::
	cd docs && go generate
	PULUMI_IGNORE_AMBIENT_PLUGINS=1 PULUMI_ACCEPT=1 PULUMI_PARALLEL_SDK_CODEGEN_TESTS=1 go test ./...
