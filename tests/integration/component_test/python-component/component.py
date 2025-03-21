from typing import Optional, TypedDict

import pulumi
import pulumi_random as random

class MyComponentArgs(TypedDict):
    who: Optional[pulumi.Input[str]]
    """Who to greet"""

class MyComponent(pulumi.ComponentResource):
    greeting: pulumi.Output[str]
    """ The greeting message """

    def __init__(self, name: str, args: MyComponentArgs, opts: Optional[pulumi.ResourceOptions] = None):
        super().__init__("my-component:index:MyComponent", name, {}, opts)
        who = args.get("who") or "Pulumipus"
        greeting_word = random.RandomShuffle(
           f"{name}-greeting",
           inputs=["Hello", "Bonjour", "Ciao", "Hola"],
           result_count=1,
           opts=pulumi.ResourceOptions(parent=self),
        )
        self.greeting = pulumi.Output.concat(greeting_word.results[0], ", ", who, "!")
        self.register_outputs({
            "greeting": self.greeting,
        })
