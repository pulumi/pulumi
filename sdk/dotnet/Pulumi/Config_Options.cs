// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System.Collections.Generic;
using System.Text.RegularExpressions;

namespace Pulumi
{
    public partial class Config
    {
        /// <summary>
        /// StringOptions may be used to constrain the set of legal values a string config value may contain.
        /// </summary>
        public class StringOptions
        {
            /// <summary>
            /// The legal enum values. If it does not match, a ConfigEnumError is thrown.
            /// </summary>
            public ISet<string> AllowedValues = new HashSet<string>();

            /// <summary>
            /// The minimum string length. If the string is not this long, a ConfigRangeError is thrown.
            /// </summary>
            public int? MinLength { get; }

            /// <summary>
            /// The maximum string length. If the string is longer than this, a ConfigRangeError is thrown.
            /// </summary>
            public int? MaxLength;

            /// <summary>
            /// A regular expression the string must match. If it does not match, a ConfigPatternError is thrown.
            /// </summary>
            public Regex? Pattern;
        }

        /// <summary>
        /// Int32Options may be used to constrain the set of legal values a number config value may contain.
        /// </summary>
        public class Int32Options
        {
            /// <summary>
            /// The minimum number value, inclusive. If the number is less than this, a ConfigRangeError
            /// is thrown.
            /// </summary>
            public int? Min;

            /// <summary>
            /// The maximum number value, inclusive. If the number is greater than this, a
            /// ConfigRangeError is thrown.
            /// </summary>
            public int? Max;
        }
    }
}