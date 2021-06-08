// Copyright 2016-2019, Pulumi Corporation

using System;

namespace Pulumi
{
    public interface IDeploymentLogger
    {
        void Debug(string message);

        void Info(string message);

        void Warn(string message);

        void Error(string message);

        void Error(Exception exception, string message);
    }
}
