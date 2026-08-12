import pulumi
import pulumi_simple as simple

import_ = simple.Resource("import", value=True,
opts = pulumi.ResourceOptions(import_="fakeID123"))
not_import = simple.Resource("notImport", value=True)
