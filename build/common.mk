# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

# common.mk provides most of the scaffolding for our build system. It
# provides default targets for each project we want to build.
#
# The default targets we use are:
#
#  - ensure: restores any dependencies needed for the build from
#            remote sources (e.g dep ensure or yarn install)
#
#  - build: builds a project but does not install it.
#
#  - install: copies the bits we plan to ship into a layout in
#             `PULUMI_ROOT` that looks like what a customer would get
#             when they download and install Pulumi. For JavaScript
#             projects, installing also runs yarn link to register
#             this package, so that other projects can depend on it.
#
#  - lint: runs relevent linters for the project
#
#  - test_fast: runs the fast tests for a project. These are often
#               go unit tests or javascript unit tests, they should
#               complete quickly, as we expect developers to run them
#               fequently as part of their "inner loop" development.
#
#  - test_all: runs all of test_fast and then runs additional testing,
#              which may take longer (some times a lot longer!). These
#              are often integration tests which will use `pulumi` to
#              deploy example Pulumi projects, creating cloud
#              resources along the way.
#
# In addition, we have a few higher level targets that just depend on
# these targets:
#
#  - only_build: this target runs build and install targets
#
#  - only_test: this target runs the list and test_all targets
#               (test_all itself runs test_fast)
#
#  - default: this is the target that is run by default when no
#             arguments are passed to make, it runs the build, lint,
#             install and test_fast targets
#
#  - core: this target behaves like `default` except for the case
#          where a project declares SUB_PROJECTS (see a discussion on
#          that later). In that case, building `core` target does not
#          build sub projects.
#
#  - all: this target runs build, lint, install and test_all (which
#         itself runs test_fast).
#
# Before including this makefile, a project may define some values
# that this makefile understands:
#
# - PROJECT_NAME: If set, make default and make all will print a banner
#                 with the project name when they are built.
#
# - SUB_PROJECTS: If set, each item in the list is treated as a path
#                 to another project (relative to the directory of the
#                 main Makefile) which should be built as well. When
#                 this happens, the default and all targets first
#                 build the default or all target of each child
#                 project. For each subproject we also create targets
#                 with our standard names, prepended by the target
#                 name and an underscore, which just calls Make for
#                 that specific target. These can be handy targets to
#                 build explicitly on the command line from time to
#                 time.
#
# - NODE_MODULE_NAME: If set, an install target will be auto-generated
#                     that installs the module to
#                     $(PULUMI_ROOT)/node_modules/$(NODE_MODULE_NAME)
#
# This Makefile also provides some convience methods:
#
# STEP_MESSAGE is a macro that can be invoked with `$(call
# STEP_MESSAGE)` and it will print the name of the current target (in
# green text) to the console. All the targets provided by this makefile
# do that by default.
#
# Importing common.mk should be the first thing your Makefile does, after
# optionally setting SUB_PROJECTS, PROJECT_NAME and NODE_MODULE_NAME.
SHELL       ?= /bin/bash
.SHELLFLAGS := -ec

STEP_MESSAGE = @printf "\033[0;32m$(shell echo '$@' | tr a-z A-Z | tr '_' ' '):\033[0m\n"

# Our install targets place items item into $PULUMI_ROOT.
PULUMI_ROOT ?= $$HOME/.pulumi-dev

# Use Python 3 explicitly vs expecting that `python` will resolve to a python 3
# runtime.
PYTHON ?= python3

ROOT_DIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

PULUMI_BIN          := $(PULUMI_ROOT)/bin
PULUMI_NODE_MODULES := $(PULUMI_ROOT)/node_modules

# Extra options to pass to `go test` command, for example:
#
#     make GO_TEST_OPTIONS="-short -test.v" test_fast
GO_TEST_OPTIONS :=

GO_TEST_PARALLELISM     ?= 10   # -parallel, number of parallel tests to run within a package
GO_TEST_PKG_PARALLELISM ?= 2    # -p flag, number of parallel packages to test
GO_TEST_SHUFFLE         ?= off  # -shuffle flag, randomizes order of tests within a package
GO_TEST_TAGS            ?= all
GO_TEST_RACE            ?= true

GO_TEST_FLAGS = -count=1 -cover -tags="${GO_TEST_TAGS}" -timeout 1h \
	-parallel=${GO_TEST_PARALLELISM} \
	-shuffle=${GO_TEST_SHUFFLE} \
	-p=${GO_TEST_PKG_PARALLELISM} \
	-race=${GO_TEST_RACE} \
	${GO_TEST_OPTIONS}
