#!/usr/bin/env bash
# Idempotent setup: nginx + Let's Encrypt certificate for DevSpeak AI.
# Run once on the EC2 instance. Re-running is safe — the script detects
# existing certificate and skips issuance.
#
# Usage (na EC2):
#   cd ~/devspeak-ai
#   git pull origin main
#   bash infra/scripts/setup-nginx-https.sh

set -euo pipefail

DOMAIN="13-220-172-249.sslip.io"
EMAIL="mariaeduarda.devcloud@gmail.com"
WEBROOT="/var/www/letsencrypt"
CONF_TARGET="/etc/nginx/conf.d/devspeak.conf"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONF_SOURCE="$SCRIPT_DIR/../nginx/devspeak.conf"

echo "==> Installing nginx + certbot..."
sudo amazon-linux-extras install -y nginx1 >/dev/null
sudo amazon-linux-extras install -y epel    >/dev/null
sudo yum install -y certbot                 >/dev/null

echo "==> Preparing ACME webroot..."
sudo mkdir -p "$WEBROOT/.well-known/acme-challenge"

echo "==> Removing default nginx config (if present)..."
sudo rm -f /etc/nginx/conf.d/default.conf

if sudo test -d "/etc/letsencrypt/live/$DOMAIN"; then
    echo "==> Certificate already exists for $DOMAIN — skipping issuance."
    HAS_CERT=1
else
    HAS_CERT=0
fi

if [ "$HAS_CERT" -eq 0 ]; then
    echo "==> Bootstrapping nginx with HTTP-only config for ACME challenge..."
    sudo tee "$CONF_TARGET" > /dev/null <<EOF
server {
    listen 80;
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root $WEBROOT;
    }

    location / {
        return 404;
    }
}
EOF
    sudo systemctl enable --now nginx
    sudo nginx -t
    sudo systemctl reload nginx

    echo "==> Requesting certificate from Let's Encrypt..."
    sudo certbot certonly \
        --webroot -w "$WEBROOT" \
        -d "$DOMAIN" \
        --email "$EMAIL" \
        --agree-tos --non-interactive --no-eff-email
fi

echo "==> Installing full nginx config (HTTPS + reverse proxy)..."
sudo cp "$CONF_SOURCE" "$CONF_TARGET"
sudo nginx -t
sudo systemctl reload nginx

echo "==> Setting up automatic renewal (daily check at 03:00)..."
sudo tee /etc/cron.d/certbot-renew > /dev/null <<'EOF'
0 3 * * * root certbot renew --quiet --post-hook "systemctl reload nginx"
EOF

echo ""
echo "=================================="
echo "Setup complete."
echo "Test: curl -I https://$DOMAIN/health"
echo "=================================="
