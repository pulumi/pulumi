BASE=$(realpath ./)
INSTALL=/usr/local/bin

.PHONY: default
default: banner build test

.PHONY: banner
banner:
	@echo "\033[1;37m===============\033[0m"
	@echo "\033[1;37mLumiJS Compiler\033[0m"
	@echo "\033[1;37m===============\033[0m"

.PHONY: clean
clean:
	@rm -rf ./bin
	@rm -rf ${INSTALL}

.PHONY: build
build:
	@echo "\033[0;32mBUILD:\033[0m"
	@yarn run build

.PHONY: install
install:
	@echo "\033[0;32mINSTALL:\033[0m"
	@ln -sf ${BASE}/lumijs ${INSTALL}/lumijs

.PHONY: test
test:
	@echo "\033[0;32mTEST:\033[0m"
	@yarn run test

.PHONY: verify
verify: gen
	@$(shell git diff --quiet .)

