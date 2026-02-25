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
    Avoid conflicts with Python keywords.
    """
    if keyword.iskeyword(value):
        return value + "_"
    return value


def _pascal_case(value: str) -> str:
    """
    Convert a string into PascalCase for class names.
    """
    return _sanitise_keyword(stringcase.pascalcase(_prepare_string(value)))


def _snake_case(value: str) -> str:
    """
    Convert a string into snake_case.
    """
    return _sanitise_keyword(stringcase.snakecase(_prepare_string(value)))


def _create_command_name(breadcrumbs: list[str]) -> str:
    """
    Convert a list of subcommand breadcrumbs into the unconfigured CLI command.
    """
    return "pulumi " + " ".join(breadcrumbs)


def _create_options_type_name(breadcrumbs: list[str]) -> str:
    """
    Convert a list of subcommand breadcrumbs into the options type name.
    """
    return _pascal_case(_create_command_name(breadcrumbs)) + "Options"


def _generate_options_types(
    structure: Mapping[str, Any],
    source: list[ast.stmt],
    breadcrumbs: list[str] = [],
    inherited: Dict[str, Mapping[str, Any]] = {},
) -> None:
    """
    Collect all the flags for the current subcommand, including all the parent flags.
    """
    command = _create_command_name(breadcrumbs)
    class_name = _create_options_type_name(breadcrumbs)

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


def _convert_argument_type(flag_type: str) -> str:
    """
    Convert an argument type from the specification into a Python type annotation.
    """
    if flag_type == "string":
        return "str"
    if flag_type == "boolean":
        return "bool"
    if flag_type == "int":
        return "int"
    raise ValueError(f"Unknown argument type: {flag_type!r}")


def _generate_commands(
    structure: Mapping[str, Any],
    methods: list[ast.stmt],
    breadcrumbs: list[str] | None = None,
) -> None:
    """
    Generate the low-level Automation API methods that build CLI commands.
    """
    if breadcrumbs is None:
        breadcrumbs = []

    node_type = structure.get("type")

    if node_type == "menu":
        commands = structure.get("commands") or {}
        for name, child in commands.items():
            _generate_commands(
                child,
                methods,
                breadcrumbs=[*breadcrumbs, str(name)],
            )

        # Non-executable menus are just containers for subcommands.
        if not structure.get("executable"):
            return

    if not breadcrumbs:
        return
    options_type = _create_options_type_name(breadcrumbs)
    method_name = _snake_case("_".join(breadcrumbs))

    # Argument specification for the CLI command.
    arguments: list[Dict[str, Any]] = []
    variadic_info: Dict[str, Any] | None = None

    if node_type == "command":
        argument_spec = structure.get("arguments") or {}
        argument_items = argument_spec.get("arguments") or []

        if argument_items:
            required = int(argument_spec.get("requiredArguments") or 0)
            variadic_flag = bool(argument_spec.get("variadic") or False)
            last_index = len(argument_items) - 1

            for index, argument in enumerate(argument_items):
                raw_name = str(argument.get("name") or f"arg_{index + 1}")
                identifier = _snake_case(raw_name) or f"arg_{index + 1}"

                is_variadic = variadic_flag and index == last_index
                is_optional = index >= required and not is_variadic

                arg_type = _convert_argument_type(str(argument.get("type") or "string"))
                annotation_expr = ast.parse(arg_type, mode="eval").body

                info: Dict[str, Any] = {
                    "name": identifier,
                    "optional": is_optional,
                    "variadic": is_variadic,
                    "annotation": annotation_expr,
                }

                arguments.append(info)

                if is_variadic:
                    variadic_info = info

    # Build the function signature: (self, __options: OptionsType, ...)
    pos_args: list[ast.arg] = [
        ast.arg(arg="self"),
        ast.arg(arg="__options", annotation=ast.Name(id=options_type, ctx=ast.Load())),
    ]
    has_default: list[bool] = [False, False]

    for info in arg_infos:
        if info.get("variadic"):
            continue

        name = str(info["name"])
        annotation_expr = info["annotation"]
        optional = bool(info["optional"])

        pos_args.append(ast.arg(arg=name, annotation=annotation_expr))
        has_default.append(optional)

    defaults: list[ast.expr] = []
    if any(has_default):
        first_default = next(i for i, flag in enumerate(has_default) if flag)
        count = len(has_default) - first_default
        defaults = [ast.Constant(value=None) for _ in range(count)]

    vararg: ast.arg | None = None
    if variadic_info is not None:
        vararg = ast.arg(
            arg=str(variadic_info["name"]),
            annotation=variadic_info["annotation"],
        )

    arguments = ast.arguments(
        posonlyargs=[],
        args=pos_args,
        vararg=vararg,
        kwonlyargs=[],
        kw_defaults=[],
        kwarg=None,
        defaults=defaults,
    )

    # Helper constructors for AST nodes.
    def _name(identifier: str, ctx: ast.expr_context = ast.Load()) -> ast.Name:
        return ast.Name(id=identifier, ctx=ctx)

    def _attr(value: ast.expr, attr: str) -> ast.Attribute:
        return ast.Attribute(value=value, attr=attr, ctx=ast.Load())

    def _call(func: ast.expr, *args: ast.expr) -> ast.Call:
        return ast.Call(func=func, args=list(args), keywords=[])

    body: list[ast.stmt] = []

    # __final = ['pulumi']
    body.append(
        ast.Assign(
            targets=[_name("__final", ctx=ast.Store())],
            value=ast.List(elts=[ast.Constant(value="pulumi")], ctx=ast.Load()),
        )
    )

    # __final.append('<breadcrumb>')
    for breadcrumb in breadcrumbs:
        body.append(
            ast.Expr(
                value=_call(
                    _attr(_name("__final"), "append"),
                    ast.Constant(value=str(breadcrumb)),
                )
            )
        )

    # __flags = []
    body.append(
        ast.Assign(
            targets=[_name("__flags", ctx=ast.Store())],
            value=ast.List(elts=[], ctx=ast.Load()),
        )
    )

    # Flags: build __flags from __options.
    flags: Dict[str, Mapping[str, Any]] = structure.get("flags") or {}
    for flag in flags.values():
        flag_name = str(flag.get("name", ""))
        if not flag_name:
            continue

        attribute = _snake_case(flag_name)
        if not attribute:
            continue

        repeatable = bool(flag.get("repeatable", False))
        flag_type = str(flag.get("type", "string"))

        if repeatable:
            # for __item in (getattr(__options, '<attr>', []) or []):
            source_call = _call(
                _name("getattr"),
                _name("__options"),
                ast.Constant(value=attribute),
                ast.List(elts=[], ctx=ast.Load()),
            )
            loop_iter = ast.BoolOp(
                op=ast.Or(),
                values=[
                    source_call,
                    ast.List(elts=[], ctx=ast.Load()),
                ],
            )

            loop_body: list[ast.stmt] = []

            if flag_type == "boolean":
                # if __item: __flags.append('--flag')
                loop_body.append(
                    ast.If(
                        test=_name("__item"),
                        body=[
                            ast.Expr(
                                value=_call(
                                    _attr(_name("__flags"), "append"),
                                    ast.Constant(value=f"--{flag_name}"),
                                )
                            )
                        ],
                        orelse=[],
                    )
                )
            else:
                # if __item is not None: __flags.extend(['--flag', str(__item)])
                loop_body.append(
                    ast.If(
                        test=ast.Compare(
                            left=_name("__item"),
                            ops=[ast.IsNot()],
                            comparators=[ast.Constant(value=None)],
                        ),
                        body=[
                            ast.Expr(
                                value=_call(
                                    _attr(_name("__flags"), "extend"),
                                    ast.List(
                                        elts=[
                                            ast.Constant(value=f"--{flag_name}"),
                                            _call(_name("str"), _name("__item")),
                                        ],
                                        ctx=ast.Load(),
                                    ),
                                )
                            )
                        ],
                        orelse=[],
                    )
                )

            body.append(
                ast.For(
                    target=_name("__item", ctx=ast.Store()),
                    iter=loop_iter,
                    body=loop_body,
                    orelse=[],
                )
            )
        else:
            # value_expr = getattr(__options, '<attr>', None)
            value_call = _call(
                _name("getattr"),
                _name("__options"),
                ast.Constant(value=attribute),
                ast.Constant(value=None),
            )

            if flag_type == "boolean":
                # if value_expr: __flags.append('--flag')
                body.append(
                    ast.If(
                        test=value_call,
                        body=[
                            ast.Expr(
                                value=_call(
                                    _attr(_name("__flags"), "append"),
                                    ast.Constant(value=f"--{flag_name}"),
                                )
                            )
                        ],
                        orelse=[],
                    )
                )
            else:
                # if value_expr is not None:
                #     __flags.extend(['--flag', str(value_expr)])
                body.append(
                    ast.If(
                        test=ast.Compare(
                            left=value_call,
                            ops=[ast.IsNot()],
                            comparators=[ast.Constant(value=None)],
                        ),
                        body=[
                            ast.Expr(
                                value=_call(
                                    _attr(_name("__flags"), "extend"),
                                    ast.List(
                                        elts=[
                                            ast.Constant(value=f"--{flag_name}"),
                                            _call(
                                                _name("str"),
                                                _call(
                                                    _name("getattr"),
                                                    _name("__options"),
                                                    ast.Constant(value=attribute),
                                                    ast.Constant(value=None),
                                                ),
                                            ),
                                        ],
                                        ctx=ast.Load(),
                                    ),
                                )
                            )
                        ],
                        orelse=[],
                    )
                )

    # __final.extend(__flags)
    body.append(
        ast.Expr(
            value=_call(
                _attr(_name("__final"), "extend"),
                _name("__flags"),
            )
        )
    )

    if arguments:
        # __arguments = []
        body.append(
            ast.Assign(
                targets=[_name("__arguments", ctx=ast.Store())],
                value=ast.List(elts=[], ctx=ast.Load()),
            )
        )

        for info in arguments:
            name = str(info.get("name", ""))
            optional = bool(info.get("optional", False))
            variadic = bool(info.get("variadic", False))

            if not name:
                continue

            if variadic:
                # for __item in name: __arguments.append(str(__item))
                body.append(
                    ast.For(
                        target=_name("__item", ctx=ast.Store()),
                        iter=_name(name),
                        body=[
                            ast.Expr(
                                value=_call(
                                    _attr(_name("__arguments"), "append"),
                                    _call(_name("str"), _name("__item")),
                                )
                            )
                        ],
                        orelse=[],
                    )
                )
            elif optional:
                # if name is not None: __arguments.append(str(name))
                body.append(
                    ast.If(
                        test=ast.Compare(
                            left=_name(name),
                            ops=[ast.IsNot()],
                            comparators=[ast.Constant(value=None)],
                        ),
                        body=[
                            ast.Expr(
                                value=_call(
                                    _attr(_name("__arguments"), "append"),
                                    _call(_name("str"), _name(name)),
                                )
                            )
                        ],
                        orelse=[],
                    )
                )
            else:
                # __arguments.append(str(name))
                body.append(
                    ast.Expr(
                        value=_call(
                            _attr(_name("__arguments"), "append"),
                            _call(_name("str"), _name(name)),
                        )
                    )
                )

        # if __arguments: __final.append('--'); __final.extend(__arguments)
        body.append(
            ast.If(
                test=_name("__arguments"),
                body=[
                    ast.Expr(
                        value=_call(
                            _attr(_name("__final"), "append"),
                            ast.Constant(value="--"),
                        )
                    ),
                    ast.Expr(
                        value=_call(
                            _attr(_name("__final"), "extend"),
                            _name("__arguments"),
                        )
                    ),
                ],
                orelse=[],
            )
        )

    # return " ".join(__final)
    body.append(
        ast.Return(
            value=_call(
                _attr(ast.Constant(value=" "), "join"),
                _name("__final"),
            )
        )
    )

    method_node = ast.FunctionDef(
        name=method_name,
        args=arguments,
        body=body,
        decorator_list=[],
        returns=ast.Name(id="str", ctx=ast.Load()),
        type_comment=None,
    )

    methods.append(method_node)


def _generate_api(structure: Mapping[str, Any], source: list[ast.stmt]) -> None:
    """
    Generate the API class that exposes one method per CLI command.
    """
    api_body: list[ast.stmt] = []

    api_body.append(
        ast.Expr(
            value=ast.Constant(
                value="The low-level Automation API",
            )
        )
    )

    _generate_commands(structure, api_body)

    source.append(
        ast.ClassDef(
            name="API",
            bases=[],
            keywords=[],
            body=api_body,
            decorator_list=[],
        )
    )


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

    module_body: list[ast.stmt] = []

    _generate_options_types(structure, module_body)
    _generate_api(structure, module_body)

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