GO_TEST_FAST_FLAGS = -short ${GO_TEST_FLAGS}

GO_TEST      = $(PYTHON) $(ROOT_DIR)/../scripts/go-test.py $(GO_TEST_FLAGS)
GO_TEST_FAST = $(PYTHON) $(ROOT_DIR)/../scripts/go-test.py $(GO_TEST_FAST_FLAGS)

GOPROXY = https://proxy.golang.org
export GOPROXY

.PHONY: default all only_build only_test lint install test_all core build

# ensure that `default` is the target that is run when no arguments are passed to make
default::

# If there are sub projects, our default, all, and ensure targets will
# recurse into them.
ifneq ($(SUB_PROJECTS),)
only_build:: $(SUB_PROJECTS:%=%_only_build)
only_test:: $(SUB_PROJECTS:%=%_only_test)
only_test_fast:: $(SUB_PROJECTS:%=%_only_test_fast)
default:: $(SUB_PROJECTS:%=%_default)
all:: $(SUB_PROJECTS:%=%_all)
build:: $(SUB_PROJECTS:%=%_build)
install:: $(SUB_PROJECTS:%=%_install)
test_all:: $(SUB_PROJECTS:%=%_test_all)
dist:: $(SUB_PROJECTS:%=%_dist)
brew:: $(SUB_PROJECTS:%=%_brew)
endif

# `core` is like `default` except it does not build sub projects.
core:: build lint install test_fast

# If $(PROJECT_NAME) has been set, have our default and all targets
# print a nice banner.
ifneq ($(PROJECT_NAME),)
default::
	@printf "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m\n"
	@printf "\033[1;37m$(PROJECT_NAME)\033[1;37m\n"
	@printf "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m\n"
all::
	@printf "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m\n"
	@printf "\033[1;37m$(PROJECT_NAME)\033[1;37m\n"
	@printf "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m\n"
endif

default:: build install lint test_fast
all:: build install lint test_all


build::
	$(call STEP_MESSAGE)

lint::
	$(call STEP_MESSAGE)

test_fast::
	$(call STEP_MESSAGE)

install::
	$(call STEP_MESSAGE)
	@# Implicitly creates PULUMI_ROOT.
	@mkdir -p $(PULUMI_BIN)
	@mkdir -p $(PULUMI_NODE_MODULES)

dist::
	$(call STEP_MESSAGE)

brew::
	$(call STEP_MESSAGE)

test_all::
	$(call STEP_MESSAGE)

ifneq ($(NODE_MODULE_NAME),)
install::
	[ ! -e "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)" ] || rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	mkdir -p "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	cp -r bin/. "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	cp yarn.lock "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/node_modules"
	cd "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)" && \
	yarn install --prefer-offline --production && \
	(yarn unlink > /dev/null 2>&1 || true) && \
	yarn link
endif

only_build:: build install
only_test:: lint test_all
only_test_fast:: lint test_fast

# Generate targets for each sub project. This project's default and
# all targets will depend on the sub project's targets, and the
# individual targets for sub projects are added as a convenience when
# invoking make from the command line
ifneq ($(SUB_PROJECTS),)
$(SUB_PROJECTS:%=%_default):
	@$(MAKE) -C ./$(@:%_default=%) default
$(SUB_PROJECTS:%=%_all):
	@$(MAKE) -C ./$(@:%_all=%) all
$(SUB_PROJECTS:%=%_ensure):
	@$(MAKE) -C ./$(@:%_ensure=%) ensure
$(SUB_PROJECTS:%=%_build):
	@$(MAKE) -C ./$(@:%_build=%) build
$(SUB_PROJECTS:%=%_lint):
	@$(MAKE) -C ./$(@:%_lint=%) lint
$(SUB_PROJECTS:%=%_test_fast):
	@$(MAKE) -C ./$(@:%_test_fast=%) test_fast
$(SUB_PROJECTS:%=%_test_all):
	@$(MAKE) -C ./$(@:%_test_all=%) test_all
$(SUB_PROJECTS:%=%_install):
	@$(MAKE) -C ./$(@:%_install=%) install
$(SUB_PROJECTS:%=%_only_build):
	@$(MAKE) -C ./$(@:%_only_build=%) only_build
