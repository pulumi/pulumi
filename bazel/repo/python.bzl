load(
    "@io_tweag_rules_nixpkgs//nixpkgs:nixpkgs.bzl",
    "nixpkgs_python_configure",
)
load("@rules_python//python:pip.bzl", "pip_parse")

def setup_python(name = "setup_python"):
    nixpkgs_python_configure(
        name = "nixpkgs_python_config",
        repository = "@nixpkgs",
        python3_attribute_path = "python3",
    )

    pip_parse(
        name = "pip",
        python_interpreter_target = "@nixpkgs_python_config_python3//:bin/python",
        requirements_lock = "//:requirements_lock.txt",
    )
