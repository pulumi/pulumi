import os

for sub_path in os.environ['PATH'].split(os.pathsep):
    print(sub_path)
