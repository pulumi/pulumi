// Copyright 2016-2022, Pulumi Corporation

using System;
using System.Linq;

namespace Pulumi.Automation.Tests
{
    public static class Utility
    {
        public static string GetTestSuffix()
        {
            var random = new Random();
            var result = random.Next(); // 31 bits, highest bit will be 0 (signed)
            return result.ToString("x"); // 8 hex characters
        }

        public static string RandomStackName()
        {
            const string chars = "abcdefghijklmnopqrstuvwxyz";
            return new string(Enumerable.Range(1, 8).Select(_ => chars[new Random().Next(chars.Length)]).ToArray());
        }

        public static string GetTestOrg()
            => Environment.GetEnvironmentVariable("PULUMI_TEST_ORG") ?? "pulumi-test";

        public static string FullyQualifiedStackName(string org, string project, string stack)
            => $"{org}/{project}/{stack}";
    }
}
