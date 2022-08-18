using Pulumi;

return await Deployment.RunAsync(() =>
{
	var stackRef = new Pulumi.StackReference("foo/bar/dev");
});

