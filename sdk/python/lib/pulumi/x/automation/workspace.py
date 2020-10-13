from abc import ABC, abstractmethod
from typing import (
    TypeVar,
    Generic,
    Set,
    Callable,
    Awaitable,
    Union,
    cast,
    Mapping,
    Any,
    List,
    Optional,
)


class Workspace(ABC):
    """
    Workspace is the execution context containing a single Pulumi project, a program, and multiple stacks.
    Workspaces are used to manage the execution environment, providing various utilities such as plugin
    installation, environment configuration ($PULUMI_HOME), and creation, deletion, and listing of Stacks.
    """

    work_dir: str
    """
    The working directory to run Pulumi CLI commands
    """

    pulumi_home: Optional[str]
    """
    The directory override for CLI metadata if set.
    This customizes the location of $PULUMI_HOME where metadata is stored and plugins are installed.
    """

    secrets_provider: Optional[str]
    """
    The secrets provider to use for encryption and decryption of stack secrets.
    See: https://www.pulumi.com/docs/intro/concepts/config/#available-encryption-providers
    """

    # TODO improve typing to encapsulate stack exports
    program: Optional[Callable[[], Any]]
    """
    The inline program `PulumiFn` to be used for Preview/Update operations if any.
    If none is specified, the stack will refer to ProjectSettings for this information.
    """

    env_vars: Optional[Mapping[str, str]]
    """
    Environment values scoped to the current workspace. These will be supplied to every Pulumi command.
    """

    @abstractmethod
    async def create_stack(self, name: str) -> None:
        """
        Creates and sets a new stack with the stack name, failing if one already exists.

        :param str name: The name of the stack to create
        :return: None, but throws a CommandException if the operation was unsuccessful for any reason
        :rtype: None
        """
        pass
