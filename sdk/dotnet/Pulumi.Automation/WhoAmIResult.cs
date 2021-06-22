// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public class WhoAmIResult
    {
        public string User { get; }

        public WhoAmIResult(string user)
        {
            this.User = user;
        }
    }
}
