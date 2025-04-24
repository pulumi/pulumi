# Copyright 2024, Pulumi Corporation.

import os
import sys
from pathlib import Path

import pulumi.provider as provider


class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self):
        super().__init__(Provider.VERSION)

    def construct(self, name, resource_type, inputs, options):
        return provider.ConstructResult("", {"ITS_ALIVE": "IT'S ALIVE!"})


if __name__ == "__main__":
    provider_dir = Path(__file__).absolute().parent
    expected_venv = provider_dir / "venv"
    venv = os.getenv("VIRTUAL_ENV", "")
    assert (
        Path(venv) == expected_venv
    ), f"Expected VIRTUAL_ENV to be {expected_venv}, got {venv}"
    provider.main(Provider(), sys.argv[1:])
