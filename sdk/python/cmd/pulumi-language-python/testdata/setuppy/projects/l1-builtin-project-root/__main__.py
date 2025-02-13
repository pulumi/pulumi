import pulumi

pulumi.export("projectRootOutput", pulumi.runtime.get_project_root())
