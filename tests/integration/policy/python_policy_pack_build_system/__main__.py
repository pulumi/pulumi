from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ReportViolation,
    ResourceValidationArgs,
    ResourceValidationPolicy,
)

def noop(args: ResourceValidationArgs, report_violation: ReportViolation):
    pass

noop_policy = ResourceValidationPolicy(
    name="noop_policy",
    description="does nothing!",
    validate=noop
)

PolicyPack(
    name="aws-python",
    enforcement_level=EnforcementLevel.MANDATORY,
    policies=[
        noop_policy,
    ],
)
