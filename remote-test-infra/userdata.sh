#!/bin/bash
# Userdata script for Pulumi test runner EC2 instance.
# Installs build toolchain (Go, Node, Python, mise) and creates testrunner user.
# Logs to /var/log/userdata.log. Writes /tmp/userdata-complete when done.
set -euo pipefail
exec > >(tee -a /var/log/userdata.log) 2>&1
echo "=== userdata.sh starting at $(date) ==="

export DEBIAN_FRONTEND=noninteractive

# ── System packages ──────────────────────────────────────────────────────────
apt-get update -y
apt-get install -y git make gcc curl unzip jq rsync build-essential software-properties-common

# ── Go 1.25 ──────────────────────────────────────────────────────────────────
GO_VERSION="1.25.0"
if [ ! -d /usr/local/go ]; then
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
fi

# ── Node.js 20 ───────────────────────────────────────────────────────────────
if ! command -v node &>/dev/null; then
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
    apt-get install -y nodejs
fi

# ── Python 3.11 ──────────────────────────────────────────────────────────────
if ! command -v python3.11 &>/dev/null; then
    add-apt-repository -y ppa:deadsnakes/ppa
    apt-get update -y
    apt-get install -y python3.11 python3.11-venv python3.11-dev
fi

# ── mise ─────────────────────────────────────────────────────────────────────
if [ ! -f /usr/local/bin/mise ]; then
    curl https://mise.run | MISE_INSTALL_PATH=/usr/local/bin/mise bash
fi

# ── Create testrunner user ───────────────────────────────────────────────────
if ! id testrunner &>/dev/null; then
    useradd -m -s /bin/bash testrunner
fi

# Passwordless sudo
echo "testrunner ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/testrunner
chmod 0440 /etc/sudoers.d/testrunner

# SSH access — cloud-init will have placed the authorized key for ubuntu;
# copy it for testrunner as well.
TESTRUNNER_SSH="/home/testrunner/.ssh"
mkdir -p "$TESTRUNNER_SSH"
if [ -f /home/ubuntu/.ssh/authorized_keys ]; then
    cp /home/ubuntu/.ssh/authorized_keys "$TESTRUNNER_SSH/authorized_keys"
fi
chown -R testrunner:testrunner "$TESTRUNNER_SSH"
chmod 700 "$TESTRUNNER_SSH"
chmod 600 "$TESTRUNNER_SSH/authorized_keys" 2>/dev/null || true

# ── PATH setup ───────────────────────────────────────────────────────────────
cat > /etc/profile.d/testrunner-env.sh << 'ENVEOF'
export PATH="/usr/local/go/bin:$PATH"
export PATH="$HOME/go/bin:$PATH"
export PATH="$HOME/bin:$PATH"
export PATH="$HOME/.local/bin:$PATH"
export GOPATH="$HOME/go"
ENVEOF
chmod 644 /etc/profile.d/testrunner-env.sh

# ── Activate mise for testrunner ─────────────────────────────────────────────
TESTRUNNER_BASHRC="/home/testrunner/.bashrc"
if ! grep -q 'mise activate' "$TESTRUNNER_BASHRC" 2>/dev/null; then
    echo 'eval "$(/usr/local/bin/mise activate bash)"' >> "$TESTRUNNER_BASHRC"
    chown testrunner:testrunner "$TESTRUNNER_BASHRC"
fi

# ── Signal completion ────────────────────────────────────────────────────────
touch /tmp/userdata-complete
echo "=== userdata.sh completed at $(date) ==="
