from typing import TypedDict, Union

import pulumi


class Args(TypedDict):
    uni: Union[str, int]  # Unions are not supported


class Component(pulumi.ComponentResource):
    def __init__(self, args: Args): ...
