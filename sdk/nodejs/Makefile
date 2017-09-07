PROCCNT=$(shell nproc --all)

.PHONY: default
default: banner build install

.PHONY: banner
banner:
	@echo "\033[1;37m==================\033[0m"
	@echo "\033[1;37mPulumi Node.js SDK\033[0m"
	@echo "\033[1;37m==================\033[0m"

.PHONY: lint
lint:
	@echo "\033[0;32mLINT:\033[0m"
	yarn run lint

.PHONY: clean
clean:
	rm -rf ./bin

.PHONY: build
build:
	@echo "\033[0;32mBUILD:\033[0m"
	yarn run build

.PHONY: install
install:
	@echo "\033[0;32mINSTALL:\033[0m"
	cp package.json bin/
	cd bin/ && yarn link

