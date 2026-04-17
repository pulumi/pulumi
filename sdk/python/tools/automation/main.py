from __future__ import annotations

import ast
import datetime
import json
import keyword
import os
import sys
from pathlib import Path
from typing import Any, Dict, List, Mapping, Optional, Tuple

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


def _create_command_name(breadcrumbs: List[str]) -> str:
    """Convert a list of subcommand breadcrumbs into the CLI command string."""
    return "pulumi " + " ".join(breadcrumbs)


def _create_options_type_name(breadcrumbs: List[str]) -> str:
    """Convert a list of subcommand breadcrumbs into the options type name."""
    command = "pulumi " + " ".join(breadcrumbs)
    return _pascal_case(command) + "Options"


def _create_method_name(breadcrumbs: List[str]) -> str:
    """Convert a list of subcommand breadcrumbs into a snake_case method name."""
    return _snake_case("_".join(breadcrumbs))


def _base_flag(flag: Mapping[str, Any]) -> Dict[str, Any]:
    """
    Strip omit/preset fields from a flag so that override information doesn't
    leak from parent to child via inheritance.
    """
    return {k: v for k, v in flag.items() if k not in ("omit", "preset")}


def _generate_options_types(
    structure: Mapping[str, Any],
    source: List[ast.stmt],
    breadcrumbs: List[str] = [],
    inherited: Dict[str, Mapping[str, Any]] = {},
) -> None:
    """
    Collect all the flags for the current subcommand, including all the parent flags.
    Only emit options types for structures that will also have a corresponding
    command method (commands and executable menus), skipping non-executable menus
    like the root node.
    """
    command = _create_command_name(breadcrumbs)
    class_name = _create_options_type_name(breadcrumbs)

    all_flags: Dict[str, Mapping[str, Any]] = {
        **inherited,
        **(structure.get("flags") or {}),
    }

    # Only emit options types for nodes that produce command methods.
    should_emit = structure.get("type") == "command" or (
        structure.get("type") == "menu" and structure.get("executable")
    )

    if should_emit:
        # Visible flags (not omitted).
        visible_flags = {k: v for k, v in all_flags.items() if not v.get("omit")}

        flag_items: List[Tuple[str, str, str]] = []
        for flag in visible_flags.values():
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

        class_body: List[ast.stmt] = []

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
                bases=[ast.Name(id="BaseOptions", ctx=ast.Load())],
                keywords=[
                    ast.keyword(arg="total", value=ast.Constant(value=False)),
                ],
                body=class_body,
                decorator_list=[],
            )
        )

    # Recurse into child commands if this node is a menu.
    if structure.get("type") == "menu":
        commands = structure.get("commands") or {}
        child_inherited = {k: _base_flag(v) for k, v in all_flags.items()}
        for name, child in commands.items():
            _generate_options_types(
                child,
                source,
                breadcrumbs=[*breadcrumbs, str(name)],
                inherited=child_inherited,
            )


