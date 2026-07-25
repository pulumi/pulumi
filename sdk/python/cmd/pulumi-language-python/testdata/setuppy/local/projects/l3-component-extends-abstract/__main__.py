import pulumi
import pulumi_inheritabstract as inheritabstract

child = inheritabstract.ConcreteChild("child",
    seed="s",
    extra="e")
pulumi.export("abstractOutput", child.abstract_output)
pulumi.export("concreteOutput", child.concrete_output)
