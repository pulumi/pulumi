# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from component import Component


component = Component("component")
result = component.get_message("hello")
