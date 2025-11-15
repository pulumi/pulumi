"""A Python Pulumi program"""

import pulumi
import import my_namespace_mypkg as mypkg

res = mypkg.Resource("test")
