import pulumi
import os

pulumi.export("cwdOutput", os.getcwd())
