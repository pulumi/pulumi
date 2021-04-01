// Copyright 2016-2021, Pulumi Corporation

using System;

namespace Pulumi.Automation.Exceptions
{
    /// <summary>
    ///
    /// Thrown when creating a Workspace detects a conflict between
    /// project settings found on disk (such as Pulumi.yaml) and a
    /// ProjectSettings object passed to the Create API.
    ///
    /// There are two resolutions:
    ///
    /// (A) to use the ProjectSettings, delete the Pulumi.yaml file
    ///     from WorkDir or use a different WorkDir
    ///
    /// (B) to use the exiting Pulumi.yaml from WorkDir, avoid
    ///     customizing the ProjectSettings
    ///
    /// </summary>
    public class ProjectSettingsConflictException : Exception
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
