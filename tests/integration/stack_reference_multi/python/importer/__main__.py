import pulumi

config = pulumi.Config()

def cb(future):
    oldVal = future.result()
    print(oldVal)

exporterStackName = config.require('exporter_stack_name')
org = config.require('org')
a = pulumi.StackReference(f'{org}/exporter/{exporterStackName}')

pulumi.export('val1', a.get_output('val'))
pulumi.export('val2', pulumi.Output.secret(['d', 'x']))


