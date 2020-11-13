namespace Pulumi.X.Automation
{
    public class StackInfo
    {
        public string Name { get; }

        public bool IsCurrent { get; }

        public string? LastUpdate { get; }

        public bool IsUpdateInProgress { get; }

        public int? ResourceCount { get; }

        public string? Url { get; }

        public StackInfo(
            string name,
            bool isCurrent,
            string? lastUpdate,
            bool isUpdateInProgress,
            int? resourceCount,
            string? url)
        {
            this.Name = name;
            this.IsCurrent = isCurrent;
            this.LastUpdate = lastUpdate;
            this.IsUpdateInProgress = isUpdateInProgress;
            this.ResourceCount = resourceCount;
            this.Url = url;
        }
    }
}
