#!/usr/bin/env bash
# Idempotent setup: nginx + Let's Encrypt para o DevSpeak AI.
# Re-execução é segura — pula a emissão se o certificado já existe.
#
# Uso (na EC2):
#   cd ~/devspeak-ai
#   git pull origin main
#
#   # 1) Passando via env:
#   DOMAIN=devspeak.exemplo.com EMAIL=voce@exemplo.com bash infra/scripts/setup-nginx-https.sh
#
#   # 2) Ou como argumentos posicionais:
#   bash infra/scripts/setup-nginx-https.sh devspeak.exemplo.com voce@exemplo.com
#
# Pré-requisitos:
#   - DOMAIN com registro A apontando para o IP público da EC2
#   - portas 80 e 443 abertas no security group
#   - o container speech-service já rodando em 127.0.0.1:8080
#     (o workflow de deploy passa a usar -p 127.0.0.1:8080:8080 quando Nginx está na frente)

set -euo pipefail

DOMAIN="${1:-${DOMAIN:-}}"
EMAIL="${2:-${EMAIL:-}}"

if [[ -z "$DOMAIN" || -z "$EMAIL" ]]; then
    cat >&2 <<EOF
Erro: DOMAIN e EMAIL são obrigatórios.

Exemplos:
  DOMAIN=devspeak.exemplo.com EMAIL=voce@exemplo.com bash $0
  bash $0 devspeak.exemplo.com voce@exemplo.com
EOF
    exit 1
fi

WEBROOT="/var/www/letsencrypt"
CONF_TARGET="/etc/nginx/conf.d/devspeak.conf"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONF_TEMPLATE="$SCRIPT_DIR/../nginx/devspeak.conf"

echo "==> Configuração:"
echo "    DOMAIN = $DOMAIN"
echo "    EMAIL  = $EMAIL"
echo ""

echo "==> Instalando nginx + certbot..."
if command -v amazon-linux-extras >/dev/null 2>&1; then
    sudo amazon-linux-extras install -y nginx1 >/dev/null
    sudo amazon-linux-extras install -y epel    >/dev/null
    sudo yum install -y certbot                 >/dev/null
else
    # Amazon Linux 2023 / outras distros
    sudo dnf install -y nginx certbot >/dev/null 2>&1 || sudo yum install -y nginx certbot >/dev/null
fi

echo "==> Preparando webroot do ACME..."
sudo mkdir -p "$WEBROOT/.well-known/acme-challenge"

echo "==> Removendo config default do nginx (se existir)..."
sudo rm -f /etc/nginx/conf.d/default.conf

if sudo test -d "/etc/letsencrypt/live/$DOMAIN"; then
    echo "==> Certificado já existe para $DOMAIN — pulando emissão."
    HAS_CERT=1
else
    HAS_CERT=0
fi

if [ "$HAS_CERT" -eq 0 ]; then
    echo "==> Subindo nginx temporário (HTTP-only) para o desafio ACME..."
    sudo tee "$CONF_TARGET" > /dev/null <<EOF
server {
    listen 80;
    server_name $DOMAIN;

    location /.well-known/acme-challenge/ {
        root $WEBROOT;
    }

    location / { return 404; }
}
EOF
    sudo systemctl enable --now nginx
    sudo nginx -t
    sudo systemctl reload nginx

    echo "==> Solicitando certificado ao Let's Encrypt..."
    sudo certbot certonly \
        --webroot -w "$WEBROOT" \
        -d "$DOMAIN" \
        --email "$EMAIL" \
        --agree-tos --non-interactive --no-eff-email
fi

echo "==> Instalando config final (HTTPS + reverse proxy) para $DOMAIN..."
# Substitui o placeholder no template e escreve a config final
sudo sed "s/__DOMAIN__/$DOMAIN/g" "$CONF_TEMPLATE" | sudo tee "$CONF_TARGET" > /dev/null
sudo nginx -t
sudo systemctl reload nginx

echo "==> Agendando renovação automática (diária 03:00)..."
sudo tee /etc/cron.d/certbot-renew > /dev/null <<'EOF'
0 3 * * * root certbot renew --quiet --post-hook "systemctl reload nginx"
EOF

echo ""
echo "=================================="
echo "Setup completo."
echo "Teste: curl -I https://$DOMAIN/health"
echo "=================================="
