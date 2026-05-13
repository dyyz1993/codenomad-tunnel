#!/bin/sh
set -e

if [ -f /home/tunnel/.ssh/authorized_keys ]; then
    chown tunnel:tunnel /home/tunnel/.ssh/authorized_keys 2>/dev/null || true
    chmod 600 /home/tunnel/.ssh/authorized_keys 2>/dev/null || true
    echo "SSH authorized_keys loaded."
else
    echo "WARNING: No authorized_keys mounted. SSH login will not work."
fi

echo "Starting SSH daemon on port 2222..."
/usr/sbin/sshd -e -p 2222

echo "Starting tunnel-hub..."
exec /usr/local/bin/tunnel-hub "$@"
