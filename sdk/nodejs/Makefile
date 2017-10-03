PROCCNT=$(shell nproc --all)

.PHONY: default
default: banner build test install

.PHONY: banner
banner:
	@echo "\033[1;37m==================\033[0m"
	@echo "\033[1;37mPulumi Node.js SDK\033[0m"
	@echo "\033[1;37m==================\033[0m"

.PHONY: configure
configure:
	yarn install $(YARNFLAGS)
	cd runtime/native && ./ensure_node_v8.sh
	cd runtime/native && ../../node_modules/.bin/node-gyp configure

.PHONY: lint
lint:
	@echo "\033[0;32mLINT:\033[0m"
	yarn run lint $(YARNFLAGS)

.PHONY: clean
clean:
	rm -rf ./bin

.PHONY: build
build:
	@echo "\033[0;32mBUILD:\033[0m"
	yarn run build $(YARNFLAGS)

.PHONY: test
test:
	@echo "\033[0;32mTEST:\033[0m"
	yarn run test $(YARNFLAGS)

.PHONY: install
install:
	@echo "\033[0;32mINSTALL:\033[0m"
	cp package.json bin/
	cd bin/ && yarn link $(YARNFLAGS)

