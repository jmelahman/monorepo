#!/usr/bin/env bash

set -euo pipefail

if [[ "${DEVCONTAINER_FIREWALL:-true}" == "false" ]]; then
    echo "Firewall disabled (DEVCONTAINER_FIREWALL=false); skipping setup."
    exit 0
fi

echo "Setting up firewall..."

# Only flush the filter table. The nat table holds Docker's redirect for the
# embedded DNS resolver at 127.0.0.11; flushing it breaks DNS for the rest of
# this script and the container.
iptables -F
iptables -X

# Create ipset for allowed destinations
ipset create allowed-domains hash:net || true
ipset flush allowed-domains

# Fetch GitHub IP ranges
GITHUB_IPS=$(curl -s https://api.github.com/meta | jq -r '.api[]' 2>/dev/null || echo "")
for ip in $GITHUB_IPS; do
    ipset add allowed-domains "$ip" 2>/dev/null || true
done

# Resolve allowed domains
ALLOWED_DOMAINS=(
    "github.com"
    "registry.npmjs.org"
    "api.anthropic.com"
    "api-staging.anthropic.com"
    "files.anthropic.com"
    "mcp.vanta.com"
    "app.vanta.com"
    "api.vanta.com"
    "sentry.io"
    "update.code.visualstudio.com"
    "proxy.golang.org"
    "sum.golang.org"
    "storage.googleapis.com"
    "static.rust-lang.org"
    "index.crates.io"
    "static.crates.io"
)

for domain in "${ALLOWED_DOMAINS[@]}"; do
    IPS=$(getent ahosts "$domain" 2>/dev/null | awk '{print $1}' | sort -u || echo "")
    for ip in $IPS; do
        ipset add allowed-domains "$ip/32" 2>/dev/null || true
    done
done

# Detect host network
if [[ "${DOCKER_HOST:-}" == "unix://"* ]]; then
    DOCKER_GATEWAY=$(ip -4 route show | grep "^default" | awk '{print $3}')
    ipset add allowed-domains "$DOCKER_GATEWAY/32" 2>/dev/null || true
fi

# Allow attached docker network subnets (e.g. kanban-net) so we can reach
# sibling containers by service name.
for SUBNET in $(ip -4 route show | awk '/proto kernel/ {print $1}'); do
    ipset add allowed-domains "$SUBNET" 2>/dev/null || true
done

# Set default policies to DROP
iptables -P FORWARD DROP
iptables -P INPUT DROP
iptables -P OUTPUT DROP

# Allow established connections
iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# Allow loopback
iptables -A INPUT -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT

# Allow DNS
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT

# Allow outbound to allowed destinations
iptables -A OUTPUT -m set --match-set allowed-domains dst -j ACCEPT

# Reject unauthorized outbound
iptables -A OUTPUT -j REJECT --reject-with icmp-host-unreachable

# Validate firewall configuration
echo "Validating firewall configuration..."

BLOCKED_SITES=("example.com" "google.com" "facebook.com")
for site in "${BLOCKED_SITES[@]}"; do
    if timeout 2 ping -c 1 "$site" &>/dev/null; then
        echo "Warning: $site is still reachable"
    fi
done

if ! timeout 5 curl -s https://api.github.com/meta > /dev/null; then
    echo "Warning: GitHub API is not accessible"
fi

echo "Firewall setup complete"
