resource namespaceAuthorizationRule "azure-native:servicebus:NamespaceAuthorizationRule" {
	authorizationRuleName = "sdk-AuthRules-1788"
	namespaceName = "sdk-Namespace-6914"
	resourceGroupName = "ArunMonocle"
	rights = [
		"Listen",
		"Send"
	]
}
