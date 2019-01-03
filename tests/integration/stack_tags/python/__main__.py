# Copyright 2016-2019, Pulumi Corporation.  All rights reserved.

import pulumi

config = pulumi.Config('stack_tags_py')
customtag = config.require_bool('customtag')

expected = {
    'pulumi:project': 'stack_tags_py',
    'pulumi:runtime': 'python',
    'pulumi:description': 'A simple Python program that uses stack tags'
}
if customtag:
    expected['foo'] = 'bar'

for name, value in expected.items():
    assert pulumi.get_stack_tag(name) == value

tags = pulumi.get_stack_tags()
for name, value in expected.items():
    assert tags[name] == value
