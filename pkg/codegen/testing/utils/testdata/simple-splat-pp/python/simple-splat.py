import pulumi
import pulumi_splat as splat

all_keys = splat.get_ssh_keys()
main = splat.Server("main", ssh_keys=[__item.name for __item in all_keys.ssh_keys])
