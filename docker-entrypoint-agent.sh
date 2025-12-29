#!/bin/bash
set -e

echo "Starting strongSwan daemon..."
# Start strongSwan daemon in background
ipsec start --nofork &

# Wait for VICI socket to be available
echo "Waiting for VICI socket..."
for i in {1..30}; do
    if [ -S /var/run/charon.vici ]; then
        echo "VICI socket ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: VICI socket not available after 30 seconds"
        exit 1
    fi
    sleep 1
done

echo "Starting IPsec Agent..."
exec ./ipsec-agent "$@"
