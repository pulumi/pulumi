using System.Collections.Generic;
using Pulumi;

return await Deployment.RunAsync(() =>
{
	var stackRef = new Pulumi.Pulumi.StackReference("stackRef", new()
	{
		Name = "foo/bar/dev",
	});

});
