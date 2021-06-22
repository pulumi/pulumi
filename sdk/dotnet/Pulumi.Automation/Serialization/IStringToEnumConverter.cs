// Copyright 2016-2021, Pulumi Corporation

using System;

namespace Pulumi.Automation.Serialization
{
    internal interface IStringToEnumConverter<out TEnum>
        where TEnum: struct, Enum
    {
        TEnum Convert(string input);
    }
}
