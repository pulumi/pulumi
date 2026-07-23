# Copyright 2026, Pulumi Corporation.
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
import tempfile
from collections.abc import Iterator

import pytest


@pytest.fixture(scope="session", autouse=True)
def test_backend() -> Iterator[None]:
    """Configure an isolated backend for every Automation API test.

    Use Pulumi Cloud when an access token is available and a temporary file backend otherwise. Because pytest-xdist runs
    a separate session per worker, local tests also get separate backends. The fixture is automatic so every Pulumi
    subprocess inherits the backend and Go workspace isolation consistently.
    """
    # The standalone Go test programs must use their own modules, not the
    # repository's optional development go.work file.
    old_go_work = os.environ.get("GOWORK")
    os.environ["GOWORK"] = "off"

    try:
        if os.getenv("PULUMI_ACCESS_TOKEN"):
            old_backend_url = os.environ.get("PULUMI_BACKEND_URL")
            os.environ["PULUMI_BACKEND_URL"] = (
                old_backend_url or "https://api.pulumi.com"
            )
            try:
                yield
            finally:
                if old_backend_url is None:
                    os.environ.pop("PULUMI_BACKEND_URL", None)
                else:
                    os.environ["PULUMI_BACKEND_URL"] = old_backend_url
            return

        with tempfile.TemporaryDirectory(prefix="python-tests-automation-") as backend:
            old_backend_url = os.environ.get("PULUMI_BACKEND_URL")
            old_passphrase = os.environ.get("PULUMI_CONFIG_PASSPHRASE")
            os.environ["PULUMI_BACKEND_URL"] = f"file://{backend}"
            os.environ["PULUMI_CONFIG_PASSPHRASE"] = "test"
            try:
                yield
            finally:
                if old_backend_url is None:
                    os.environ.pop("PULUMI_BACKEND_URL", None)
                else:
                    os.environ["PULUMI_BACKEND_URL"] = old_backend_url
                if old_passphrase is None:
                    os.environ.pop("PULUMI_CONFIG_PASSPHRASE", None)
                else:
                    os.environ["PULUMI_CONFIG_PASSPHRASE"] = old_passphrase
    finally:
        if old_go_work is None:
            os.environ.pop("GOWORK", None)
        else:
            os.environ["GOWORK"] = old_go_work
