import pulumi
import pulumi_azure_native as azure_native

rawkode_group = azure_native.resources.ResourceGroup("rawkode-group", location="WestUs")
rawkode_storage = azure_native.storage.StorageAccount("rawkode-storage",
    resource_group_name=rawkode_group.name,
    kind="StorageV2",
    sku=azure_native.storage.SkuArgs(
        name="Standard_LRS",
    ))
rawkode_website = azure_native.storage.StorageAccountStaticWebsite("rawkode-website",
    resource_group_name=rawkode_group.name,
    account_name=rawkode_storage.name,
    index_document="index.html",
    error404_document="404.html")
rawkode_index_html = azure_native.storage.Blob("rawkode-index.html",
    resource_group_name=rawkode_group.name,
    account_name=rawkode_storage.name,
    container_name=rawkode_website.container_name,
    content_type="text/html",
    type=azure_native.storage.BlobType.BLOCK,
    source=pulumi.FileAsset("./website/index.html"))
stack72_group = azure_native.resources.ResourceGroup("stack72-group", location="WestUs")
stack72_storage = azure_native.storage.StorageAccount("stack72-storage",
    resource_group_name=stack72_group.name,
    kind="StorageV2",
    sku=azure_native.storage.SkuArgs(
        name="Standard_LRS",
    ))
stack72_website = azure_native.storage.StorageAccountStaticWebsite("stack72-website",
    resource_group_name=stack72_group.name,
    account_name=stack72_storage.name,
    index_document="index.html",
    error404_document="404.html")
stack72_index_html = azure_native.storage.Blob("stack72-index.html",
    resource_group_name=stack72_group.name,
    account_name=stack72_storage.name,
    container_name=stack72_website.container_name,
    content_type="text/html",
    type=azure_native.storage.BlobType.BLOCK,
    source=pulumi.FileAsset("./website/index.html"))
