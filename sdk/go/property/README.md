# Property Values

The Pulumi value system (formerly `resource.PropertyValue`).

## `property.Value`

### Normalization

`property.Value`, `property.Map` and `property.Array` are all normalized by construction,
which means that you can't construct 2 values with the same semantic meaning but a
different memory representation. Let's look at a couple of examples to understand what
this means:

#### Null Values

`property.New` will always return a null value when given an input that compares as equal
to `nil`. These all result in the same value in memory:

``` go
property.New(([]property.Value)(nil))
property.New((map[string]property.Value)(nil))
property.New(property.Null)
property.Value{}
```

That means that you can safely use these with `reflect.DeepEquals` and `assert.Equal`.

#### Empty Maps and Empty Arrays

In Pulumi's type system, there are empty maps and empty arrays, distinct from null values.

For maps, we have:

``` go
property.New(map[string]property.Value{})
property.New(property.Map{})
```

Constructing a non-empty map and then deleting the inner element will have the same
result. This will return true:

``` go
reflect.DeepEquals(
    property.NewMap(map[string]property.Value{
        "a": property.Value{},
    }).Delete("a"),
    property.Map{},
)
```

For arrays, we have:

``` go
property.New([]property.Value{})
property.New(property.Array{})
```

#### Markers

Any `property.Value` can have 2 kinds of markers: secretness and resource dependencies.

It never matters how these are applied, only what's there. These values are the same:

``` go
property.New("a string").
    WithSecret(true).
    WithDependencies([]urn.URN{urn2, urn1, urn2})

property.WithGoValue(
    property.New(property.Null).
        WithDependencies([]urn.URN{urn1, urn2}).
        WithSecret(true),
    "a string",
)
```

## Relationship to Pulumi's Protobuf Wire Format

`property.Value` represents the semantics of properties sent over the wire. It does not
attempt to faithfully replicate the wire format. This means that a parsing function
`func(*struct.Struct) property.Value` will be surjective relation from `*struct.Struct`
(Pulumi's wire format for property values) to `property.Value`. The function cannot be
injective, since it intentionally unifies elements with equivalent semantics. For example,
consider the following wire values, all of which represent the secret number 42:

``` json
{
    "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270", # Mark as a secret value
    "plaintext": {
        "4dabf18193072939515e22adb298388d": "d0e6a833031e9bbcd3f4e8bde6ca49a4", # Mark as an output value
        "value": 42
    }
}
```

``` json
{
    "4dabf18193072939515e22adb298388d": "d0e6a833031e9bbcd3f4e8bde6ca49a4", # Mark as an output value
    "secret": true,
    "value": 42
}
```

``` json
{
    "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270", # Mark as a secret value
    "plaintext": 42
}
```

Because there isn't a semantic difference between them, `property.Value` will represent
all 3 values in the same way:

``` go
property.New(42.0).WithSecret(true)
```

The goal here is to make life easier for both provider authors and engine maintainers by
lifting the value space above what the wire can represent. This does mean that you
**cannot roundtrip** the wire format through `property.Value`. Given the above example,
`property.New(42.0).WithSecret(true)` would be converted into it's "canonical" wire format
representation:

``` json
{
    "4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270", # Mark this as secret
    "plaintext": 42
}
```
