#!/bin/sh
# Bootstraps the box: Docker, swap, rclone, and SSH hardening. Declarative
# enough that "the box died" or "move region" is a terraform apply rather than
# an afternoon of remembering what was installed.
#
# A shell script rather than the cloud-config this used to be: some providers
# (Lightsail among them) prepend their own "#!/bin/sh" preamble to user data,
# so the shebang wins and any cloud-config below it is executed as shell and
# fails. A shebanged script is run correctly everywhere, which keeps this file
# portable if the server moves provider again.
#
# Keep this POSIX sh and idempotent: it may run appended to someone else's
# script, and it is also the recovery path for a box whose first boot failed.
set -eu

export DEBIAN_FRONTEND=noninteractive

# A failed first boot can leave the Docker repo listed but unsigned, which
# makes every later apt-get update fail. Drop the list if its key is missing;
# it is rewritten below.
[ -f /etc/apt/keyrings/docker.asc ] || rm -f /etc/apt/sources.list.d/docker.list

apt-get update
apt-get upgrade -y
apt-get install -y ca-certificates curl gnupg rsync unattended-upgrades unzip

# 2 GB of swap on a 2 GB box turns a would-be OOM kill into a slowdown you can
# observe. Postgres plus three containers has little headroom otherwise.
if [ ! -e /swapfile ]; then
    fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

cat > /etc/ssh/sshd_config.d/99-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
EOF

cat > /etc/cron.d/tenkinow-backup <<'EOF'
# Nightly logical backup to R2 at 03:17 UTC.
17 3 * * * root /srv/tenkinow/backup.sh >> /var/log/tenkinow-backup.log 2>&1
EOF
chmod 0644 /etc/cron.d/tenkinow-backup

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian $(. /etc/os-release && echo "$VERSION_CODENAME") stable" > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker

# Debian ships rclone 1.60 (2022), which 501s against R2 when setting modtime.
# Install the current build instead.
command -v rclone >/dev/null || curl -fsSL https://rclone.org/install.sh | bash

mkdir -p /srv/tenkinow /var/lib/tenkinow/pgdata /var/lib/tenkinow/backups

# Lightsail provisions the SSH key for "admin"; Hetzner provisions it for root.
# Copying it across makes `make deploy` work unchanged on both, and is a no-op
# where /home/admin does not exist.
#
# Overwrite rather than skip: Lightsail ships a root authorized_keys whose
# forced command ("Please login as the user admin") refuses every login, so a
# no-clobber copy leaves root permanently locked out.
if [ -f /home/admin/.ssh/authorized_keys ]; then
    mkdir -p /root/.ssh && chmod 700 /root/.ssh
    cp /home/admin/.ssh/authorized_keys /root/.ssh/authorized_keys
    chmod 600 /root/.ssh/authorized_keys
fi

systemctl restart ssh

echo "tenkinow: provisioning complete"
