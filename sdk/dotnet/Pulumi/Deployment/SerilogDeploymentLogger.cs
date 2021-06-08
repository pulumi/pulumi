// Copyright 2016-2019, Pulumi Corporation

using System;
using Microsoft.Extensions.Configuration;
using Serilog;
using Serilog.Events;

namespace Pulumi
{
    internal class SerilogDeploymentLogger : IDeploymentLogger
    {
        private readonly ILogger _logger;

        private SerilogDeploymentLogger(ILogger logger)
            => _logger = logger;

        public void Debug(string message)
            => _logger.Debug(message);

        public void Error(string message)
            => _logger.Error(message);

        public void Error(Exception exception, string message)
            => _logger.Error(exception, message);

        public void Info(string message)
            => _logger.Information(message);

        public void Warn(string message)
            => _logger.Warning(message);

        public static IDeploymentLogger Create()
        {
            var verboseLogging = !string.IsNullOrEmpty(Environment.GetEnvironmentVariable("PULUMI_DOTNET_LOG_VERBOSE"));

            var configRoot = new ConfigurationBuilder()
                .AddEnvironmentVariables()
                .Build();

            var logger = new LoggerConfiguration()
                .MinimumLevel.Is(verboseLogging ? LogEventLevel.Verbose : LogEventLevel.Fatal)
                .ReadFrom.Configuration(configRoot)
                .WriteTo.Console()
                .CreateLogger()
                .ForContext<Deployment>();

            return new SerilogDeploymentLogger(logger);
        }
    }
}
