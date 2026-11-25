from typing import TypedDict

import pulumi
from pulumi.provider.experimental import component_provider_host


class WrapperArgs(TypedDict):
    pass


class Wrapper(pulumi.ComponentResource):
    inner_urn: pulumi.Output[str]

    def __init__(
        self, name: str, args: WrapperArgs, opts: pulumi.ResourceOptions | None = None
    ):
        super().__init__("nested-failure:index:Wrapper", name, args, opts)
        inner = pulumi.ComponentResource(
            "failingcomponent:index:Component",
            f"{name}-inner",
            {},
            pulumi.ResourceOptions(parent=self, version="59.0.0"),
            remote=True,
        )
        self.inner_urn = inner.urn
        self.register_outputs({"inner_urn": self.inner_urn})


if __name__ == "__main__":
    component_provider_host([Wrapper], "nested-failure", version="63.0.0")
