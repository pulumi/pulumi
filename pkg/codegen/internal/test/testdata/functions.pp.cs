using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        var encoded = Convert.ToBase64String(%v)("haha business");
        var joined = string.Join("-", 
        {
            "haha",
            "business",
        });
    }

}
