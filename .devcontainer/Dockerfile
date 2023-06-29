FROM pulumi/pulumi

# Install golangci-lint
RUN version=1.42.1 \
    && curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /go/bin v$version \
    && golangci-lint version

# Install pulumictl
RUN version=0.0.42 \
    && curl -fsSLO https://github.com/pulumi/pulumictl/releases/download/v$version/pulumictl-v$version-linux-amd64.tar.gz \
    && tar -xzf pulumictl-v$version-linux-amd64.tar.gz --directory /usr/local/bin --no-same-owner pulumictl \
    && rm -f pulumictl-v$version-linux-amd64.tar.gz \
    && pulumictl version

# Add non-root user
ARG USER_NAME=user
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USER_NAME \
    && useradd --uid $USER_UID --gid $USER_GID --shell /bin/bash -m $USER_NAME \
    && mkdir -p /etc/sudoers.d \
    && echo "$USER_NAME ALL=(ALL:ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER_NAME \
    && chmod 0440 /etc/sudoers.d/$USER_NAME

RUN mkdir -p /go/bin \
    && chown -R $USER_NAME: /go \
    && mkdir -p $HOME/.pulumi/bin \
    && chown -R $USER_NAME: $HOME/.pulumi

USER $USER_NAME

# The base container sets XDG_CACHE_HOME XDG_CONFIG_HOME specifically for the root user, we can't unset them in a way that vscode will pick up, so we set them to values for the new user.
# Installing go extensions via vscode use these paths so if we just leave it set to /root/.cache we'll get permission errors.
ENV XDG_CONFIG_HOME=/home/$USER_NAME/.config
ENV XDG_CACHE_HOME=/home/$USER_NAME/.cache

RUN echo "export PATH=$HOME/.pulumi:$HOME/.pulumi/bin:$GOPATH/bin:/usr/local/go/bin:$PATH" >> ~/.bashrc \
    && echo "alias l='ls -aF'" >> ~/.bash_aliases \
    && echo "alias ll='ls -ahlF'" >> ~/.bash_aliases \
    && echo "alias ls='ls --color=auto --group-directories-first'" >> ~/.bash_aliases
