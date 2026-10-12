#!/bin/sh
# Publishes the dashboard and apps to the tailnet over HTTPS. Run as root.
# The configuration is kept by tailscaled, so this is only needed again
# after changing it.
set -eu

# Starts from nothing, which also drops anything served by hand.
tailscale serve reset
# nginx routes each path to its app.
tailscale serve --bg http://127.0.0.1:80
# kanban only works at the root of a host, so it gets a port of its own.
tailscale serve --bg --https=8443 http://127.0.0.1:7474
tailscale serve status
