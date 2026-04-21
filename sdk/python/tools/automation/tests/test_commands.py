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
import importlib.util
import os
import subprocess
import sys
import unittest
from collections.abc import Callable
from pathlib import Path
from typing import Any, Dict, List

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
        command = self.api.cancel("my-stack")
        self.assertEqual(command, "pulumi cancel --yes -- my-stack")

    def test_cancel_no_stack(self) -> None:
        command = self.api.cancel()
        self.assertEqual(command, "pulumi cancel --yes")

    def test_cancel_with_option(self) -> None:
        command = self.api.cancel(stack="dev")
        self.assertEqual(command, "pulumi cancel --yes --stack dev")

    def test_org_get_default(self) -> None:
        command = self.api.org_get_default()
        self.assertEqual(command, "pulumi org get-default")

    def test_org_set_default(self) -> None:
        command = self.api.org_set_default("my-org")
        self.assertEqual(command, "pulumi org set-default -- my-org")

    def test_org_search_with_query_flags(self) -> None:
        command = self.api.org_search(
            org="my-org",
            query=["type:aws:s3/bucketv2:BucketV2", "modified:>=2023-09-01"],
            output="json",
        )
        self.assertEqual(
            command,
            "pulumi org search --org my-org --output json "
            "--query type:aws:s3/bucketv2:BucketV2 --query modified:>=2023-09-01",
        )

    def test_org_search_ai(self) -> None:
        command = self.api.org_search_ai(
            org="my-org",
            query="find all S3 buckets",
        )
        self.assertEqual(
            command,
            "pulumi org search ai --org my-org --query find all S3 buckets",
        )

    def test_org_executable_menu(self) -> None:
        command = self.api.org()
        self.assertEqual(command, "pulumi org")

    def test_state_move_variadic(self) -> None:
        command = self.api.state_move("urn:1", "urn:2", dest="prod", source="dev")
        self.assertEqual(
            command,
            "pulumi state move --yes --dest prod --source dev -- urn:1 urn:2",
        )

    def test_state_move_no_args(self) -> None:
        command = self.api.state_move()
        self.assertEqual(command, "pulumi state move --yes")

    def test_state_move_boolean_flag(self) -> None:
        command = self.api.state_move("urn:1", include_parents=True)
        self.assertEqual(
            command,
            "pulumi state move --yes --include-parents -- urn:1",
        )

    def test_base_options_kwargs_propagate(self) -> None:
        # The four BaseOptions kwargs are lifted into every generated method
        # and must be forwarded verbatim to ``self._run``. Spy on ``_run`` and
        # assert it sees exactly those four kwargs with the expected values.
        #
        # The assertion is on the full captured dict (not ``assertIn`` per
        # key), so a future fifth BaseOptions kwarg added to the generator
        # forces this test to be updated — which, in turn, catches any
        # boilerplate that forgot to accept it.
        on_out: Callable[[str], Any] = lambda _: None
        on_err: Callable[[str], Any] = lambda _: None

        api = self.mod.API()
        captured: Dict[str, Any] = {}
        original_run = api._run

        def spy(args: List[str], **kwargs: Any) -> str:
            captured.update(kwargs)
            return original_run(args, **kwargs)

        api._run = spy  # type: ignore[method-assign]

        api.cancel(
            stack="dev",
            cwd="/tmp",
            additional_env={"FOO": "bar"},
            on_output=on_out,
            on_error=on_err,
        )

        self.assertEqual(
            captured,
            {
                "cwd": "/tmp",
                "additional_env": {"FOO": "bar"},
                "on_output": on_out,
                "on_error": on_err,
            },
        )


class TestCollisionGuards(unittest.TestCase):
    """Unit tests for the generator's collision checks against reserved names."""

    @classmethod
    def setUpClass(cls) -> None:
        # Import main.py directly under a non-conflicting name (the generated
        # output also uses ``main`` as its module name).
        spec = importlib.util.spec_from_file_location(
            "automation_generator", str(TOOLS_DIR / "main.py")
        )
        assert spec is not None and spec.loader is not None
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        cls.gen = module

    def test_flag_collides_with_reserved_kwarg(self) -> None:
        structure = {
            "type": "command",
            "flags": {"cwd": {"name": "cwd", "type": "string"}},
            "arguments": {"arguments": []},
        }
        with self.assertRaises(ValueError) as ctx:
            self.gen._generate_commands(structure, [], breadcrumbs=["test"])
        self.assertIn("reserved keyword argument", str(ctx.exception))

    def test_positional_collides_with_reserved_kwarg(self) -> None:
        structure = {
            "type": "command",
            "arguments": {"arguments": [{"name": "cwd"}]},
        }
        with self.assertRaises(ValueError) as ctx:
            self.gen._generate_commands(structure, [], breadcrumbs=["test"])
        self.assertIn("reserved keyword argument", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
