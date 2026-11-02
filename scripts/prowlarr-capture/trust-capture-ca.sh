#!/bin/sh
set -eu
# LinuxServer runs custom-cont-init.d as root before starting Prowlarr.
# Trust only this isolated recorder's CA; leave certificate validation enabled.
cp /capture-ca/mitmproxy-ca.crt /usr/local/share/ca-certificates/mediaz-capture.crt
update-ca-certificates
