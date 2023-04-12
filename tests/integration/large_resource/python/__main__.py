import pulumi

# Create a very long string (>4mb)
long_string = "a" * 5 * 1024 * 1025

# Create a very deep array (>100 levels)
deep_array = []
for i in range(0, 200):
    deep_array = [deep_array]

pulumi.export("long_string",  long_string)
pulumi.export("deep_array",  deep_array)
