// Copyright 2016-2021, Pulumi Corporation

using System;
using YamlDotNet.Core;
using YamlDotNet.Core.Events;

namespace Pulumi.Automation.Serialization.Yaml
{
    internal static class YamlScalarExtensions
    {
        public static bool ReadBoolean(this Scalar scalar, string propertyName, Type type)
        {
            if (bool.TryParse(scalar.Value, out var boolean))
                return boolean;

            throw new YamlException($"Unable to deserialize [{type.FullName}]. Exepecting a boolean for [{propertyName}].");
        }
    }
}
