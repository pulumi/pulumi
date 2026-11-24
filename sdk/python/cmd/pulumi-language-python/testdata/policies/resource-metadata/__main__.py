# Copyright 2026, Pulumi Corporation.  All rights reserved.

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ResourceValidationPolicy,
    StackValidationPolicy,
)


def attr(obj, *names):
    for name in names:
        if isinstance(obj, dict) and name in obj:
            return obj[name]
        if hasattr(obj, name):
            return getattr(obj, name)
    return None


def resource_type(resource):
    return attr(resource, "resource_type", "type")


def resource_named(resource, name):
    return resource_type(resource) == "simple:index:Resource" and attr(resource, "name") == name


def resource_options(resource):
    return attr(resource, "opts", "options", "resource_options") or {}


def resource_provider(resource):
    return attr(resource, "provider") or {}


def resource_parent(resource):
    return attr(resource, "parent") or attr(resource_options(resource), "parent") or ""


def contains(values, expected):
    return values is not None and not isinstance(values, str) and any(str(value) == expected for value in values)


def urn_string(value):
    return str(attr(value, "urn", "URN") or value)


def contains_urn(values, name):
    return values is not None and not isinstance(values, str) and any(f"::{name}" in urn_string(value) for value in values)


def timeout_create(resource):
    custom_timeouts = attr(resource_options(resource), "custom_timeouts", "customTimeouts") or {}
    return str(attr(custom_timeouts, "create", "create_seconds", "createSeconds") or "")


def property_dependencies(resource, key):
    deps = attr(resource, "property_dependencies", "propertyDependencies") or {}
    if isinstance(deps, dict):
        value = deps.get(key) or {}
        return attr(value, "urns") or value
    if hasattr(deps, "get"):
        return deps.get(key) or []
    return []


def report_if(args, name, ok, report_violation):
    if resource_named(args, name) and ok:
        report_violation("metadata matched")


def validate_identity(args, report_violation):
    props = attr(args, "props") or {}
    report_if(
        args,
        "identity",
        props.get("value") is True and "::identity" in str(attr(args, "urn")),
        report_violation,
    )


def validate_protect(args, report_violation):
    report_if(args, "protected", attr(resource_options(args), "protect") is True, report_violation)


def validate_ignore_changes(args, report_violation):
    report_if(
        args,
        "ignoreChanges",
        contains(attr(resource_options(args), "ignore_changes", "ignoreChanges"), "value"),
        report_violation,
    )


def validate_delete_before_replace(args, report_violation):
    report_if(
        args,
        "deleteBeforeReplace",
        attr(resource_options(args), "delete_before_replace", "deleteBeforeReplace") is True,
        report_violation,
    )


def validate_additional_secret_outputs(args, report_violation):
    report_if(
        args,
        "secretOutput",
        contains(
            attr(resource_options(args), "additional_secret_outputs", "additionalSecretOutputs"),
            "value",
        ),
        report_violation,
    )


def validate_custom_timeouts(args, report_violation):
    report_if(args, "customTimeouts", timeout_create(args) in ["5m", "5m0s", "300", "300.0"], report_violation)


def validate_provider(args, report_violation):
    provider = resource_provider(args)
    report_if(
        args,
        "explicitProvider",
        resource_type(provider) == "pulumi:providers:simple" and attr(provider, "name") == "prov",
        report_violation,
    )


def validate_parent(args, report_violation):
    report_if(args, "child", "::parent" in str(resource_parent(args)), report_violation)


def stack_resource_named(resources, name):
    for resource in resources:
        if resource_named(resource, name):
            return resource
    return None


def validate_dependencies(args, report_violation):
    resource = stack_resource_named(args.resources, "dependsOn")
    if resource is not None and contains_urn(attr(resource, "dependencies"), "dependency"):
        report_violation("metadata matched", attr(resource, "urn"))


def validate_property_dependencies(args, report_violation):
    resource = stack_resource_named(args.resources, "propertyDependency")
    if resource is not None and contains_urn(property_dependencies(resource, "value"), "dependency"):
        report_violation("metadata matched", attr(resource, "urn"))


PolicyPack(
    name="resource-metadata",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="name-type-urn-props",
            description="Resource identity metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_identity,
        ),
        ResourceValidationPolicy(
            name="protect",
            description="Protect option metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_protect,
        ),
        ResourceValidationPolicy(
            name="ignore-changes",
            description="IgnoreChanges option metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_ignore_changes,
        ),
        ResourceValidationPolicy(
            name="delete-before-replace",
            description="DeleteBeforeReplace option metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_delete_before_replace,
        ),
        ResourceValidationPolicy(
            name="additional-secret-outputs",
            description="AdditionalSecretOutputs option metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_additional_secret_outputs,
        ),
        ResourceValidationPolicy(
            name="custom-timeouts",
            description="CustomTimeouts option metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_custom_timeouts,
        ),
        ResourceValidationPolicy(
            name="provider",
            description="Provider metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_provider,
        ),
        ResourceValidationPolicy(
            name="parent",
            description="Parent metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_parent,
        ),
        StackValidationPolicy(
            name="dependencies",
            description="Dependency metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_dependencies,
        ),
        StackValidationPolicy(
            name="property-dependencies",
            description="Property dependency metadata is available",
            enforcement_level=EnforcementLevel.ADVISORY,
            validate=validate_property_dependencies,
        ),
    ],
)
