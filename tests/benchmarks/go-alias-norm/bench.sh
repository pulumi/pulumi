#!/usr/bin/env bash

set -euo pipefail

pulumi version

time pulumi destroy --yes

pulumi config set mode new
time pulumi up --yes --skip-preview

pulumi config set mode alias
time pulumi up --yes --skip-preview


export PATH=~/.pulumi-dev/bin:$PATH

pulumi version

time pulumi destroy --yes

pulumi config set mode new
time pulumi up --yes --skip-preview

pulumi config set mode alias
time pulumi up --yes --skip-preview
