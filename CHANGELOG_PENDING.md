### Improvements

- [engine] - Interpret `pluginDownloadURL` as the provider host url when
  downloading plugins.
  [#8544](https://github.com/pulumi/pulumi/pull/8544)

- [sdk/dotnet] - `InputMap` and `InputList` can now be initialized
  with any value that implicitly converts to the collection type.
  These values are then automatically appended, for example:

        var list = new InputList<string>
        {
            "V1",
            Output.Create("V2"),
            new[] { "V3", "V4" },
            new List<string> { "V5", "V6" },
            Output.Create(ImmutableArray.Create("V7", "V8"))
        };

  This feature simplifies the syntax for constructing resources and
  specifying resource options such as the `DependsOn` option.

  [#8498](https://github.com/pulumi/pulumi/pull/8498)

### Bug Fixes

- [sdk/python] - Fixes an issue with stack outputs persisting after
  they are removed from the Pulumi program
  [#8583](https://github.com/pulumi/pulumi/pull/8583)

- [auto/*] - Fixes `stack.setConfig()` breaking when trying to set
  values that look like flags (such as `-value`)
  [#8518](https://github.com/pulumi/pulumi/pull/8614)

- [sdk/dotnet] - Don't throw converting value types that don't match schema
  [#8628](https://github.com/pulumi/pulumi/pull/8628)
