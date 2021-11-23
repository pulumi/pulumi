import os
import shutil

print('== Begin PATH fragments ==')
for sub_path in os.environ['PATH'].split(os.pathsep):
    print(sub_path)
print('== End PATH fragments ==')

print('Locating pulumi executable:')
print(shutil.which('pulumi'))
