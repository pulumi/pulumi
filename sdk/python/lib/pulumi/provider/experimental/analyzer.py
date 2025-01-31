# Copyright 2025, Pulumi Corporation.
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

import importlib.util
import inspect
import sys
from pathlib import Path
from types import ModuleType

from ...resource import ComponentResource
from .component import ComponentDefinition, PropertyDefinition
from .metadata import Metadata

_NoneType = type(None)  # Available as typing.NoneType in >= 3.10


class Analyzer:
    def __init__(self, metadata: Metadata):
        self.metadata = metadata

    def analyze(self, path: Path) -> dict[str, ComponentDefinition]:
        """
        Analyze walks the directory at `path` and searches for
        ComponentResources in Python files.
        """
        components: dict[str, ComponentDefinition] = {}
        for file_path in self.iter(path):
            components.update(self.analyze_file(file_path))
        return components

    def iter(self, path: Path):
        for file_path in path.glob("**/*.py"):
            if is_in_venv(file_path):
                continue
            yield file_path

    def analyze_file(self, file_path: Path) -> dict[str, ComponentDefinition]:
        components: dict[str, ComponentDefinition] = {}
        module_type = self.load_module(file_path)
        for name in dir(module_type):
            obj = getattr(module_type, name)
            if inspect.isclass(obj) and ComponentResource in obj.__bases__:
                components[name] = self.analyze_component(obj)
        return components

    def find_component(self, path: Path, name: str) -> type[ComponentResource]:
        """
        Find a component by name in the directory at `self.path`.

        :param name: The name of the component to find.
        """
        for file_path in self.iter(path):
            mod = self.load_module(file_path)
            comp = getattr(mod, name, None)
            if comp:
                return comp
        raise Exception(f"Could not find component {name}")

    def load_module(self, file_path: Path) -> ModuleType:
        name = file_path.name.replace(".py", "")
        spec = importlib.util.spec_from_file_location("component_file", file_path)
        if not spec:
            raise Exception(f"Could not load module spec at {file_path}")
        module_type = importlib.util.module_from_spec(spec)
        sys.modules[name] = module_type
        if not spec.loader:
            raise Exception(f"Could not load module at {file_path}")
        spec.loader.exec_module(module_type)
        return module_type

    def analyze_component(
        self, component: type[ComponentResource]
    ) -> ComponentDefinition:
        args = component.__init__.__annotations__.get("args", None)
        if not args:
            raise Exception(
                f"Could not find `args` keyword argument in {component}'s __init__ method"
            )
        return ComponentDefinition(
            description=component.__doc__.strip() if component.__doc__ else None,
            inputs=self.analyze_types(args),
            outputs=self.analyze_types(component),
        )

    def analyze_types(self, typ: type) -> dict[str, PropertyDefinition]:
        return {}


def is_in_venv(path: Path):
    venv = Path(sys.prefix).resolve()
    path = path.resolve()
    return venv in path.parents
