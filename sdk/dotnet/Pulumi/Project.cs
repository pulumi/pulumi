// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    public static class Project
    {
        public static string Name => GlobalOptions.Instance.Project;
    }
}
