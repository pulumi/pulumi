from pulumi import ResourceOptions, Input, Inputs
from typing import Optional


class ConstructResult:
    """ConstructResult represents the results of a call to
`Provider.construct`.

    """

    urn: Input[str]
    """The URN of the constructed resource."""

    state: Inputs
    """Any state that was computed during construction."""

    def __init__(self, urn: Input[str], state: Inputs) -> None:
        self.urn = urn
        self.state = state


class Provider:
    """Provider represents an object that implements the resources and
    functions for a particular Pulumi package.

    """

    version: str

    def __init__(self, version: str) -> None:
        self.version = version

    def construct(self, name: str, type: str, inputs: Inputs,
                  options: Optional[ResourceOptions]=None) -> ConstructResult:
        """Construct creates a new component resource.

        :param name str: The name of the resource to create.
        :param type str: The type of the resource to create.
        :param inputs Inputs: The inputs to the resource.
        :param options Optional[ResourceOptions] The options for the resource.
        """

        raise Exception("Subclass of Provider must implement 'construct'")
