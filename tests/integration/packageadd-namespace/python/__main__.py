"""A Python Pulumi program"""

import pulumi
import example_mypkg as mypkg

res = mypkg.Resource("test")
