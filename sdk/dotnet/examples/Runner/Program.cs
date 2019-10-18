using System;
using System.Collections.Generic;

namespace Pulumi.AzureExamples
{
    class Program
    {
        class Example
        {
            public string Name;
            public Func<IDictionary<string, Output<string>>> Run;
        }

        static void Main(string[] args)
        {
            var examples = new Example[] 
            { 
                new Example { Name = "Minimal C#", Run = CSharpExamples.Minimal.Run },
                new Example { Name = "Web App C#", Run = CSharpExamples.WebApp.Run },
                new Example { Name = "Minimal F#", Run = FSharpExamples.Minimal.run },
                new Example { Name = "Minimal F# based on C/E", Run = FSharpExamples.MinimalCE.run },
                new Example { Name = "Minimal VB.NET", Run = VBExamples.Minimal.Run },
            };

            foreach (var example in examples)
            {
                Console.WriteLine($"Updating ({example.Name}):\n");
                Console.WriteLine("    Type                      Name        Status");
                var exports = example.Run();
                Console.WriteLine("\nOutputs:");
                foreach (var v in exports)
                {
                    Console.WriteLine($"  + {v.Key}: {v.Value}");
                }
                Console.WriteLine("\n\n");
            }
        }
    }
}
