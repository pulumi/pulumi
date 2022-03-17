// Copyright 2016-2019, Pulumi Corporation

// ReSharper disable NotAccessedField.Global
namespace Pulumi
{
    internal class Options
    {
        public readonly bool QueryMode;
        public readonly int Parallel;
        public readonly string? Pwd;
        public readonly string Monitor;
        public readonly string Engine;
        public readonly string? Tracing;

        public Options(bool queryMode, int parallel, string? pwd, string monitor, string engine, string? tracing)
        {
            QueryMode = queryMode;
            Parallel = parallel;
            Pwd = pwd;
            Monitor = monitor;
            Engine = engine;
            Tracing = tracing;
        }
    }
}
