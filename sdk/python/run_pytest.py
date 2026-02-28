"""Bazel test entry point that runs pytest on specified test files."""
import sys
import pytest

if __name__ == "__main__":
    sys.exit(pytest.main(sys.argv[1:]))
