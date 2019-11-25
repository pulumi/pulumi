// Copyright 2016-2019, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.IO;
using System.Reflection;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CodeFixes;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using TestHelper;

namespace Pulumi.Analyzers.Test
{
    [TestClass]
    public class UnitTest : CodeFixVerifier
    {
        public static List<string> GetSources()
        {
            var assembly = typeof(UnitTest).Assembly;
            foreach (var resourceName in assembly.GetManifestResourceNames())
            {
                Console.WriteLine("res name: " + resourceName);
            }

            return new List<string>
            {
                ReadStream(assembly, "Input.cs"),
                ReadStream(assembly, "InputList.cs"),
                ReadStream(assembly, "InputMap.cs"),
                ReadStream(assembly, "InputArgs.cs"),
                ReadStream(assembly, "Attributes.cs"),
            };
        }

        private static string ReadStream(Assembly assembly, string name)
        {
            using var stream = assembly.GetManifestResourceStream(name);
            using var reader = new StreamReader(stream ?? throw new NotSupportedException($"Missing embedded {name} file"));
            return reader.ReadToEnd().Trim();
        }

        [TestMethod]
        public void TestOptionalPropNotProvidedWithObjectCreation()
        {
            var sources = GetSources();
            sources.Add(
@"
using Pulumi;
using Pulumi.Serialization;

public class Args : Pulumi.InputArgs
{
    [Input(""name"")]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        new Args {};
    }
}
");

            VerifyCSharpDiagnostic(sources.ToArray());
        }

        [TestMethod]
        public void TestRequiredPropNotProvidedWithObjectCreation()
        {
            var sources = GetSources();
            sources.Insert(0,
@"
using Pulumi;
using Pulumi.Serialization;

public class Args : Pulumi.InputArgs
{
    [Input(""name"", required: true)]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        new Args {};
    }
}
");


            VerifyCSharpDiagnostic(sources.ToArray(), new DiagnosticResult
            {
                Id = CheckRequiredInputsAnalyzer.DiagnosticId,
                Message = string.Format(Resources.AnalyzerMessageFormat, "Name"),
                Severity = DiagnosticSeverity.Warning,
                Locations =
                    new[] { new DiagnosticResultLocation("Test0.cs", 15, 9) }
            });
        }

        [TestMethod]
        public void TestRequiredPropProvidedWithObjectCreation()
        {
            var sources = GetSources();
            sources.Insert(0,
@"
using Pulumi;
using Pulumi.Serialization;

public class Args : Pulumi.InputArgs
{
    [Input(""name"", required: true)]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        new Args { Name = ""a"" };
    }
}
");


            VerifyCSharpDiagnostic(sources.ToArray());
        }

        [TestMethod]
        public void TestOptionalPropNotProvidedWithVariable()
        {
            var sources = GetSources();
            sources.Add(
@"
using Pulumi;
using Pulumi.Serialization;

public class Args : Pulumi.InputArgs
{
    [Input(""name"")]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        var v = new Args {};
    }
}
");

            VerifyCSharpDiagnostic(sources.ToArray());
        }

        [TestMethod]
        public void TestRequiredPropNotProvidedWithVariable()
        {
            var sources = GetSources();
            sources.Insert(0,
@"
using Pulumi;
using Pulumi.Serialization;

public class Args : Pulumi.InputArgs
{
    [Input(""name"", required: true)]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        var v = new Args {};
    }
}
");


            VerifyCSharpDiagnostic(sources.ToArray(), new DiagnosticResult
            {
                Id = CheckRequiredInputsAnalyzer.DiagnosticId,
                Message = string.Format(Resources.AnalyzerMessageFormat, "Name"),
                Severity = DiagnosticSeverity.Warning,
                Locations =
                    new[] { new DiagnosticResultLocation("Test0.cs", 15, 13) }
            });
        }

        [TestMethod]
        public void TestRequiredPropProvidedWithVariable()
        {
            var sources = GetSources();
            sources.Insert(0,
@"
using Pulumi;
using Pulumi.Serialization;

public class Args : Pulumi.InputArgs
{
    [Input(""name"", required: true)]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        var v = new Args { Name = ""a"" };
    }
}
");


            VerifyCSharpDiagnostic(sources.ToArray());
        }

        [TestMethod]
        public void TestRequiredPropProvidedWithVariableLater()
        {
            var sources = GetSources();
            sources.Insert(0,
@"
using Pulumi;
using Pulumi.Serialization;

public class Args : Pulumi.InputArgs
{
    [Input(""name"", required: true)]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        var v = new Args { };
        v.Name = ""a"";
    }
}
");


            VerifyCSharpDiagnostic(sources.ToArray());
        }

        [TestMethod]
        public void TestRequiredPropNotProvidedWithNestedObjectCreation()
        {
            var sources = GetSources();
            sources.Insert(0,
@"
using Pulumi;
using Pulumi.Serialization;

public class Outer : Pulumi.InputArgs
{
    [Input(""inner"")]
    public Input<Args> Inner { get; set; }
}

public class Args : Pulumi.InputArgs
{
    [Input(""name"", required: true)]
    public Input<string> Name { get; set; }
}

class Usage
{
    void M()
    {
        new Outer { Inner = new Args { } }
    }
}
");


            VerifyCSharpDiagnostic(sources.ToArray(), new DiagnosticResult
            {
                Id = CheckRequiredInputsAnalyzer.DiagnosticId,
                Message = string.Format(Resources.AnalyzerMessageFormat, "Name"),
                Severity = DiagnosticSeverity.Warning,
                Locations =
                    new[] { new DiagnosticResultLocation("Test0.cs", 21, 29) }
            });
        }

        protected override CodeFixProvider GetCSharpCodeFixProvider()
            => null;

        protected override DiagnosticAnalyzer GetCSharpDiagnosticAnalyzer()
            => new CheckRequiredInputsAnalyzer();
    }
}
