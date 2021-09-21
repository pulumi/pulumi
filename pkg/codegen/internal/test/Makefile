.PHONY = update_random update_aws update_azure update_azure_native update_kubernetes

RANDOM_SCHEMA       := "https://raw.githubusercontent.com/pulumi/pulumi-random/master/provider/cmd/pulumi-resource-random/schema.json"
AWS_SCHEMA          := "https://raw.githubusercontent.com/pulumi/pulumi-aws/master/provider/cmd/pulumi-resource-aws/schema.json"
AZURE_SCHEMA        := "https://raw.githubusercontent.com/pulumi/pulumi-azure/master/provider/cmd/pulumi-resource-azure/schema.json"
AZURE_NATIVE_SCHEMA := "https://raw.githubusercontent.com/pulumi/pulumi-azure-native/master/provider/cmd/pulumi-resource-azure-native/schema.json"
KUBERNETES_SCHEMA   := "https://raw.githubusercontent.com/pulumi/pulumi-kubernetes/master/provider/cmd/pulumi-resource-kubernetes/schema.json"

define add_version
	cd testdata && gsed -i '3 i \ \ \ \ "version": "v$(2)",' $(1).json
endef

update_schema: update_random update_aws update_azure update_azure_native update_kubernetes

update_random:
	cd testdata && curl ${RANDOM_SCHEMA} > random.json
	$(call add_version,random,4.2.0)

update_aws:
	cd testdata && curl ${AWS_SCHEMA} > aws.json
	$(call add_version,aws,4.21.1)

update_azure:
	cd testdata && curl ${AZURE_SCHEMA} > azure.json
	$(call add_version,azure,4.18.0)

update_azure_native:
	cd testdata && curl ${AZURE_NATIVE_SCHEMA} > azure-native.json
	$(call add_version,azure-native,1.29.0)

update_kubernetes:
	cd testdata && curl ${KUBERNETES_SCHEMA} > kubernetes.json
	$(call add_version,kubernetes,3.7.2)