def _generate_commands(
    structure: Mapping[str, Any],
    methods: List[ast.stmt],
    breadcrumbs: List[str] = [],
    inherited: Dict[str, Mapping[str, Any]] = {},
) -> None:
    """
    Generate command methods on the API class.
    Each method builds a CLI command string from options and arguments.
    """
    all_flags: Dict[str, Mapping[str, Any]] = {
        **inherited,
        **(structure.get("flags") or {}),
    }

    # Recurse into child commands first.
    if structure.get("type") == "menu" and structure.get("commands"):
        child_inherited = {k: _base_flag(v) for k, v in all_flags.items()}
        for name, child in structure["commands"].items():
            _generate_commands(
                child,
                methods,
                breadcrumbs=[*breadcrumbs, str(name)],
                inherited=child_inherited,
            )

    # Non-executable menus don't produce methods.
    if structure.get("type") == "menu" and not structure.get("executable"):
        return

    method_name = _create_method_name(breadcrumbs)
    options_type = _create_options_type_name(breadcrumbs)

    # Build parameter list: self, options, then positional arguments.
    params: List[ast.arg] = [
        ast.arg(arg="self"),
        ast.arg(arg="options", annotation=ast.Name(id=options_type, ctx=ast.Load())),
    ]
    defaults: List[ast.expr] = []

    arg_specs: List[Tuple[str, bool, bool]] = []  # (name, optional, variadic)
    arguments_spec = structure.get("arguments") if structure.get("type") == "command" else None
    vararg: Optional[ast.arg] = None

    if arguments_spec:
        arg_list = arguments_spec.get("arguments", [])
        required_count = arguments_spec.get("requiredArguments", 0)
        is_variadic = arguments_spec.get("variadic", False)

        for i, arg in enumerate(arg_list):
            arg_name = _snake_case(arg.get("name", f"arg{i}"))
            arg_type = arg.get("type", "string")
            optional = i >= required_count
            variadic = i == len(arg_list) - 1 and is_variadic

            if variadic:
                vararg = ast.arg(
                    arg=arg_name,
                    annotation=ast.Name(id=_convert_type(arg_type, False), ctx=ast.Load()),
                )
            else:
                params.append(
                    ast.arg(
                        arg=arg_name,
                        annotation=ast.Name(id=_convert_type(arg_type, False), ctx=ast.Load()),
                    )
                )
                if optional:
                    defaults.append(ast.Constant(value=None))

            arg_specs.append((arg_name, optional, variadic))

    # Build method body.
    body: List[ast.stmt] = []

    # __final = []
    body.append(
        ast.Assign(
            targets=[ast.Name(id="__final", ctx=ast.Store())],
            value=ast.List(elts=[], ctx=ast.Load()),
            lineno=0,
        )
    )
    # __final.append(breadcrumb) for each breadcrumb
    for bc in breadcrumbs:
        body.append(
            ast.Expr(
                value=ast.Call(
                    func=ast.Attribute(
                        value=ast.Name(id="__final", ctx=ast.Load()),
                        attr="append",
                        ctx=ast.Load(),
                    ),
                    args=[ast.Constant(value=bc)],
                    keywords=[],
                )
            )
        )

    # __flags = []
    body.append(
        ast.Assign(
            targets=[ast.Name(id="__flags", ctx=ast.Store())],
            value=ast.List(elts=[], ctx=ast.Load()),
            lineno=0,
        )
    )

    # Preset flags (sorted by name for determinism).
    preset_flags = sorted(
        [f for f in all_flags.values() if "preset" in f],
        key=lambda f: f["name"],
    )
    for flag in preset_flags:
        _emit_preset_flag(body, flag)

    # Option flags (not omitted).
    visible_flags = [f for f in all_flags.values() if not f.get("omit")]
    for flag in visible_flags:
        _emit_option_flag(body, flag)

    # __final.extend(__flags)
    body.append(
        ast.Expr(
            value=ast.Call(
                func=ast.Attribute(
                    value=ast.Name(id="__final", ctx=ast.Load()),
                    attr="extend",
                    ctx=ast.Load(),
                ),
                args=[ast.Name(id="__flags", ctx=ast.Load())],
                keywords=[],
            )
        )
    )

    # __arguments = []
    body.append(
        ast.Assign(
            targets=[ast.Name(id="__arguments", ctx=ast.Store())],
            value=ast.List(elts=[], ctx=ast.Load()),
            lineno=0,
        )
    )

    # Process positional arguments.
    if arguments_spec:
        for arg_name, optional, variadic in arg_specs:
            _emit_argument(body, arg_name, optional, variadic)

    # if __arguments: __final.append("--"); __final.extend(__arguments)
    body.append(
        ast.If(
            test=ast.Name(id="__arguments", ctx=ast.Load()),
            body=[
                ast.Expr(
                    value=ast.Call(
                        func=ast.Attribute(
                            value=ast.Name(id="__final", ctx=ast.Load()),
                            attr="append",
                            ctx=ast.Load(),
                        ),
                        args=[ast.Constant(value="--")],
                        keywords=[],
                    )
                ),
                ast.Expr(
                    value=ast.Call(
                        func=ast.Attribute(
                            value=ast.Name(id="__final", ctx=ast.Load()),
                            attr="extend",
                            ctx=ast.Load(),
                        ),
                        args=[ast.Name(id="__arguments", ctx=ast.Load())],
                        keywords=[],
                    )
                ),
            ],
            orelse=[],
        )
    )

    # return self._run(options, __final)
    body.append(
        ast.Return(
            value=ast.Call(
                func=ast.Attribute(
                    value=ast.Name(id="self", ctx=ast.Load()),
                    attr="_run",
                    ctx=ast.Load(),
                ),
                args=[
                    ast.Name(id="options", ctx=ast.Load()),
                    ast.Name(id="__final", ctx=ast.Load()),
                ],
                keywords=[],
            )
        )
    )

    methods.append(
        ast.FunctionDef(
            name=method_name,
            args=ast.arguments(
                posonlyargs=[],
                args=params,
                vararg=vararg,
                kwonlyargs=[],
                kw_defaults=[],
                defaults=defaults,
            ),
            body=body,
            decorator_list=[],
            returns=None,
        )
    )


