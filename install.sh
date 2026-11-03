#!/usr/bin/env bash
# TunnelGo one-shot installer (see DEPLOY.md).
# Usage: bash install.sh [panel_port] [master_password_hex]
#   e.g. bash install.sh 80 $(printf 'MyPass' | xxd -p)
set -ex
PORT=${1:-8080}
NEWPW=
if [ -n "${2:-}" ]; then NEWPW=$(printf "$(echo "$2" | sed 's/../\\x&/g')"); fi
export DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a
IP=${IP:-$(ip -4 route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}')}
IFACE=$(ip -br route show default | awk '{print $5; exit}')
apt-get update
apt-get install -y wireguard wireguard-tools iptables curl git ca-certificates ufw jq unzip openssl
printf 'net.ipv4.ip_forward=1\nnet.ipv6.conf.all.forwarding=1\n' > /etc/sysctl.d/99-wireguard.conf
sysctl --system
mkdir -p /etc/wireguard; cd /etc/wireguard; umask 077
[ -f server.key ] || wg genkey > server.key
wg pubkey < server.key > server.pub
cat > wg0.conf <<W
[Interface]
PrivateKey = $(cat server.key)
Address = 10.8.0.1/24
ListenPort = 51820
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -A FORWARD -o wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o $IFACE -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -D FORWARD -o wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o $IFACE -j MASQUERADE
W
umask 022
systemctl enable --now wg-quick@wg0
ufw allow OpenSSH
ufw allow 51820/udp
ufw allow $PORT/tcp
ufw allow 8388:8487/tcp
ufw allow 8443/tcp
ufw --force enable
GV=1.26.4
curl -fsSL https://go.dev/dl/go$GV.linux-amd64.tar.gz -o /tmp/go.tgz || { GV=$(curl -fsSL 'https://go.dev/dl/?mode=json' | jq -r '.[0].version' | sed 's/go//'); curl -fsSL https://go.dev/dl/go$GV.linux-amd64.tar.gz -o /tmp/go.tgz; }
rm -rf /usr/local/go; tar -C /usr/local -xzf /tmp/go.tgz
export PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=auto
echo 'export PATH=/usr/local/go/bin:$PATH' > /etc/profile.d/golang.sh
go version
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
systemctl disable --now xray || true
useradd --system --home /var/lib/tunnelgo --shell /usr/sbin/nologin tunnelgo || true
mkdir -p /var/lib/tunnelgo/data /var/lib/tunnelgo/xray /etc/tunnelgo
chown -R tunnelgo:tunnelgo /var/lib/tunnelgo
chmod 750 /var/lib/tunnelgo /var/lib/tunnelgo/data /var/lib/tunnelgo/xray
chown root:tunnelgo /etc/wireguard /etc/wireguard/wg0.conf
chmod 750 /etc/wireguard
chmod 660 /etc/wireguard/wg0.conf
rm -rf /opt/tunnelgo
git clone https://github.com/AskerovAdil/TunnelGo.git /opt/tunnelgo
cd /opt/tunnelgo
go build -o /usr/local/bin/tunnelgo ./cmd/tunnelgo
chmod 755 /usr/local/bin/tunnelgo
if [ ! -f /etc/tunnelgo/env ]; then
PW=$(openssl rand -base64 18 | tr -d '/+=')
SS=$(openssl rand -hex 32)
cat > /etc/tunnelgo/env <<E
TUNNELGO_LISTEN=:$PORT
TUNNELGO_LOG_LEVEL=info
TUNNELGO_SHUTDOWN_TIMEOUT=10s
TUNNELGO_MASTER_PASSWORD=$PW
TUNNELGO_SESSION_SECRET=$SS
TUNNELGO_SESSION_TTL=24h
TUNNELGO_SECURE_COOKIES=false
TUNNELGO_DB_PATH=/var/lib/tunnelgo/data/tunnelgo.db
TUNNELGO_WG_INTERFACE=wg0
TUNNELGO_WG_CONFIG=/etc/wireguard/wg0.conf
TUNNELGO_WG_ENDPOINT=$IP:51820
TUNNELGO_WG_DNS=1.1.1.1
TUNNELGO_WG_ALLOWED_IPS=0.0.0.0/0,::/0
TUNNELGO_WG_KEEPALIVE=25
TUNNELGO_SS_ENABLED=true
TUNNELGO_SS_HOST=$IP
TUNNELGO_SS_METHOD=AEAD_AES_256_GCM
TUNNELGO_SS_PORT_MIN=8388
TUNNELGO_SS_PORT_MAX=8487
TUNNELGO_VLESS_ENABLED=true
TUNNELGO_VLESS_HOST=$IP
TUNNELGO_VLESS_PORT=8443
TUNNELGO_VLESS_BINARY=/usr/local/bin/xray
TUNNELGO_VLESS_CONFIG=/var/lib/tunnelgo/xray/config.json
TUNNELGO_VLESS_KEYS=/var/lib/tunnelgo/xray/reality.json
TUNNELGO_VLESS_DEST=www.cloudflare.com:443
TUNNELGO_VLESS_SERVER_NAMES=www.cloudflare.com
E
fi
sed -i "s|^TUNNELGO_LISTEN=.*|TUNNELGO_LISTEN=:$PORT|" /etc/tunnelgo/env
if [ -n "$NEWPW" ]; then set +x; sed -i "s|^TUNNELGO_MASTER_PASSWORD=.*|TUNNELGO_MASTER_PASSWORD=$NEWPW|" /etc/tunnelgo/env; set -x; fi
chown root:tunnelgo /etc/tunnelgo/env
chmod 640 /etc/tunnelgo/env
cat > /etc/systemd/system/tunnelgo.service <<'U'
[Unit]
Description=TunnelGo VPN manager
After=network-online.target wg-quick@wg0.service
Wants=network-online.target
[Service]
Type=simple
User=tunnelgo
Group=tunnelgo
EnvironmentFile=/etc/tunnelgo/env
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart=/usr/local/bin/tunnelgo
Restart=on-failure
RestartSec=3
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
NoNewPrivileges=true
StandardOutput=journal
StandardError=journal
[Install]
WantedBy=multi-user.target
U
systemctl daemon-reload
systemctl enable --now tunnelgo
systemctl restart tunnelgo
sleep 4
set +x
echo; echo "===== HEALTHZ ====="; curl -sS http://127.0.0.1:$PORT/healthz; echo
wg show wg0 | head -3
ss -tlnp | grep -E ":$PORT |8443|8388" || true
echo "===== DONE ====="
echo "Panel: http://$IP:$PORT/login"
grep MASTER_PASSWORD /etc/tunnelgo/env
