using Pulumi;
using Random = Pulumi.Random;

class MyStack : Stack
{
    public MyStack()
    {
        var randomPassword = new Random.RandomPassword("randomPassword", new Random.RandomPasswordArgs
        {
            Length = 16,
            Special = true,
            OverrideSpecial = "_%@",
        });
        this.Password = randomPassword.Result;
    }

    [Output("password")]
    public Output<string> Password { get; set; }
}
