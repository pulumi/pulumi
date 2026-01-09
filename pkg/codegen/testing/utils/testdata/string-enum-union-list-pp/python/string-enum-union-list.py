import pulumi
import pulumi_azure_native as azure_native

namespace_authorization_rule = azure_native.servicebus.NamespaceAuthorizationRule("namespaceAuthorizationRule",
    authorization_rule_name="sdk-AuthRules-1788",
    namespace_name="sdk-Namespace-6914",
    resource_group_name="ArunMonocle",
    rights=[
        azure_native.servicebus.AccessRights.LISTEN,
        azure_native.servicebus.AccessRights.SEND,
    ])
