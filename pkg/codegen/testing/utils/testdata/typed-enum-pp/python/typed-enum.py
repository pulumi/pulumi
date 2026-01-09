import pulumi
import pulumi_azure_native as azure_native

some_string = "foobar"
type_var = "Block"
staticwebsite = azure_native.storage.StorageAccountStaticWebsite("staticwebsite",
    resource_group_name=some_string,
    account_name=some_string)
# Safe enum
faviconpng = azure_native.storage.Blob("faviconpng",
    resource_group_name=some_string,
    account_name=some_string,
    container_name=some_string,
    type=azure_native.storage.BlobType.BLOCK)
# Output umsafe enum
_404html = azure_native.storage.Blob("_404html",
    resource_group_name=some_string,
    account_name=some_string,
    container_name=some_string,
    type=staticwebsite.index_document)
# Unsafe enum
another = azure_native.storage.Blob("another",
    resource_group_name=some_string,
    account_name=some_string,
    container_name=some_string,
    type=azure_native.storage.BlobType(type_var))
