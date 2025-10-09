import pulumi
import os

pulumi.export("rootDirectoryOutput", pulumi.get_root_directory())
pulumi.export("workingDirectoryOutput", os.getcwd())
