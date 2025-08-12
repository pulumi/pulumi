# Copyright 2016-2022, Pulumi Corporation.
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

from typing import List, Mapping, Optional, Union

from pulumi.automation._local_workspace import ExecutorImage, LocalWorkspace, Secret
from pulumi.automation._remote_stack import RemoteStack
from pulumi.automation._stack import Stack


class RemoteWorkspaceOptions:
    """
    Extensibility options to configure a RemoteWorkspace.
    """

    env_vars: Optional[Mapping[str, Union[str, Secret]]]
    pre_run_commands: Optional[List[str]]
    skip_install_dependencies: Optional[bool]
    executor_image: Optional[ExecutorImage]
    inherit_settings: Optional[bool]

    def __init__(
        self,
        *,
        env_vars: Optional[Mapping[str, Union[str, Secret]]] = None,
        pre_run_commands: Optional[List[str]] = None,
        skip_install_dependencies: Optional[bool] = None,
        executor_image: Optional[ExecutorImage] = None,
        inherit_settings: Optional[bool] = None,
    ):
        self.env_vars = env_vars
        self.pre_run_commands = pre_run_commands
        self.skip_install_dependencies = skip_install_dependencies
        self.executor_image = executor_image
        self.inherit_settings = inherit_settings


class RemoteGitAuth:
    """
    Authentication options for the repository that can be specified for a private Git repo.
    There are three different authentication paths:

     - Personal accesstoken
     - SSH private key (and its optional password)
     - Basic auth username and password

    Only one authentication path is valid.
    """

    ssh_private_key_path: Optional[str]
    """
    The absolute path to a private key for access to the git repo.
    """

    ssh_private_key: Optional[str]
    """
    The (contents) private key for access to the git repo.
    """

    password: Optional[str]
    """
    The password that pairs with a username or as part of an SSH Private Key.
    """

    personal_access_token: Optional[str]
    """
    A Git personal access token in replacement of your password.
    """

    username: Optional[str]
    """
    The username to use when authenticating to a git repository.
    """

    def __init__(
        self,
        *,
        ssh_private_key_path: Optional[str] = None,
        ssh_private_key: Optional[str] = None,
        password: Optional[str] = None,
        personal_access_token: Optional[str] = None,
        username: Optional[str] = None,
    ):
        self.ssh_private_key_path = ssh_private_key_path
        self.ssh_private_key = ssh_private_key
        self.password = password
        self.personal_access_token = personal_access_token
        self.username = username


def create_remote_stack_git_source(
    stack_name: str,
    url: Optional[str] = None,
    *,
    branch: Optional[str] = None,
    commit_hash: Optional[str] = None,
    project_path: Optional[str] = None,
    auth: Optional[RemoteGitAuth] = None,
    opts: Optional[RemoteWorkspaceOptions] = None,
) -> RemoteStack:
    """
    PREVIEW: Creates a Stack backed by a RemoteWorkspace with source code from the specified Git repository.
    Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
    """
    if not _is_fully_qualified_stack_name(stack_name):
        raise Exception(f'stack name "{stack_name}" must be fully qualified.')

    ws = _create_local_workspace(
        url=url,
        project_path=project_path,
        branch=branch,
        commit_hash=commit_hash,
        auth=auth,
        opts=opts,
    )
    stack = Stack.create(stack_name, ws)
    return RemoteStack(stack)


def create_or_select_remote_stack_git_source(
    stack_name: str,
    url: Optional[str] = None,
    *,
    branch: Optional[str] = None,
    commit_hash: Optional[str] = None,
    project_path: Optional[str] = None,
    auth: Optional[RemoteGitAuth] = None,
    opts: Optional[RemoteWorkspaceOptions] = None,
) -> RemoteStack:
    """
    PREVIEW: Creates or selects an existing Stack backed by a RemoteWorkspace with source code from the specified
    Git repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
    """
    if not _is_fully_qualified_stack_name(stack_name):
        raise Exception(f'stack name "{stack_name}" must be fully qualified.')

    ws = _create_local_workspace(
        url=url,
        project_path=project_path,
        branch=branch,
        commit_hash=commit_hash,
        auth=auth,
        opts=opts,
    )
    stack = Stack.create_or_select(stack_name, ws)
    return RemoteStack(stack)


def select_remote_stack_git_source(
    stack_name: str,
    url: Optional[str] = None,
    *,
    branch: Optional[str] = None,
    commit_hash: Optional[str] = None,
    project_path: Optional[str] = None,
    auth: Optional[RemoteGitAuth] = None,
    opts: Optional[RemoteWorkspaceOptions] = None,
) -> RemoteStack:
    """
    PREVIEW: Creates or selects an existing Stack backed by a RemoteWorkspace with source code from the specified
    Git repository. Pulumi operations on the stack (Preview, Update, Refresh, and Destroy) are performed remotely.
    """
    if not _is_fully_qualified_stack_name(stack_name):
        raise Exception(f'stack name "{stack_name}" must be fully qualified.')

    ws = _create_local_workspace(
        url=url,
        project_path=project_path,
        branch=branch,
        commit_hash=commit_hash,
        auth=auth,
        opts=opts,
    )
    stack = Stack.select(stack_name, ws)
    return RemoteStack(stack)


def _create_local_workspace(
    url: Optional[str],
    branch: Optional[str] = None,
    commit_hash: Optional[str] = None,
    project_path: Optional[str] = None,
    auth: Optional[RemoteGitAuth] = None,
    opts: Optional[RemoteWorkspaceOptions] = None,
) -> LocalWorkspace:
    env_vars = None
    pre_run_commands = None
    skip_install_dependencies = None
    remote_executor_image = None
    inherit_settings = None
    if opts is not None:
        env_vars = opts.env_vars
        pre_run_commands = opts.pre_run_commands
        skip_install_dependencies = opts.skip_install_dependencies
        remote_executor_image = opts.executor_image
        inherit_settings = opts.inherit_settings

    if not url and not inherit_settings:
        raise Exception("url is required.")
    if branch and commit_hash:
        raise Exception("branch and commit_hash cannot both be specified.")
    if not branch and not commit_hash and not inherit_settings:
        raise Exception("either branch or commit_hash is required.")
    if auth is not None:
        if auth.ssh_private_key and auth.ssh_private_key_path:
            raise Exception(
                "ssh_private_key and ssh_private_key_path cannot both be specified."
            )

    ws = LocalWorkspace()
    ws._remote = True
    ws._remote_env_vars = env_vars
    ws._remote_pre_run_commands = pre_run_commands
    ws._remote_skip_install_dependencies = skip_install_dependencies
    ws._remote_executor_image = remote_executor_image
    ws._remote_inherit_settings = inherit_settings
    ws._remote_git_url = url
    ws._remote_git_project_path = project_path
    ws._remote_git_branch = branch
    ws._remote_git_commit_hash = commit_hash
    ws._remote_git_auth = auth

    # Ensure the CLI supports the commands being executed.
    if not ws._version_check_opt_out():
        if not ws._remote_supported():
            raise Exception(
                "The Pulumi CLI version does not support remote operations. Please upgrade."
            )
        if inherit_settings and not ws._remote_inherit_settings_supported():
            raise Exception(
                "The Pulumi CLI version does not support remote inherit settings. Please upgrade."
            )

    return ws


def _is_fully_qualified_stack_name(stack: str) -> bool:
    split = stack.split("/")
    return len(split) == 3 and split[0] != "" and split[1] != "" and split[2] != ""
