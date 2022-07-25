FROM gitpod/workspace-full
USER gitpod

ENV PATH="$HOME/.pulumi:$HOME/.pulumi/bin:$PATH"

# Install .NET 6SDK binaries on Ubuntu
# Based on: https://dev.to/carlos487/installing-dotnet-core-in-ubuntu-20-04-6jh
RUN mkdir -p /home/gitpod/dotnet && \
     curl -fsSL https://download.visualstudio.microsoft.com/download/pr/0e83f50a-0619-45e6-8f16-dc4f41d1bb16/e0de908b2f070ef9e7e3b6ddea9d268c/dotnet-sdk-6.0.302-linux-x64.tar.gz \
     | tar xz -C /home/gitpod/dotnet && \
     wget -qO- https://github.com/pulumi/pulumictl/releases/download/v0.0.28/pulumictl-v0.0.28-linux-amd64.tar.gz | sudo tar zxvf - -C /usr/local/bin


ENV DOTNET_ROOT=/home/gitpod/dotnet
ENV PATH=$PATH:/home/gitpod/dotnet:/usr/local/bin

