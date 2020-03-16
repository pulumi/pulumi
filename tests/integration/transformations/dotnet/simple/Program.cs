// Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

using System;
using System.Threading.Tasks;
using Pulumi;
using Pulumi.Random;

class MyComponent : ComponentResource
{
    public RandomString Child { get; }
    
    public MyComponent(string name, ComponentResourceOptions? options = null)
        : base("my:component:MyComponent", name, options)
    {
        this.Child = new RandomString($"{name}-child",
            new RandomStringArgs { Length = 5 },
            new CustomResourceOptions {Parent = this, AdditionalSecretOutputs = {"special"} });
    }
}

// Scenario #5 - cross-resource transformations that inject the output of one resource to the input
// of the other one.
class MyOtherComponent : ComponentResource
{
    public RandomString Child1 { get; }
    public RandomString Child2 { get; }
    
    public MyOtherComponent(string name, ComponentResourceOptions? options = null)
        : base("my:component:MyComponent", name, options)
    {
        this.Child1 = new RandomString($"{name}-child1",
            new RandomStringArgs { Length = 5 },
            new CustomResourceOptions { Parent = this });
        
        this.Child2 = new RandomString($"{name}-child2",
            new RandomStringArgs { Length = 6 },
            new CustomResourceOptions { Parent = this });
    }
}

class TransformationsStack : Stack
{   
    public TransformationsStack() : base(new StackOptions { ResourceTransformations = {Scenario3} })
    {
        // Scenario #1 - apply a transformation to a CustomResource
        var res1 = new RandomString("res1", new RandomStringArgs { Length = 5 }, new CustomResourceOptions
        {
            ResourceTransformations =
            { 
                args =>
                {
                    var options = CustomResourceOptions.Merge(
                        (CustomResourceOptions)args.Options,
                        new CustomResourceOptions {AdditionalSecretOutputs = {"length"}});
                    return new ResourceTransformationResult(args.Args, options);
                }
            }
        });
        
        // Scenario #2 - apply a transformation to a Component to transform its children
        var res2 = new MyComponent("res2", new ComponentResourceOptions
        {
            ResourceTransformations =
            {
                args =>
                {
                    if (args.Resource.GetResourceType() == RandomStringType && args.Args is RandomStringArgs oldArgs)
                    {
                        var resultArgs = new RandomStringArgs {Length = oldArgs.Length, MinUpper = 2};
                        var resultOpts = CustomResourceOptions.Merge((CustomResourceOptions)args.Options,
                            new CustomResourceOptions {AdditionalSecretOutputs = {"length"}});
                        return new ResourceTransformationResult(resultArgs, resultOpts);
                    }

                    return null;
                }
            }
        });
        
        // Scenario #3 - apply a transformation to the Stack to transform all resources in the stack.
        var res3 = new RandomString("res3", new RandomStringArgs { Length = 5 });
        
        // Scenario #4 - transformations are applied in order of decreasing specificity
        // 1. (not in this example) Child transformation
        // 2. First parent transformation
        // 3. Second parent transformation
        // 4. Stack transformation
        var res4 = new MyComponent("res4", new ComponentResourceOptions
        {
            ResourceTransformations = { args => scenario4(args, "value1"), args => scenario4(args, "value2") }
        });
        
        ResourceTransformationResult? scenario4(ResourceTransformationArgs args, string v)
        {
            if (args.Resource.GetResourceType() == RandomStringType && args.Args is RandomStringArgs oldArgs)
            {
                var resultArgs = new RandomStringArgs
                    {Length = oldArgs.Length, OverrideSpecial = Output.Format($"{oldArgs.OverrideSpecial}{v}")};
                return new ResourceTransformationResult(resultArgs, args.Options);
            }

            return null;
        }

        // Scenario #5 - cross-resource transformations that inject dependencies on one resource into another.
        var res5 = new MyOtherComponent("res5", new ComponentResourceOptions
        {
            ResourceTransformations = { transformChild1DependsOnChild2() }
        });

        ResourceTransformation transformChild1DependsOnChild2()
        {
            // Create a task completion source that wil be resolved once we find child2.
            // This is needed because we do not know what order we will see the resource
            // registrations of child1 and child2.
            var child2ArgsTaskSource = new TaskCompletionSource<RandomStringArgs>();
            return transform;
            ResourceTransformationResult? transform(ResourceTransformationArgs args)
            {
                // Return a transformation which will rewrite child1 to depend on the promise for child2, and
                // will resolve that promise when it finds child2.
                //return (args: pulumi.ResourceTransformationArgs) => {
                if (args.Args is RandomStringArgs resourceArgs)
                {
                    var resourceName = args.Resource.GetResourceName();
                    if (resourceName.EndsWith("-child2"))
                    {
                        // Resolve the child2 promise with the child2 resource.
                        child2ArgsTaskSource.SetResult(resourceArgs);
                        return null;
                    }

                    if (resourceName.EndsWith("-child1"))
                    {
                        var child2Length = resourceArgs.Length.ToOutput()
                            .Apply(async input =>
                            {
                                if (input != 5)
                                {
                                    // Not strictly necessary - but shows we can confirm invariants we expect to be true.
                                    throw new Exception("unexpected input value");
                                }

                                var child2Args = await child2ArgsTaskSource.Task;
                                return child2Args.Length;
                            })
                            .Apply(output => output);
                        
                        var newArgs = new RandomStringArgs {Length = child2Length};
                        
                        return new ResourceTransformationResult(newArgs, args.Options);
                    }
                }
                
                return null;
            }
        }
    }
        
    // Scenario #3 - apply a transformation to the Stack to transform all (future) resources in the stack
    private static ResourceTransformationResult? Scenario3(ResourceTransformationArgs args)
    {
        if (args.Resource.GetResourceType() == RandomStringType && args.Args is RandomStringArgs oldArgs)
        {
            var resultArgs = new RandomStringArgs
            {
                Length = oldArgs.Length,
                MinUpper = oldArgs.MinUpper,
                OverrideSpecial = Output.Format($"{oldArgs.OverrideSpecial}stackvalue")
            };
            return new ResourceTransformationResult(resultArgs, args.Options);
        }

        return null;
    }   

    private const string RandomStringType = "random:index/randomString:RandomString";
}

class Program
{
    static Task<int> Main(string[] args) => Deployment.RunAsync<TransformationsStack>();
}
