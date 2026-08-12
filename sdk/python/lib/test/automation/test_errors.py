# Copyright 2016, Pulumi Corporation.
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

import subprocess
import sys

import pytest

from pulumi.automation import (
    CompilationError,
    InlineSourceRuntimeError,
    RuntimeError,
    create_stack,
)

from .test_local_workspace import stack_namer, get_test_path

compilation_error_project = "compilation_error"
runtime_error_project = "runtime_error"


def install_dependencies(project_dir: str) -> None:
    subprocess.run(
        ["pulumi", "install"], check=True, cwd=project_dir, capture_output=True
    )


def test_inline_runtime_error_python():
    project_name = "inline_runtime_error_python"
    stack_name = stack_namer(project_name)
    stack = create_stack(stack_name, program=failing_program, project_name=project_name)
    inline_error_text = "python inline source runtime error"

    try:
        with pytest.raises(InlineSourceRuntimeError, match=inline_error_text):
            stack.up()
        with pytest.raises(InlineSourceRuntimeError, match=inline_error_text):
            stack.preview()
    finally:
        stack.workspace.remove_stack(stack_name, force=True)


@pytest.mark.parametrize(
    "lang,error",
    [
        ("python", "failed with an unhandled exception"),
        ("go", "panic: runtime error"),
        ("dotnet", "failed with an unhandled exception"),
        ("javascript", "failed with an unhandled exception"),
        ("typescript", "failed with an unhandled exception"),
    ],
)
def test_runtime_errors(lang: str, error: str):
    stack_name = stack_namer(runtime_error_project)
    project_dir = get_test_path("errors", runtime_error_project, lang)
    install_dependencies(project_dir)

    stack = create_stack(stack_name, work_dir=project_dir)

    try:
        with pytest.raises(RuntimeError, match=error):
            stack.up()
    finally:
        stack.workspace.remove_stack(stack_name, force=True)


@pytest.mark.parametrize(
    "lang,error",
    [
        ("go", ": syntax error:|: undefined:"),
        ("dotnet", "Build FAILED."),
        pytest.param(
            "typescript",
            "Unable to compile TypeScript",
            marks=pytest.mark.skipif(
                sys.platform == "win32",
                reason="environment setup fails on Windows",
            ),
        ),
    ],
)
def test_compilation_errors(lang: str, error: str):
    stack_name = stack_namer(compilation_error_project)
    project_dir = get_test_path("errors", compilation_error_project, lang)
    if lang == "typescript":
        install_dependencies(project_dir)
    stack = create_stack(stack_name, work_dir=project_dir)

    try:
        with pytest.raises(CompilationError, match=error):
            stack.up()
    finally:
        stack.workspace.remove_stack(stack_name, force=True)


def failing_program():
    my_list = []
    oh_no = my_list[0]
