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

        /// <summary>
        ///
        /// FullPath of the Pulumi.yaml (or Pulumi.yml, Pulumi.json)
        /// settings file found on disk.
        ///
        /// </summary>
        public string SettingsFileLocation { get; }

        internal ProjectSettingsConflictException(string settingsFileLocation)
            : base($"Custom {nameof(ProjectSettings)} passed in code conflict with settings found on disk: {settingsFileLocation}")
        {
            SettingsFileLocation = settingsFileLocation;
        }
    }
}
