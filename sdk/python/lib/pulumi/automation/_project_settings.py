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

from typing import (
    Any,
    Dict,
    Iterable,
    Mapping,
    Optional,
    Tuple,
    Union,
)


class ProjectRuntimeInfo:
    """A description of the Project's program runtime and associated metadata."""

    name: str
    options: Optional[Mapping[str, Any]]

    def __init__(self, name: str, options: Optional[Mapping[str, Any]] = None):
        self.name = name
        self.options = options

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> "ProjectRuntimeInfo":
        """Deserialize a ProjectRuntimeInfo from a dictionary."""
        return ProjectRuntimeInfo(
            name=data["name"],
            options=data.get("options"),
        )

    def to_dict(self) -> Dict[str, Any]:
        """Serialize a ProjectRuntimeInfo to a dictionary."""
        return _to_dict(
            [
                ("name", self.name),
                ("options", self.options),
            ]
        )


class ProjectTemplateConfigValue:
    """A placeholder config value for a project template."""

    description: Optional[str]
    default: Optional[str]
    secret: bool

    def __init__(
        self,
        description: Optional[str] = None,
        default: Optional[str] = None,
        secret: bool = False,
    ):
        self.description = description
        self.default = default
        self.secret = secret

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> "ProjectTemplateConfigValue":
        """Deserialize a ProjectTemplateConfigValue from a dictionary."""
        return ProjectTemplateConfigValue(
            description=data.get("description"),
            default=data.get("default"),
            secret=data.get("secret", False),
        )

    def to_dict(self) -> Dict[str, Any]:
        """Serialize a ProjectTemplateConfigValue to a dictionary."""
        return _to_dict(
            [
                ("description", self.description),
                ("default", self.default),
                ("secret", self.secret),
            ]
        )


class ProjectTemplate:
    """A template used to seed new stacks created from this project."""

    description: Optional[str]
    quickstart: Optional[str]
    config: Mapping[str, ProjectTemplateConfigValue]
    important: Optional[bool]

    def __init__(
        self,
        description: Optional[str] = None,
        quickstart: Optional[str] = None,
        config: Optional[Mapping[str, ProjectTemplateConfigValue]] = None,
        important: Optional[bool] = None,
    ):
        self.description = description
        self.quickstart = quickstart
        self.config = config or {}
        self.important = important

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> "ProjectTemplate":
        """Deserialize a ProjectTemplate from a dictionary."""
        return ProjectTemplate(
            description=data.get("description"),
            quickstart=data.get("quickstart"),
            config={
                k: ProjectTemplateConfigValue.from_dict(v)
                for k, v in data.get("config", {}).items()
            },
            important=data.get("important"),
        )

    def to_dict(self) -> Dict[str, Any]:
        """Serialize a ProjectTemplate to a dictionary."""
        return _to_dict(
            [
                ("description", self.description),
                ("quickstart", self.quickstart),
                ("config", {k: v.to_dict() for k, v in self.config.items()}),
                ("important", self.important),
            ]
        )


class ProjectBackend:
    """Configuration for the project's Pulumi state storage backend."""

    url: Optional[str]

    def __init__(self, url: Optional[str] = None):
        self.url = url

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> "ProjectBackend":
        """Deserialize a ProjectBackend from a dictionary."""
        return ProjectBackend(
            url=data.get("url"),
        )

    def to_dict(self) -> Dict[str, Any]:
        """Serialize a ProjectBackend to a dictionary."""
        return _to_dict(
            [
                ("url", self.url),
            ]
        )


class ProjectSettings:
    """A Pulumi project manifest. It describes metadata applying to all sub-stacks created from the project."""

    name: str
    runtime: Union[str, ProjectRuntimeInfo]
    main: Optional[str] = None
    description: Optional[str] = None
    author: Optional[str] = None
    website: Optional[str] = None
    license: Optional[str] = None
    config: Optional[str] = None
    template: Optional[ProjectTemplate] = None
    backend: Optional[ProjectBackend] = None

    def __init__(
        self,
        name: str,
        runtime: Union[str, ProjectRuntimeInfo],
        main: Optional[str] = None,
        description: Optional[str] = None,
        author: Optional[str] = None,
        website: Optional[str] = None,
        license: Optional[str] = None,
        config: Optional[str] = None,
        template: Optional[ProjectTemplate] = None,
        backend: Optional[ProjectBackend] = None,
    ):
        if isinstance(runtime, str) and runtime not in [
            "nodejs",
            "python",
            "go",
            "dotnet",
        ]:
            raise ValueError(
                f"Invalid value {runtime!r} for runtime. "
                f"Must be one of: 'nodejs', 'python', 'go', 'dotnet'."
            )
        self.name = name
        self.runtime = runtime
        self.main = main
        self.description = description
        self.author = author
        self.website = website
        self.license = license
        self.config = config
        self.template = template
        self.backend = backend

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> "ProjectSettings":
        """Deserialize a ProjectSettings from a dictionary."""
        runtime = data["runtime"]
        if isinstance(runtime, dict):
            runtime = ProjectRuntimeInfo.from_dict(runtime)

        template = data.get("template")
        if template is not None:
            template = ProjectTemplate.from_dict(template)

        backend = data.get("backend")
        if backend is not None:
            backend = ProjectBackend.from_dict(backend)

        return ProjectSettings(
            name=data["name"],
            runtime=runtime,
            main=data.get("main"),
            description=data.get("description"),
            author=data.get("author"),
            website=data.get("website"),
            license=data.get("license"),
            config=data.get("config"),
            template=template,
            backend=backend,
        )

    def to_dict(self) -> Dict[str, Any]:
        """Serialize a ProjectSettings to a dictionary."""
        return _to_dict(
            [
                ("name", self.name),
                (
                    "runtime",
                    (
                        self.runtime.to_dict()
                        if isinstance(self.runtime, ProjectRuntimeInfo)
                        else self.runtime
                    ),
                ),
                ("main", self.main),
                ("description", self.description),
                ("author", self.author),
                ("website", self.website),
                ("license", self.license),
                ("config", self.config),
                ("template", self.template.to_dict() if self.template else None),
                ("backend", self.backend.to_dict() if self.backend else None),
            ]
        )


def _to_dict(kvs: Iterable[Tuple[str, Any]]) -> Dict[str, Any]:
    """Convert a list of key-value pairs into a dictionary, filtering out None values."""
    return {k: v for k, v in kvs if v is not None}
