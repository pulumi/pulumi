# Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ReportViolation,
    ResourceValidationArgs,
    ResourceValidationPolicy,
)

def dynamic_no_state_with_value_1(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type == "pulumi-nodejs:dynamic:Resource":
        if "state" in args.props and args.props["state"] == 1:
            report_violation("'state' must not have the value 1.")

def dynamic_no_state_with_value_2(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type == "pulumi-nodejs:dynamic:Resource":
        if "state" in args.props and args.props["state"] == 2:
            report_violation("'state' must not have the value 2.")

def dynamic_no_state_with_value_3(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type == "pulumi-nodejs:dynamic:Resource":
        if "state" in args.props and args.props["state"] == 3:
            report_violation("'state' must not have the value 3.")

def dynamic_no_state_with_value_4(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type == "pulumi-nodejs:dynamic:Resource":
        if "state" in args.props and args.props["state"] == 4:
            report_violation("'state' must not have the value 4.")

# Note: In the NodeJS Policy Pack, this is a strongly-typed policy, but since Python
# does not yet support filtering by type, this is checking the type directly.
def randomuuid_no_keepers(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type == "random:index/randomUuid:RandomUuid":
        if "keepers" not in args.props or not args.props["keepers"]:
            report_violation("RandomUuid must not have an empty 'keepers'.")

def dynamic_no_state_with_value_5(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type == "pulumi-nodejs:dynamic:Resource":
        if "state" in args.props and args.props["state"] == 5:
            report_violation("'state' must not have the value 5.", "some-urn")

def large_resource(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type == "pulumi-nodejs:dynamic:Resource":
        if "state" in args.props and args.props["state"] == 6:
            long_string = "a" * 5 * 1024 * 1024
            expected = len(long_string)
            result = len(args.props["longString"])
            if result != expected:
                report_violation(f"'longString' had expected length of {expected}, got {result}")


PolicyPack(
    name="validate-resource-test-policy",
    enforcement_level=EnforcementLevel.MANDATORY,
    policies=[
        ResourceValidationPolicy(
            name="dynamic-no-state-with-value-1",
            description="Prohibits setting state to 1 on dynamic resources.",
            validate=dynamic_no_state_with_value_1,
        ),
        ResourceValidationPolicy(
            name="dynamic-no-state-with-value-2",
            description="Prohibits setting state to 2 on dynamic resources.",
            validate=dynamic_no_state_with_value_2,
        ),
        ResourceValidationPolicy(
            name="dynamic-no-state-with-value-3-or-4",
            description="Prohibits setting state to 3 or 4 on dynamic resources.",
            validate=[
                dynamic_no_state_with_value_3,
                dynamic_no_state_with_value_4,
            ],
        ),
        ResourceValidationPolicy(
            name="randomuuid-no-keepers",
            description="Prohibits creating a RandomUuid without any 'keepers'.",
            validate=randomuuid_no_keepers,
        ),
        ResourceValidationPolicy(
            name="dynamic-no-state-with-value-5",
            description="Prohibits setting state to 5 on dynamic resources.",
            validate=dynamic_no_state_with_value_5,
        ),
        ResourceValidationPolicy(
            name="large-resource",
            description="Ensures that large string properties are set properly.",
            validate=large_resource,
        )
    ],
)
