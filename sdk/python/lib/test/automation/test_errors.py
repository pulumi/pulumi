# Copyright 2016-2021, Pulumi Corporation.
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
import sys
import subprocess
import unittest
import pytest
from pulumi.automation import (
    create_stack,
    InlineSourceRuntimeError,
    RuntimeError,
    CompilationError,
)
from .test_local_workspace import stack_namer, test_path

compilation_error_project = "compilation_error"
runtime_error_project = "runtime_error"


class TestErrors(unittest.TestCase):
    def test_inline_runtime_error_python(self):
        project_name = "inline_runtime_error_python"
        stack_name = stack_namer(project_name)
        stack = create_stack(stack_name, program=failing_program, project_name=project_name)
        inline_error_text = "python inline source runtime error"

        try:
            self.assertRaises(InlineSourceRuntimeError, stack.up)
            self.assertRaisesRegex(InlineSourceRuntimeError, inline_error_text, stack.up)
            self.assertRaises(InlineSourceRuntimeError, stack.preview)
            self.assertRaisesRegex(InlineSourceRuntimeError, inline_error_text, stack.preview)
        finally:
            stack.workspace.remove_stack(stack_name)

    # This test fails on Windows related to the `subprocess.run` call associated with setting up the environment.
    # Skipping for now.
    @pytest.mark.skipif(sys.platform == "win32", reason="skipping on windows")
    def test_runtime_errors(self):
        for lang in ["python", "go", "dotnet", "javascript", "typescript"]:
            stack_name = stack_namer(runtime_error_project)
            project_dir = test_path("errors", runtime_error_project, lang)

            if lang in ["javascript", "typescript"]:
                subprocess.run(["npm", "install"], check=True, cwd=project_dir, capture_output=True)
            if lang == "python":
                subprocess.run(["python3", "-m", "venv", "venv"], check=True, cwd=project_dir, capture_output=True)
                subprocess.run([os.path.join("venv", "bin", "pip"), "install", "-r", "requirements.txt"],
                               check=True, cwd=project_dir, capture_output=True)

            stack = create_stack(stack_name, work_dir=project_dir)

            try:
                self.assertRaises(RuntimeError, stack.up)
                if lang == "go":
                    self.assertRaisesRegex(RuntimeError, "panic: runtime error", stack.up)
                else:
                    self.assertRaisesRegex(RuntimeError, "failed with an unhandled exception", stack.up)
            finally:
                stack.workspace.remove_stack(stack_name)

    def test_compilation_error_go(self):
        stack_name = stack_namer(compilation_error_project)
        project_dir = test_path("errors", compilation_error_project, "go")
        stack = create_stack(stack_name, work_dir=project_dir)

        try:
            self.assertRaises(CompilationError, stack.up)
            self.assertRaisesRegex(CompilationError, ": syntax error:|: undefined:", stack.up)
        finally:
            stack.workspace.remove_stack(stack_name)

    def test_compilation_error_dotnet(self):
        stack_name = stack_namer(compilation_error_project)
        project_dir = test_path("errors", compilation_error_project, "dotnet")
        stack = create_stack(stack_name, work_dir=project_dir)

        try:
            self.assertRaises(CompilationError, stack.up)
            self.assertRaisesRegex(CompilationError, "Build FAILED.", stack.up)
        finally:
            stack.workspace.remove_stack(stack_name)

    # This test fails on Windows related to the `subprocess.run` call associated with setting up the environment.
    # Skipping for now.
    @pytest.mark.skipif(sys.platform == "win32", reason="skipping on windows")
    def test_compilation_error_typescript(self):
        stack_name = stack_namer(compilation_error_project)
        project_dir = test_path("errors", compilation_error_project, "typescript")
        subprocess.run(["npm", "install"], check=True, cwd=project_dir, capture_output=True)
        stack = create_stack(stack_name, work_dir=project_dir)

        try:
            self.assertRaises(CompilationError, stack.up)
            self.assertRaisesRegex(CompilationError, "Unable to compile TypeScript", stack.up)
        finally:
            stack.workspace.remove_stack(stack_name)


def failing_program():
    my_list = []
    oh_no = my_list[0]
