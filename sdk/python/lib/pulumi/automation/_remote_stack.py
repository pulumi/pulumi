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

from typing import List, Optional

from pulumi.automation._cmd import OnOutput
from pulumi.automation._output import OutputMap
from pulumi.automation._stack import (
    DestroyResult,
    OnEvent,
    PreviewResult,
    RefreshResult,
    Stack,
    UpResult,
    UpdateSummary,
)
from pulumi.automation._workspace import Deployment


class RemoteStack:
    """
    RemoteStack is an isolated, independencly configurable instance of a Pulumi program that is
    operated on remotely (up/preview/refresh/destroy).
    """

    __stack: Stack

    @property
    def name(self) -> str:
        return self.__stack.name

    def __init__(self, stack: Stack):
        self.__stack = stack

    def up(
        self,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
    ) -> UpResult:
        """
        Creates or updates the resources in a stack by executing the program in the Workspace.
        https://www.pulumi.com/docs/cli/commands/pulumi_up/

        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :returns: UpResult
        """
        return self.__stack.up(on_output=on_output, on_event=on_event)

    def preview(
        self,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
    ) -> PreviewResult:
        """
        Performs a dry-run update to a stack, returning pending changes.
        https://www.pulumi.com/docs/cli/commands/pulumi_preview/

        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :returns: PreviewResult
        """
        return self.__stack.preview(on_output=on_output, on_event=on_event)

    def refresh(
        self,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
    ) -> RefreshResult:
        """
        Compares the current stack’s resource state with the state known to exist in the actual
        cloud provider. Any such changes are adopted into the current stack.

        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :returns: RefreshResult
        """
        return self.__stack.refresh(on_output=on_output, on_event=on_event)

    def destroy(
        self,
        on_output: Optional[OnOutput] = None,
        on_event: Optional[OnEvent] = None,
    ) -> DestroyResult:
        """
        Destroy deletes all resources in a stack, leaving all history and configuration intact.

        :param on_output: A function to process the stdout stream.
        :param on_event: A function to process structured events from the Pulumi event stream.
        :returns: DestroyResult
        """
        return self.__stack.destroy(on_output=on_output, on_event=on_event)

    def outputs(self) -> OutputMap:
        """
        Gets the current set of Stack outputs from the last Stack.up().

        :returns: OutputMap
        """
        return self.__stack.outputs()

    def history(
        self,
        page_size: Optional[int] = None,
        page: Optional[int] = None,
    ) -> List[UpdateSummary]:
        """
        Returns a list summarizing all previous and current results from Stack lifecycle operations
        (up/preview/refresh/destroy).

        :param page_size: Paginate history entries (used in combination with page), defaults to all.
        :param page: Paginate history entries (used in combination with page_size), defaults to all.
        :param show_secrets: Show config secrets when they appear in history.

        :returns: List[UpdateSummary]
        """
        # Note: Find a way to allow show_secrets as an option that doesn't require loading the project.
        return self.__stack.history(page_size=page_size, page=page, show_secrets=False)

    def cancel(self) -> None:
        """
        Cancel stops a stack's currently running update. It returns an error if no update is currently running.
        Note that this operation is _very dangerous_, and may leave the stack in an inconsistent state
        if a resource operation was pending when the update was canceled.
        This command is not supported for diy backends.
        """
        self.__stack.cancel()

    def export_stack(self) -> Deployment:
        """
        export_stack exports the deployment state of the stack.
        This can be combined with Stack.import_state to edit a stack's state (such as recovery from failed deployments).

        :returns: Deployment
        """
        return self.__stack.export_stack()

    def import_stack(self, state: Deployment) -> None:
        """
        import_stack imports the specified deployment state into a pre-existing stack.
        This can be combined with Stack.export_state to edit a stack's state (such as recovery from failed deployments).

        :param state: The deployment state to import.
        """
        self.__stack.import_stack(state=state)