def _emit_preset_flag(body: List[ast.stmt], flag: Mapping[str, Any]) -> None:
    """Emit code that adds a preset flag value to __flags."""
    preset = flag.get("preset")
    if preset is None:
        return

    flag_name = flag["name"]
    is_omitted = flag.get("omit", False)

    def make_append_stmts() -> List[ast.stmt]:
        stmts: List[ast.stmt] = []
        if isinstance(preset, bool):
            if preset:
                stmts.append(
                    ast.Expr(
                        value=ast.Call(
                            func=ast.Attribute(
                                value=ast.Name(id="__flags", ctx=ast.Load()),
                                attr="append",
                                ctx=ast.Load(),
                            ),
                            args=[ast.Constant(value=f"--{flag_name}")],
                            keywords=[],
                        )
                    )
                )
        elif isinstance(preset, (str, int)):
            stmts.append(
                ast.Expr(
                    value=ast.Call(
                        func=ast.Attribute(
                            value=ast.Name(id="__flags", ctx=ast.Load()),
                            attr="extend",
                            ctx=ast.Load(),
                        ),
                        args=[
                            ast.List(
                                elts=[
                                    ast.Constant(value=f"--{flag_name}"),
                                    ast.Constant(value=str(preset)),
                                ],
                                ctx=ast.Load(),
                            )
                        ],
                        keywords=[],
                    )
                )
            )
        elif isinstance(preset, list):
            # for __preset in [...]: __flags.extend(["--flag", __preset])
            stmts.append(
                ast.For(
                    target=ast.Name(id="__preset", ctx=ast.Store()),
                    iter=ast.List(
                        elts=[ast.Constant(value=v) for v in preset],
                        ctx=ast.Load(),
                    ),
                    body=[
                        ast.Expr(
                            value=ast.Call(
                                func=ast.Attribute(
                                    value=ast.Name(id="__flags", ctx=ast.Load()),
                                    attr="extend",
                                    ctx=ast.Load(),
                                ),
                                args=[
                                    ast.List(
                                        elts=[
                                            ast.Constant(value=f"--{flag_name}"),
                                            ast.Name(id="__preset", ctx=ast.Load()),
                                        ],
                                        ctx=ast.Load(),
                                    )
                                ],
                                keywords=[],
                            )
                        )
                    ],
                    orelse=[],
                )
            )
        return stmts

    append_stmts = make_append_stmts()
    if not append_stmts:
        return

    if not is_omitted:
        # Wrap in: if options.get("<opt_name>") is None:
        opt_name = _snake_case(flag_name)
        body.append(
            ast.If(
                test=ast.Compare(
                    left=ast.Call(
                        func=ast.Attribute(
                            value=ast.Name(id="options", ctx=ast.Load()),
                            attr="get",
                            ctx=ast.Load(),
                        ),
                        args=[ast.Constant(value=opt_name)],
                        keywords=[],
                    ),
                    ops=[ast.Is()],
                    comparators=[ast.Constant(value=None)],
                ),
                body=append_stmts,
                orelse=[],
            )
        )
    else:
        body.extend(append_stmts)


