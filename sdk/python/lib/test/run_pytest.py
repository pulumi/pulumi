# Pytest runner for Bazel py_test targets.
# This wrapper allows tests using relative imports to work correctly
# by running them through pytest instead of directly as __main__.
import sys
import pytest

sys.exit(pytest.main(sys.argv[1:]))
