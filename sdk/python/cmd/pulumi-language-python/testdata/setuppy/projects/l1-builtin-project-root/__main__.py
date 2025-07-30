import pulumi

pulumi.export("rootDirectoryOutput", pulumi.get_root_directory())
