# Property Paths

[`PropertyValue`s](TODO: document these) in Pulumi are often grouped together either as a map of `PropertyKey` to `PropertyValue`, or as a single object property value.

To address these we can always use `PropertyPath`s.  `PropertyPath`s represent a path to a nested property in a `PropertyValue`.  Examples of valid paths are:

```
root
root.nested
root["nested"]
root.double.nest
root["double"].nest
root["double"]["nest"]
root.array[0]
root.array[100]
root.array[0].nested
root.array[0][1].nested
root.nested.array[0].double[1]
root["key with \"escaped\" quotes"]
root["key with a ."]
["root key with \"escaped\" quotes"].nested
["root key with a ."][100]
root.array[*].field
root.array["*"].field
```

Conveniently we can turn a `map[PropertyKey]PropertyValue` into a `PropertyValue` to be addressed by a `PropertyPath`, by just using `NewObjectProperty`.
