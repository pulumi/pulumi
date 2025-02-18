import pulumi

pulumi.export("rootDirectoryOutput", pulumi.runtime.get_root_directory())
