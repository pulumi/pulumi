#!/bin/sh
# Print Bazel build system help. Run with: bazel run //:help

cat <<'EOF'
===============================================================================
 Pulumi Bazel Build System
===============================================================================

COMMON WORKFLOWS
----------------

  Build & install for local dev (host platform only):
    bazel build //build/release:install
    Output: bazel-bin/build/release/install/   (flat bin directory)

  Build all release archives (all 6 platforms, cross-compiled):
    bazel build //build/release:all_archives --define PULUMI_VERSION=3.x.y

  Build one platform archive:
    bazel build //build/release:archive_linux_amd64 --define PULUMI_VERSION=3.x.y

  Build a single Go binary (host platform):
    bazel build //:pulumi

  Build a single Go binary (cross-compiled):
    bazel build //build/release:pulumi_linux_amd64

  Build Node.js SDK tarball:
    bazel build //sdk/nodejs:npm_tarball --define PULUMI_VERSION=3.x.y

  Build Python SDK wheel:
    bazel build //sdk/python:wheel --define PYPI_VERSION=3.x.y

  Run Node.js SDK tests:
    bazel test //sdk/nodejs:unit_test

  Run Python SDK tests:
    bazel test //sdk/python/...

  Run a single Python test:
    bazel test //sdk/python:test_output

  Build everything (archives + SDKs):
    bazel build //build/release:all_archives //sdk/nodejs:npm_tarball \
      //sdk/python:wheel --define PULUMI_VERSION=3.x.y --define PYPI_VERSION=3.x.y

MAKE → BAZEL CHEAT SHEET
-------------------------

  make build                     bazel build //:pulumi
  make install                   bazel build //build/release:install
  make test_fast                 bazel test //sdk/nodejs:unit_test //sdk/python/...
  make sdk_nodejs_build          bazel build //sdk/nodejs:sdk_ts
  make sdk_python_build          bazel build //sdk/python:pulumi_sdk
  make dist                      bazel build //build/release:all_archives \
                                   --define PULUMI_VERSION=X.Y.Z
  (npm pack)                     bazel build //sdk/nodejs:npm_tarball
  (uv build)                     bazel build //sdk/python:wheel
  make clean                     bazel clean

KEY TARGETS
-----------

  //:pulumi                                     CLI binary (host platform)
  //:pulumi-language-go                         Go language host (host platform)
  //:pulumi-language-nodejs                     Node.js language host (host platform)
  //:pulumi-language-python                     Python language host (host platform)

  //build/release:install                       All binaries for host platform
  //build/release:all_archives                  Release archives (all platforms)
  //build/release:archive_{os}_{arch}           Single platform archive

  //sdk/nodejs:sdk_ts                           Compile TypeScript SDK
  //sdk/nodejs:npm_tarball                      Publishable npm tarball
  //sdk/nodejs:unit_test                        Node.js unit tests

  //sdk/python:pulumi_sdk                       Python SDK library
  //sdk/python:wheel                            Publishable Python wheel
  //sdk/python:test_*                           Individual Python tests

VERSION STAMPING
----------------

  Go binaries are automatically stamped via .bazelrc (--stamp).
  For release artifacts, pass the version explicitly:

    --define PULUMI_VERSION=3.225.0              Archives + npm tarball
    --define PYPI_VERSION=3.225.0                Python wheel

  Without --define, a dev version is computed from sdk/.version.

TIPS
----

  List all targets in a package:   bazel query '//build/release:*'
  Find all test targets:           bazel query 'kind(".*_test", //...)'
  Explain why something rebuilt:   bazel aquery //build/release:install
  View the dependency graph:       bazel query 'deps(//build/release:install)' --output graph

===============================================================================
EOF
