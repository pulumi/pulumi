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

import importlib
import os
import subprocess
import sys
import unittest
from pathlib import Path

TOOLS_DIR = Path(__file__).resolve().parent.parent
FIXTURE = Path(__file__).resolve().parent / "fixture.json"
BOILERPLATE = TOOLS_DIR / "boilerplate" / "testing.py"
OUTPUT_DIR = TOOLS_DIR / "output"


def setUpModule() -> None:
    """Run the generator against the test fixture before running tests."""
    subprocess.check_call(
        [sys.executable, str(TOOLS_DIR / "main.py"), str(FIXTURE), str(BOILERPLATE), str(OUTPUT_DIR)],
        cwd=str(TOOLS_DIR),
    )


# Add the output directory to the path so we can import the generated module.
sys.path.insert(0, str(OUTPUT_DIR))


class TestCommands(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        # Import the generated module (force reimport if cached).
        if "main" in sys.modules:
            importlib.reload(sys.modules["main"])
        mod = importlib.import_module("main")
        cls.mod = mod
        cls.api = mod.API()

    def test_cancel(self) -> None:
        options = self.mod.PulumiCancelOptions()
        command = self.api.cancel(options, "my-stack")
        self.assertEqual(command, "pulumi cancel --yes -- my-stack")

    def test_cancel_no_stack(self) -> None:
        options = self.mod.PulumiCancelOptions()
        command = self.api.cancel(options)
        self.assertEqual(command, "pulumi cancel --yes")

    def test_cancel_with_option(self) -> None:
        options = self.mod.PulumiCancelOptions(stack="dev")
        command = self.api.cancel(options)
        self.assertEqual(command, "pulumi cancel --yes --stack dev")

    def test_org_get_default(self) -> None:
        options = self.mod.PulumiOrgGetDefaultOptions()
        command = self.api.org_get_default(options)
        self.assertEqual(command, "pulumi org get-default")

    def test_org_set_default(self) -> None:
        options = self.mod.PulumiOrgSetDefaultOptions()
        command = self.api.org_set_default(options, "my-org")
        self.assertEqual(command, "pulumi org set-default -- my-org")

    def test_org_search_with_query_flags(self) -> None:
        options = self.mod.PulumiOrgSearchOptions(
            org="my-org",
            query=["type:aws:s3/bucketv2:BucketV2", "modified:>=2023-09-01"],
            output="json",
        )
        command = self.api.org_search(options)
        self.assertEqual(
            command,
            "pulumi org search --org my-org --output json "
            "--query type:aws:s3/bucketv2:BucketV2 --query modified:>=2023-09-01",
        )

    def test_org_search_ai(self) -> None:
        options = self.mod.PulumiOrgSearchAiOptions(
            org="my-org",
            query="find all S3 buckets",
        )
        command = self.api.org_search_ai(options)
        self.assertEqual(
            command,
            "pulumi org search ai --org my-org --query find all S3 buckets",
        )

    def test_org_executable_menu(self) -> None:
        options = self.mod.PulumiOrgOptions()
        command = self.api.org(options)
        self.assertEqual(command, "pulumi org")

    def test_state_move_variadic(self) -> None:
        options = self.mod.PulumiStateMoveOptions(dest="prod", source="dev")
        command = self.api.state_move(options, "urn:1", "urn:2")
        self.assertEqual(
            command,
            "pulumi state move --yes --dest prod --source dev -- urn:1 urn:2",
        )

    def test_state_move_no_args(self) -> None:
        options = self.mod.PulumiStateMoveOptions()
        command = self.api.state_move(options)
        self.assertEqual(command, "pulumi state move --yes")

    def test_state_move_boolean_flag(self) -> None:
        options = self.mod.PulumiStateMoveOptions(include_parents=True)
        command = self.api.state_move(options, "urn:1")
        self.assertEqual(
            command,
            "pulumi state move --yes --include-parents -- urn:1",
        )


if __name__ == "__main__":
    unittest.main()
