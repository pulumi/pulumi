// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation.Serialization.Json
{
    internal interface IJsonModel<out T>
    {
        T Convert();
    }
}