def _emit_option_flag(body: List[ast.stmt], flag: Mapping[str, Any]) -> None:
    """Emit code that adds a user-supplied option flag to __flags."""
    flag_name = flag["name"]
    flag_type = flag.get("type", "string")
    repeatable = flag.get("repeatable", False)
    required = flag.get("required", False)
    opt_name = _snake_case(flag_name)

    # Access: options.get("opt_name")
    def _opt_access() -> ast.Call:
        return ast.Call(
            func=ast.Attribute(
                value=ast.Name(id="options", ctx=ast.Load()),
                attr="get",
                ctx=ast.Load(),
            ),
            args=[ast.Constant(value=opt_name)],
            keywords=[],
        )

    if repeatable:
        # for __item in options.get("x") or []:
        inner: List[ast.stmt]
        if flag_type == "boolean":
            inner = [
                ast.Expr(
                    value=ast.Call(
                        func=ast.Attribute(
                            value=ast.Name(id="__flags", ctx=ast.Load()),
                            attr="append",
                            ctx=ast.Load(),
                        ),
                        args=[ast.Constant(value=f"--{flag_name}")],
                        keywords=[],
                    )
                )
            ]
        else:
            inner = [
                ast.Expr(
                    value=ast.Call(
                        func=ast.Attribute(
                            value=ast.Name(id="__flags", ctx=ast.Load()),
                            attr="extend",
                            ctx=ast.Load(),
                        ),
                        args=[
                            ast.List(
                                elts=[
                                    ast.Constant(value=f"--{flag_name}"),
                                    ast.Call(
                                        func=ast.Name(id="str", ctx=ast.Load()),
                                        args=[ast.Name(id="__item", ctx=ast.Load())],
                                        keywords=[],
                                    ),
                                ],
                                ctx=ast.Load(),
                            )
                        ],
                        keywords=[],
                    )
                )
            ]

        body.append(
            ast.For(
                target=ast.Name(id="__item", ctx=ast.Store()),
                iter=ast.BoolOp(
                    op=ast.Or(),
                    values=[
                        _opt_access(),
                        ast.List(elts=[], ctx=ast.Load()),
                    ],
                ),
                body=inner,
                orelse=[],
            )
        )
    elif flag_type == "boolean":
        # if options.get("x"): __flags.append("--flag")
        body.append(
            ast.If(
                test=_opt_access(),
                body=[
                    ast.Expr(
                        value=ast.Call(
                            func=ast.Attribute(
                                value=ast.Name(id="__flags", ctx=ast.Load()),
                                attr="append",
                                ctx=ast.Load(),
                            ),
                            args=[ast.Constant(value=f"--{flag_name}")],
                            keywords=[],
                        )
                    )
                ],
                orelse=[],
            )
        )
    elif required:
        # __flags.extend(["--flag", str(options.get("x"))])
        body.append(
            ast.Expr(
                value=ast.Call(
                    func=ast.Attribute(
                        value=ast.Name(id="__flags", ctx=ast.Load()),
                        attr="extend",
                        ctx=ast.Load(),
                    ),
                    args=[
                        ast.List(
                            elts=[
                                ast.Constant(value=f"--{flag_name}"),
                                ast.Call(
                                    func=ast.Name(id="str", ctx=ast.Load()),
                                    args=[_opt_access()],
                                    keywords=[],
                                ),
                            ],
                            ctx=ast.Load(),
                        )
                    ],
                    keywords=[],
                )
            )
        )
    else:
        # if options.get("x") is not None: __flags.extend(["--flag", str(...)])
        body.append(
            ast.If(
                test=ast.Compare(
                    left=_opt_access(),
                    ops=[ast.IsNot()],
                    comparators=[ast.Constant(value=None)],
                ),
                body=[
                    ast.Expr(
                        value=ast.Call(
                            func=ast.Attribute(
                                value=ast.Name(id="__flags", ctx=ast.Load()),
                                attr="extend",
                                ctx=ast.Load(),
                            ),
                            args=[
                                ast.List(
                                    elts=[
                                        ast.Constant(value=f"--{flag_name}"),
                                        ast.Call(
                                            func=ast.Name(id="str", ctx=ast.Load()),
                                            args=[_opt_access()],
                                            keywords=[],
                                        ),
                                    ],
                                    ctx=ast.Load(),
                                )
                            ],
                            keywords=[],
                        )
                    )
                ],
                orelse=[],
            )
        )


