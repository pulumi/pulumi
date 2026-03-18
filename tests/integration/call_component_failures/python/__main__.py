# Copyright 2016, Pulumi Corporation.  All rights reserved.

from component import Component

component = Component("component", foo="bar")

component.get_message()
