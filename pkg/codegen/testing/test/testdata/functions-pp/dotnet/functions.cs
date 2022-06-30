using System;
using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        var encoded = Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes("haha business"));
        var joined = string.Join("-", 
        {
            "haha",
            "business",
        });
    }

}
