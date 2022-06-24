# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from resource import Resource
from component import Component

resource = Resource("resource")

component = Component("component", {
	"message": resource.id.apply(lambda v: f"message {v}"),
	"nested": {
		"value": resource.id.apply(lambda v: f"nested.value {v}"),
	},
})
