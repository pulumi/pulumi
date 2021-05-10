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

"""
The automation module contains the Pulumi Automation API, the programmatic interface for driving Pulumi programs
without the CLI.
Generally this can be thought of as encapsulating the functionality of the CLI (`pulumi up`, `pulumi preview`,
`pulumi destroy`, `pulumi stack init`, etc.) but with more flexibility. This still requires a
CLI binary to be installed and available on your $PATH.

In addition to fine-grained building blocks, Automation API provides two out of the box ways to work with Stacks:

1. Programs locally available on-disk and addressed via a filepath (local source)::

    ```python
    stack = create_stack("myOrg/myProj/myStack", work_dir=os.path.join("..", "path", "to", "project"))
    ```

2. Programs defined as a function alongside your Automation API code (inline source)::

    ```python
    def pulumi_program():
        bucket = s3.Bucket("bucket")
        pulumi.export("bucket_name", bucket.Bucket)

    stack = create_stack("myOrg/myProj/myStack", program=pulumi_program)
    ```

Each of these creates a stack with access to the full range of Pulumi lifecycle methods
(up/preview/refresh/destroy), as well as methods for managing config, stack, and project settings::

    stack.set_config("key", ConfigValue(value="value", secret=True))
    preview_response = stack.preview()

The Automation API provides a natural way to orchestrate multiple stacks,
feeding the output of one stack as an input to the next as shown in the package-level example below.
The package can be used for a number of use cases:

- Driving pulumi deployments within CI/CD workflows
- Integration testing
- Multi-stage deployments such as blue-green deployment patterns
- Deployments involving application code like database migrations
- Building higher level tools, custom CLIs over pulumi, etc.
- Using pulumi behind a REST or GRPC API
- Debugging Pulumi programs (by using a single main entrypoint with "inline" programs)

To enable a broad range of runtime customization the API defines a `Workspace` interface.
A Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
Workspaces are used to manage the execution environment, providing various utilities such as plugin
installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
Every Stack including those in the above examples are backed by a Workspace which can be accessed via::

    ws = stack.workspace()
    ws.install_plugin("aws", "v3.20.0")

Workspaces can be explicitly created and customized beyond the three Stack creation helpers noted above::

    ws = LocalWorkspace(work_dir=os.path.join(".", "project", "path"), pulumi_home="~/.pulumi")
    stack = create_stack("org/proj/stack", ws)

A default implementation of workspace is provided as `LocalWorkspace`. This implementation relies on Pulumi.yaml
and Pulumi.[stack].yaml as the intermediate format for Project and Stack settings. Modifying ProjectSettings will
alter the Workspace Pulumi.yaml file, and setting config on a Stack will modify the Pulumi.[stack].yaml file.
This is identical to the behavior of Pulumi CLI driven workspaces. Custom Workspace
implementations can be used to store Project and Stack settings as well as Config in a different format,
such as an in-memory data structure, a shared persistent SQL database, or cloud object storage. Regardless of
the backing Workspace implementation, the Pulumi SaaS Console will still be able to display configuration
applied to updates as it does with the local version of the Workspace today.

The Automation API also provides error handling utilities to detect common cases such as concurrent update
conflicts::

    try:
        up_response = stack.up()
    except ConcurrentUpdateError:
        { /* retry logic here */ }

"""

from ._cmd import (
    CommandResult,
    OnOutput
)

from ._config import (
    ConfigMap,
    ConfigValue
)

# pylint: disable=redefined-builtin
from .errors import (
    StackNotFoundError,
    StackAlreadyExistsError,
    CommandError,
    ConcurrentUpdateError,
    InlineSourceRuntimeError,
    RuntimeError,
    CompilationError,
    InvalidVersionError
)

from .events import (
    CancelEvent,
    DiagnosticEvent,
    DiffKind,
    EngineEvent,
    PolicyEvent,
    PreludeEvent,
    PropertyDiff,
    ResOutputsEvent,
    ResourcePreEvent,
    ResOpFailedEvent,
    StdoutEngineEvent,
    StepEventStateMetadata,
    StepEventMetadata,
    SummaryEvent,
    OpMap,
    OpType
)

from ._local_workspace import (
    LocalWorkspace,
    LocalWorkspaceOptions,
    create_stack,
    select_stack,
    create_or_select_stack
)

from ._workspace import (
    PluginInfo,
    StackSummary,
    PulumiFn,
    Workspace,
    WhoAmIResult,
    Deployment,
)

from ._output import (
    OutputMap,
    OutputValue
)

from ._project_settings import (
    ProjectBackend,
    ProjectSettings,
    ProjectRuntimeInfo,
)

from ._stack_settings import (
    StackSettings
)

from ._stack import (
    OnEvent,
    Stack,
    UpdateSummary,
    UpResult,
    PreviewResult,
    RefreshResult,
    DestroyResult,
    fully_qualified_stack_name,
)

__all__ = [
    # _cmd
    "CommandResult",
    "OnOutput",

    # _config
    "ConfigMap",
    "ConfigValue",

    # errors
    "StackNotFoundError",
    "StackAlreadyExistsError",
    "CommandError",
    "ConcurrentUpdateError",
    "InlineSourceRuntimeError",
    "RuntimeError",
    "CompilationError",
    "InvalidVersionError",

    # events
    "CancelEvent",
    "DiagnosticEvent",
    "DiffKind",
    "EngineEvent",
    "PolicyEvent",
    "PreludeEvent",
    "PropertyDiff",
    "ResOutputsEvent",
    "ResourcePreEvent",
    "ResOpFailedEvent",
    "StdoutEngineEvent",
    "StepEventStateMetadata",
    "StepEventMetadata",
    "SummaryEvent",
    "OpType",
    "OpMap",

    # _local_workspace
    "LocalWorkspace",
    "LocalWorkspaceOptions",
    "create_stack",
    "select_stack",
    "create_or_select_stack",

    # _workspace
    "PluginInfo",
    "StackSummary",
    "PulumiFn",
    "Workspace",
    "Deployment",
    "WhoAmIResult",

    # _output
    "OutputMap",
    "OutputValue",

    # _project_settings
    "ProjectBackend",
    "ProjectSettings",
    "ProjectRuntimeInfo",

    # _stack_settings
    "StackSettings",

    # _stack
    "OnEvent",
    "Stack",
    "UpdateSummary",
    "UpResult",
    "PreviewResult",
    "RefreshResult",
    "DestroyResult",
    "fully_qualified_stack_name",

    # sub-modules
    "errors",
    "events"
]