def _emit_argument(
    body: List[ast.stmt],
    arg_name: str,
    optional: bool,
    variadic: bool,
) -> None:
    """Emit code that adds a positional argument to __arguments."""
    if optional and not variadic:
        # if arg_name is not None: __arguments.append(str(arg_name))
        body.append(
            ast.If(
                test=ast.Compare(
                    left=ast.Name(id=arg_name, ctx=ast.Load()),
                    ops=[ast.IsNot()],
                    comparators=[ast.Constant(value=None)],
                ),
                body=[
                    ast.Expr(
                        value=ast.Call(
                            func=ast.Attribute(
                                value=ast.Name(id="__arguments", ctx=ast.Load()),
                                attr="append",
                                ctx=ast.Load(),
                            ),
                            args=[
                                ast.Call(
                                    func=ast.Name(id="str", ctx=ast.Load()),
                                    args=[ast.Name(id=arg_name, ctx=ast.Load())],
                                    keywords=[],
                                ),
                            ],
                            keywords=[],
                        )
                    )
                ],
                orelse=[],
            )
        )
    elif variadic:
        # for __item in arg_name: __arguments.append(str(__item))
        body.append(
            ast.For(
                target=ast.Name(id="__item", ctx=ast.Store()),
                iter=ast.Name(id=arg_name, ctx=ast.Load()),
                body=[
                    ast.Expr(
                        value=ast.Call(
                            func=ast.Attribute(
                                value=ast.Name(id="__arguments", ctx=ast.Load()),
                                attr="append",
                                ctx=ast.Load(),
                            ),
                            args=[
                                ast.Call(
                                    func=ast.Name(id="str", ctx=ast.Load()),
                                    args=[ast.Name(id="__item", ctx=ast.Load())],
                                    keywords=[],
                                ),
                            ],
                            keywords=[],
                        )
                    )
                ],
                orelse=[],
            )
        )
    else:
        # __arguments.append(str(arg_name))
        body.append(
            ast.Expr(
                value=ast.Call(
                    func=ast.Attribute(
                        value=ast.Name(id="__arguments", ctx=ast.Load()),
                        attr="append",
                        ctx=ast.Load(),
                    ),
                    args=[
                        ast.Call(
                            func=ast.Name(id="str", ctx=ast.Load()),
                            args=[ast.Name(id=arg_name, ctx=ast.Load())],
                            keywords=[],
                        ),
                    ],
                    keywords=[],
                )
            )
        )


def main(argv: list[str]) -> int:
    usage = "Usage: python main.py <path-to-specification.json> [boilerplate] [output-dir]"

    if len(argv) < 2:
        print(usage, file=sys.stderr)
        return 1

    specification = Path(os.path.abspath(argv[1]))
    if not specification.is_file():
        print(f"Specification file not found: {specification}", file=sys.stderr)
        return 1

    boilerplate_path = Path(os.path.abspath(argv[2])) if len(argv) >= 3 else None
    output_dir = Path(os.path.abspath(argv[3])) if len(argv) >= 4 else Path(os.getcwd()) / "output"

    with specification.open(encoding="utf-8") as f:
        structure = json.load(f)

    output_dir.mkdir(parents=True, exist_ok=True)
    target = output_dir / "main.py"

    # Build the module AST for the generated code.
    module_body: list[ast.stmt] = []

    # Add `from __future__ import annotations` so that forward references in
    # type annotations (e.g. API methods referencing option types defined later)
    # are evaluated lazily.
    module_body.append(
        ast.ImportFrom(
            module="__future__",
            names=[ast.alias(name="annotations")],
            level=0,
        )
    )

    # If a boilerplate file is provided, parse it and use it as the base.
    if boilerplate_path and boilerplate_path.is_file():
        boilerplate_code = boilerplate_path.read_text(encoding="utf-8")
        boilerplate_tree = ast.parse(boilerplate_code)
        module_body.extend(boilerplate_tree.body)

    # Validate that the boilerplate defines a BaseOptions class.
    has_base_options = any(
        isinstance(node, ast.ClassDef) and node.name == "BaseOptions"
        for node in module_body
    )
    if not has_base_options:
        print("Boilerplate must define a `BaseOptions` class.", file=sys.stderr)
        return 1

    # Validate that the boilerplate defines an API class.
    api_class: Optional[ast.ClassDef] = None
    for node in module_body:
        if isinstance(node, ast.ClassDef) and node.name == "API":
            api_class = node
            break

    if api_class is None:
        print("Boilerplate must define an `API` class.", file=sys.stderr)
        return 1

    # Generate options types.
    _generate_options_types(structure, module_body)

    # Generate command methods on the API class.
    methods: List[ast.stmt] = []
    _generate_commands(structure, methods)
    api_class.body.extend(methods)

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
