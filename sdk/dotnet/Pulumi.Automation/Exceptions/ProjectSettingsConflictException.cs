// Copyright 2016-2021, Pulumi Corporation

using System;

namespace Pulumi.Automation.Exceptions
{
    internal class ProjectSettingsConflictException : Exception
    {
        internal string SettingsFileLocation { get; }

        internal ProjectSettingsConflictException(string settingsFileLocation)
            : base(ErrMsg(settingsFileLocation))
        {
            SettingsFileLocation = settingsFileLocation;
        }

        private static string ErrMsg(string settingsFileLocation)
        {
            return String.Format(
                "Custom ProjectSettings passed in code conflict with settings found on disk: {0}",
                settingsFileLocation);
        }
    }
}
