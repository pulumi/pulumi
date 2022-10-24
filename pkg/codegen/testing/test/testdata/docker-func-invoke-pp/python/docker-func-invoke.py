import pulumi
import pulumi_docker as docker

latest = docker.get_remote_image(name="nginx")
ubuntu = docker.RemoteImage("ubuntu", name="ubuntu:precise")
pulumi.export("remoteImageId", latest.id)
pulumi.export("ubuntuImage", ubuntu.name)