$(SUB_PROJECTS:%=%_only_test):
	@$(MAKE) -C ./$(@:%_only_test=%) only_test
$(SUB_PROJECTS:%=%_only_test_fast):
	@$(MAKE) -C ./$(@:%_only_test_fast=%) only_test_fast
$(SUB_PROJECTS:%=%_dist):
	@$(MAKE) -C ./$(@:%_dist=%) dist
$(SUB_PROJECTS:%=%_brew):
	@$(MAKE) -C ./$(@:%_brew=%) brew
endif

# As a convenience, we provide a format target that folks can build to run go fmt over all
# the go code in their tree.
.PHONY: format
format::
	$(call STEP_MESSAGE)
	find . -iname "*.go" -not \( \
		-path "./.git/*" -or \
		-path "./sdk/proto/go/*" -or \
		-path "./vendor/*" -or \
		-path "./*/compilation_error/*" -or \
		-path "./*/testdata/*" \
	\) | xargs gofumpt -w

.SECONDEXPANSION: # Needed by .make/ensure/% and .make/ensure/__%.

# Defines the target `.make/ensure/%` where `%` is an executable to check for. For
# example, the target `.make/ensure/foo` will check that `foo` is available on the user's
# path.
#
# .make/ensure/% does not imply that a target is phony. To guarantee that, you should use
# `.PHONY`.
.make/ensure/%: .make/ensure/__$$*
	@mkdir -p .make/ensure && touch $@

# How .make/ensure/% works:
#
# When a target depends on .make/ensure/node, it is caught by the .make/ensure/%
# target. With secondary-expansion set, this expands to:
#
#	.make/ensure/node: .make/ensure/__node
#		@mkdir -p .make/ensure && touch .make/ensure/node
#
# This is a simple expansion, but the indirection ensures that .make/ensure/__node is only
# depended on once. The real "work" is done during the *expansion* of .make/ensure/__%
# into .make/ensure/__node. .make/ensure/__% initially expands into:
#
#	.make/ensure/__node: $(if                                                                  \
#		$(shell if ! command -v "node" > /dev/null; then echo "missing"; fi),              \
#		$(error Missing binary dependency "node"),                                         \
#		$(info $(shell                                                                     \
#			pad="$$(printf '%0.1s' "."{1..20})";                                       \
#			exec="node";                                                               \
#			printf "Checking for %s %*.*s " "$${exec}" 0 $$((20 - $${#exec})) "$$pad"; \
#			echo "\033[0;32m✓\033[0m"                                                 \
#		))                                                                                 \
#	)
#		@mkdir -p .make/ensure && touch .make/ensure/__node
#
# Now the check makes more sense: If `command -v "node"` errors, the first `$(shell ...)`
# call returns a non-empty string, and the `$(if ...)` expands to `$(error Missing binary
# dependency "node")`. `$(error ...)` stops execution and displays to the user. If not,
# then `$(info ...)` is used to print a "found it" message to the user and the target
# dependency list expands successfully:
#
#	.make/ensure/__node:
#		@mkdir -p .make/ensure && touch $@
#
# This target is then executed as normal.
#
# ---
#
# OK, but why do it this way?
#
# By doing the check entirely within dependency expansion, we keep all targets as pure
# file dependencies. In effect, this means that make considers .make/ensure/% to be built
# on repeated runs, and listing an .make/ensure dependency does not require perpetual
# rebuilds.

# .make/ensure/__% is the inner target for .make/ensure/%.
.make/ensure/__%: $$(call __ensure_dependency,$$*)
	@mkdir -p .make/ensure && touch $@

# __ensure_dependency checks that it's first argument is on PATH. If not it errors
#
# __ensure_dependency is an implementation detail of `.make/ensure/%`, and need not be
# called directly.
define __ensure_dependency #$(1): the executable to check for: "go", "node", ...
	$(if                                                                                       \
		$(shell if ! command -v "$(1)" > /dev/null; then echo "missing"; fi),              \
		$(error Missing binary dependency "$(1)"),                                         \
		$(info $(shell                                                                     \
			pad="$$(printf '%0.1s' "."{1..20})";                                       \
			exec="$(1)";                                                               \
			printf "Checking for %s %*.*s " "$${exec}" 0 $$((20 - $${#exec})) "$$pad"; \
			echo "\033[0;32m✓\033[0m"                                                 \
		))                                                                                 \
	)
endef
