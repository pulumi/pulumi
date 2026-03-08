#!/usr/bin/env python3
from hatchling.builders.wheel import WheelBuilder
from hatchling.bridge.app import Application

app = Application()
builder = WheelBuilder(root=".", app=app)

for included_file in builder.recurse_included_files():
    print(included_file.path)
