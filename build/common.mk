# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

# common.mk provides most of the scalfholding for our build system. It
# provides default targets for each project we want to build.
#
# The default targets we use are:
#
#  - ensure: restores and dependencies needed for the build from
#            remote sources (e.g dep ensure or yarn install)
#
#  - build: builds a project but does not install it. In the case of
#           go code, this usually means running go install (which
#           would place them in `GOBIN`, but not `PULUMI_ROOT`
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
# The ensure target also provides some default behavior, detecting if
# there is a Gopkg.toml or package.json file in the current folder and
# if so calling dep ensure -v or yarn install. This behavior means that
# projects will not often need to augment the ensure target.
#
# Unlike the other leaf targets, ensure will call the ensure target on
# any sub-projects.
#
# Importing common.mk should be the first thing your Makefile does, after
# optionally setting SUB_PROJECTS, PROJECT_NAME and NODE_MODULE_NAME.
SHELL       := /bin/bash
.SHELLFLAGS := -ec

STEP_MESSAGE = @echo -e "\033[0;32m$(shell echo '$@' | tr a-z A-Z | tr '_' ' '):\033[0m"

# Our install targets place items item into $PULUMI_ROOT, if it's
# unset, default to /opt/pulumi.
ifeq ($(PULUMI_ROOT),)
	PULUMI_ROOT:=/opt/pulumi
endif

PULUMI_BIN          := $(PULUMI_ROOT)/bin
PULUMI_NODE_MODULES := $(PULUMI_ROOT)/node_modules

.PHONY: default all ensure only_build only_test build lint install test_fast test_all core

# ensure that `default` is the target that is run when no arguments are passed to make
default::

# Ensure the requisite tools are on the PATH.
#     - Prefer Python2 over Python.
PYTHON := $(shell command -v python2 2>/dev/null)
ifeq ($(PYTHON),)
	PYTHON = $(shell command -v python 2>/dev/null)
endif
ifeq ($(PYTHON),)
ensure::
	$(error "missing python 2.7 (`python2` or `python`) from your $$PATH; \
		please see https://github.com/pulumi/home/wiki/Package-Management-Prerequisites")
else
PYTHON_VERSION := $(shell command $(PYTHON) --version 2>&1)
ifeq (,$(findstring 2.7,$(PYTHON_VERSION)))
ensure::
	$(error "$(PYTHON) did not report a 2.7 version number ($(PYTHON_VERSION)); \
		please see https://github.com/pulumi/home/wiki/Package-Management-Prerequisites")
endif
endif
#     - Prefer Pip2 over Pip.
PIP := $(shell command -v pip2 2>/dev/null)
ifeq ($(PIP),)
	PIP = $(shell command -v pip 2>/dev/null)
endif
ifeq ($(PIP),)
ensure::
	$(error "missing pip 2.7 (`pip2` or `pip`) from your $$PATH; \
		please see https://github.com/pulumi/home/wiki/Package-Management-Prerequisites")
else
PIP_VERSION := $(shell command $(PIP) --version 2>&1)
ifeq (,$(findstring python 2.7,$(PIP_VERSION)))
ensure::
	$(error "$(PIP) did not report a 2.7 version number ($(PIP_VERSION)); \
		please see https://github.com/pulumi/home/wiki/Package-Management-Prerequisites")
endif
endif

# If there are sub projects, our default, all, and ensure targets will
# recurse into them.
ifneq ($(SUB_PROJECTS),)
only_build:: $(SUB_PROJECTS:%=%_only_build)
only_test:: $(SUB_PROJECTS:%=%_only_test)
default:: $(SUB_PROJECTS:%=%_default)
all:: $(SUB_PROJECTS:%=%_all)
ensure:: $(SUB_PROJECTS:%=%_ensure)
endif

# `core` is like `default` except it does not build sub projects.
core:: build lint install test_fast

# If $(PROJECT_NAME) has been set, have our default and all targets
# print a nice banner.
ifneq ($(PROJECT_NAME),)
default::
	@echo -e "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m"
	@echo -e "\033[1;37m$(PROJECT_NAME)\033[1;37m"
	@echo -e "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m"
all::
	@echo -e "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m"
	@echo -e "\033[1;37m$(PROJECT_NAME)\033[1;37m"
	@echo -e "\033[1;37m$(shell echo '$(PROJECT_NAME)' | sed -e 's/./=/g')\033[1;37m"
endif

default:: build install lint test_fast
all:: build install lint test_all

ensure::
	$(call STEP_MESSAGE)
	@if [ -e 'Gopkg.toml' ]; then echo "dep ensure -v"; dep ensure -v; fi
	@if [ -e 'package.json' ]; then echo "yarn install"; yarn install; fi

build::
	$(call STEP_MESSAGE)

lint::
	$(call STEP_MESSAGE)

test_fast::
	$(call STEP_MESSAGE)

install::
	$(call STEP_MESSAGE)
	@mkdir -p $(PULUMI_BIN)
	@mkdir -p $(PULUMI_NODE_MODULES)

test_all:: test_fast
	$(call STEP_MESSAGE)

ifneq ($(NODE_MODULE_NAME),)
install::
	[ ! -e "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)" ] || rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	mkdir -p "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	cp -r bin/. "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	cp yarn.lock "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)"
	rm -rf "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)/node_modules"
	cd "$(PULUMI_NODE_MODULES)/$(NODE_MODULE_NAME)" && \
	yarn install --offline --production && \
	(yarn unlink > /dev/null 2>&1 || true) && \
	yarn link
endif

only_build:: build install
only_test:: lint test_all

# Generate targets for each sub project. This project's default and
# all targets will depend on the sub project's targets, and the
# individual targets for sub projects are added as a convience when
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
$(SUB_PROJECTS:%=%_install):
	@$(MAKE) -C ./$(@:%_install=%) install
$(SUB_PROJECTS:%=%_only_build):
	@$(MAKE) -C ./$(@:%_only_build=%) only_build
$(SUB_PROJECTS:%=%_only_test):
	@$(MAKE) -C ./$(@:%_only_test=%) only_test
endif

# As a convinece, we provide a format target that folks can build to
# run go fmt over all the go code in their tree.
.PHONY: format
format:
	find . -iname "*.go" -not -path "./vendor/*" | xargs gofmt -s -w
