using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Random = Pulumi.Random;

return await Deployment.RunAsync(() => 
{
    var foo = new Random.RandomShuffle("foo", new()
    {
        Inputs = new[]
        {
            @"just one
newline",
            @"foo
bar
baz
qux
quux
qux",
            @"{
    ""a"": 1,
    ""b"": 2,
    ""c"": [
      ""foo"",
      ""bar"",
      ""baz"",
      ""qux"",
      ""quux""
    ]
}
",
        },
    });

});

