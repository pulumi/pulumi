from __future__ import annotations

import ast
import datetime
import json
import keyword
import os
import sys
from pathlib import Path
from typing import Any, Dict, Mapping

import stringcase


def _prepare_string(value: str) -> str:
    """
    Prepare a string for case conversion.
    """
    return value.strip().replace(" ", "_").replace("-", "_")


def _sanitise_keyword(value: str) -> str:
    """
    Sanitise a string to be a valid Python keyword.
    """
    if keyword.iskeyword(value):
        return "_" + value
    return value


def _pascal_case(value: str) -> str:
    """
    Convert a string into PascalCase.
    """
    return _sanitise_keyword(stringcase.pascalcase(_prepare_string(value)))


def _snake_case(value: str) -> str:
    """
    Convert a string into snake_case.
    """
    return _sanitise_keyword(stringcase.snakecase(_prepare_string(value)))


def _generate_options_types(
    structure: Mapping[str, Any],
    source: list[ast.stmt],
    breadcrumbs: list[str] = [],
    inherited: Dict[str, Mapping[str, Any]] = {},
) -> None:
    """
    Collect all the flags for the current subcommand, including all the parent flags.
    """
    command = "pulumi " + " ".join(breadcrumbs)
    class_name = _pascal_case(command) + "Options"

    flags: Dict[str, Mapping[str, Any]] = {
        **inherited,
        **(structure.get("flags") or {}),
    }

    # Flag identifier, type, and description.
    flag_items: list[tuple[str, str, str]] = []
    for flag in flags.values():
        name = str(flag.get("name", ""))
        if not name:
            continue

        identifier = _snake_case(name)
        if not identifier:
            continue

        flag_type = str(flag.get("type", "string"))
        repeatable = bool(flag.get("repeatable", False))
        annotation = _convert_type(flag_type, repeatable)
        description = flag.get("description")

        flag_items.append((identifier, annotation, description))

    class_body: list[ast.stmt] = []

    doc_lines = [f"Options for the `{command}` command."]
    description_lines = [
        f"{identifier}: {description}"
        for identifier, _, description in flag_items
        if description
    ]
    if description_lines:
        doc_lines.append("")
        doc_lines.extend(description_lines)

    class_body.append(
        ast.Expr(value=ast.Constant(value="\n".join(doc_lines))),
    )

    if not flag_items:
        class_body.append(ast.Pass())
    else:
        for identifier, annotation, _ in flag_items:
            annotation_expr = ast.parse(annotation, mode="eval").body
            class_body.append(
                ast.AnnAssign(
                    target=ast.Name(id=identifier, ctx=ast.Store()),
                    annotation=annotation_expr,
                    value=None,
                    simple=1,
                )
            )

    source.append(
        ast.ClassDef(
            name=class_name,
            bases=[],
            keywords=[],
            body=class_body,
            decorator_list=[],
        )
    )

    # Recurse into child commands if this node is a menu.
    if structure.get("type") == "menu":
        commands = structure.get("commands") or {}
        for name, child in commands.items():
            _generate_options_types(
                child,
                source,
                breadcrumbs=[*breadcrumbs, str(name)],
                inherited=flags,
            )


def _convert_type(flag_type: str, repeatable: bool) -> str:
    """
    Convert the specification type system to the Python type system.
    """
    base: str

    if flag_type == "string":
        base = "str"
    elif flag_type == "boolean":
        base = "bool"
    elif flag_type == "int":
        base = "int"
    else:
        raise ValueError(f"Unknown flag type: {flag_type!r}")

    return f"list[{base}]" if repeatable else base


def main(argv: list[str]) -> int:
    usage = "Usage: python main.py <path-to-specification.json>"

    if len(argv) != 2:
        print(usage, file=sys.stderr)
        return 1

    specification = Path(os.path.abspath(argv[1]))
    if not specification.is_file():
        print(f"Specification file not found: {specification}", file=sys.stderr)
        return 1

    with specification.open(encoding="utf-8") as f:
        structure = json.load(f)

    output_dir = Path(os.getcwd()) / "output"
    output_dir.mkdir(parents=True, exist_ok=True)

    target = output_dir / "main.py"

    # Build the module AST for the generated types.
    module_body: list[ast.stmt] = []

    _generate_options_types(structure, module_body)

    module = ast.Module(body=module_body, type_ignores=[])
    ast.fix_missing_locations(module)

    try:
        code = ast.unparse(module)
    except AttributeError:
        print(
            "This generator requires Python 3.9 or newer with ast.unparse() available.",
            file=sys.stderr,
        )
        return 1

    current_year = datetime.datetime.now().year

    header_lines = [
        f"# Copyright {current_year}, Pulumi Corporation.",
        "#",
        '# Licensed under the Apache License, Version 2.0 (the "License");',
        "# you may not use this file except in compliance with the License.",
        "# You may obtain a copy of the License at",
        "#",
        "#     http://www.apache.org/licenses/LICENSE-2.0",
        "#",
        "# Unless required by applicable law or agreed to in writing, software",
        '# distributed under the License is distributed on an "AS IS" BASIS,',
        "# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.",
        "# See the License for the specific language governing permissions and",
        "# limitations under the License.",
        "",
    ]

    target.write_text("\n".join(header_lines) + code + "\n", encoding="utf-8")

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))

