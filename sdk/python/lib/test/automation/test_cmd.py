# Copyright 2016-2024, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
import subprocess
import tempfile
import unittest
from pathlib import Path

import pytest
from semver import VersionInfo

from pulumi.automation import InvalidVersionError, PulumiCommand
from pulumi.automation._cmd import _fixup_path, _parse_and_validate_pulumi_version


def test_install_default_root():
    requested_version = VersionInfo.parse("3.101.0")

    p = PulumiCommand.install(version=requested_version)

    pulumiBin = (
        Path.home() / ".pulumi" / "versions" / str(requested_version) / "bin" / "pulumi"
    )
    try:
        pulumiBin.stat()
    except Exception as exception:
        pytest.fail(f"did not find pulumi binary: {exception}")
    assert p.version == "3.101.0"
    out = subprocess.check_output([pulumiBin, "version"])
    assert out.decode("utf-8").strip() == "v" + str(requested_version)


def test_install_twice():
    with tempfile.TemporaryDirectory(prefix="automation-test-") as root:
        requested_version = VersionInfo.parse("3.101.0")
        pulumiBin = Path(root) / "bin" / "pulumi"

        PulumiCommand.install(version=requested_version, root=root)
        stat1 = pulumiBin.stat()

        PulumiCommand.install(version=requested_version, root=root)
        stat2 = pulumiBin.stat()

        assert stat1.st_ino == stat2.st_ino


def test_incompatible_version():
    with tempfile.TemporaryDirectory(prefix="automation-test-") as root:
        installed_version = VersionInfo.parse("3.99.0")
        PulumiCommand.install(version=installed_version, root=root)
        requested_version = VersionInfo.parse("3.101.0")
        # Try getting an incompatible version
        try:
            PulumiCommand(root=root, version=requested_version)
            pytest.fail("expected an exception")
        except InvalidVersionError as exception:
            assert "Minimum version requirement failed" in str(exception)
        # Succeeds when disabling version check
        PulumiCommand(root=root, version=requested_version, skip_version_check=True)


def test_fixup_env():
    env = {"PATH": "/usr/bin", "SOME_VAR": "some value"}
    new_env = _fixup_path(env, "/tmp/pulumi-install/bin")
    if os.name == "nt":
        assert new_env["PATH"] == "/tmp/pulumi-install/bin;/usr/bin"
    else:
        assert new_env["PATH"] == "/tmp/pulumi-install/bin:/usr/bin"


MAJOR = "Major version mismatch."
MINIMAL = "Minimum version requirement failed."
PARSE = "Could not parse the Pulumi CLI"
version_tests = [
    # current_version, expected_error regex, opt_out
    ("100.0.0", MAJOR, False),
    ("1.0.0", MINIMAL, False),
    ("2.22.0", None, False),
    ("2.1.0", MINIMAL, False),
    ("2.21.2", None, False),
    ("2.21.1", None, False),
    ("2.21.0", MINIMAL, False),
    # Note that prerelease < release so this case will error
    ("2.21.1-alpha.1234", MINIMAL, False),
    # Test opting out of version check
    ("2.20.0", None, True),
    ("2.22.0", None, True),
    # Test invalid version
    ("invalid", PARSE, False),
    ("invalid", None, True),
]
test_min_version = VersionInfo.parse("2.21.1")


class TestParseAndValidatePulumiVersion(unittest.TestCase):
    def test_validate_pulumi_version(self):
        for current_version, expected_error, opt_out in version_tests:
            with self.subTest():
                if expected_error:
                    with self.assertRaisesRegex(
                        InvalidVersionError,
                        expected_error,
                        msg=f"min_version:{test_min_version}, current_version:{current_version}",
                    ):
                        _parse_and_validate_pulumi_version(
                            test_min_version, current_version, opt_out
                        )
                else:
                    _parse_and_validate_pulumi_version(
                        test_min_version, current_version, opt_out
                    )
