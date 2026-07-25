"""A customer's OWN gate type -- authored in Python by subclassing delivery.Gate, even though
the delivery package is authored and published in TypeScript. This is the payoff of the
"fully subclassable Gate" wish: the platform team ships the base Gate (the hard part -- the
suspend/resume/status/wake plumbing), and any team extends it with their own governed gate
types, in their own language, without reimplementing that machinery or waiting for the
platform team to add a new gate type upstream.

ChangeWindowGate only opens during an allowed deployment window (e.g. business hours, or a
freeze-aware calendar). It inherits `status`, `resumeWhen`, and `explain()` from Gate; it
supplies the `until` boolean and a time-based `wake`.
"""

import pulumi
import pulumi_delivery as delivery


# A subclass of a generated component declares its own type token via @pulumi.type_token, the
# same mechanism generated classes use, so the engine registers and addresses it as a distinct type.
@pulumi.type_token("acme:delivery:ChangeWindowGate")
class ChangeWindowGate(delivery.Gate):
    def __init__(self, name: str, allowed: pulumi.Input[bool], window: str,
                 opts: pulumi.ResourceOptions | None = None):
        # Compute this gate's boolean and its wake, then hand them to the base Gate, which owns
        # the await/suspend/status behavior. Everything else -- status, resumeWhen, explain() --
        # is inherited.
        super().__init__(
            name,
            delivery.GateArgs(
                until=allowed,
                wake=delivery.WakeArgs(kind="time", params={"window": window}),
            ),
            opts,
        )
