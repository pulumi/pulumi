# Pytest runner for Bazel py_test targets.
# This wrapper allows tests using relative imports to work correctly
# by running them through pytest instead of directly as __main__.
import os
import sys

# If running under Bazel, find the pulumi CLI binary in runfiles and add to PATH.
# This is needed for automation tests that invoke pulumi as a subprocess.
_test_srcdir = os.environ.get("TEST_SRCDIR")
if _test_srcdir:
    _pulumi_bin_dir = os.path.join(
        _test_srcdir, "_main", "pkg", "cmd", "pulumi", "pulumi_"
    )
    if os.path.isdir(_pulumi_bin_dir):
        os.environ["PATH"] = _pulumi_bin_dir + os.pathsep + os.environ.get("PATH", "")

import pytest

sys.exit(pytest.main(sys.argv[1:]))
