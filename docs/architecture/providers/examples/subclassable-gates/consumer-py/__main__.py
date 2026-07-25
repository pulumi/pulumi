"""A rollout that mixes a platform-provided gate with the customer's own gate type. Both are
`delivery.Gate`s, so the rollout treats them uniformly -- and the customer's ChangeWindowGate
inherited all of Gate's suspend/resume/status/explain machinery for free."""

import pulumi
import pulumi_delivery as delivery

from change_window_gate import ChangeWindowGate

# A platform-provided gate: prod waits for a human.
prod_approval = delivery.ApprovalGate(
    "prod-approval",
    delivery.ApprovalGateArgs(
        until=False,
        approvers=["release-captain"],
        instructions="Confirm the canary is healthy before promoting.",
    ),
)

# The customer's own gate type, authored here in Python by subclassing the (TypeScript-authored)
# delivery.Gate. It only opens during the allowed change window.
window = ChangeWindowGate("business-hours", allowed=False, window="Mon-Fri 09:00-17:00 PT")

# Both are Gates; the rollout holds them in one list and reads the inherited surface uniformly.
gates: list[delivery.Gate] = [prod_approval, window]

pulumi.export("holds", pulumi.Output.all(*[g.explain().reason for g in gates]))
pulumi.export("window_status", window.status)          # inherited output
pulumi.export("window_resumes_when", window.resume_when)  # inherited output
