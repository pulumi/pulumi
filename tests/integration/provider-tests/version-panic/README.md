# Repro readme

This repro addresses issue #11287.
A provider can provide an empty version string in its schema. This will cause the engine to crash when trying to use the schema.
