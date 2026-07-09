# TunnelGo — установка на Linux-сервер

Пошаговая инструкция: WireGuard + веб-панель TunnelGo + Shadowsocks + VLESS Reality (xray) для Hiddify.

Замени везде:

| Плейсхолдер | Пример |
|-------------|--------|
| `YOUR_PUBLIC_IP` | `141.98.84.252` |
| `YOUR_IFACE` | `eth0` / `ens3` (см. `ip -br a`) |
| `REPO_URL` | `https://github.com/AskerovAdil/TunnelGo.git` |

ОС: Ubuntu 22.04/24.04 или Debian 12. Нужен root/sudo и публичный IP.

---

## Что получится

| Сервис | Порт | Назначение |
|--------|------|------------|
| TunnelGo UI | `8080/tcp` | веб-панель |
| WireGuard | `51820/udp` | официальный клиент WireGuard (`.conf`) |
| Shadowsocks | `8388–8487/tcp` | по одному порту на клиента |
| VLESS Reality | `8443/tcp` | Hiddify (системный прокси, без admin/TUN) |

Для Hiddify на рабочем ПК **без прав администратора** используй кнопку **Hiddify** → ссылка `vless://...`.

---

## Шаг 1. Обновить систему и пакеты

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y \
  wireguard wireguard-tools \
  curl git ca-certificates \
  ufw jq unzip
```

Проверка:

```bash
wg --version
which wg wg-quick
```

Узнать сетевой интерфейс (для PostUp/PostDown):

```bash
ip -br a
# пример: eth0 или ens3 → это YOUR_IFACE
```

---

## Шаг 2. IP forwarding

```bash
echo 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-wireguard.conf
echo 'net.ipv6.conf.all.forwarding=1' | sudo tee -a /etc/sysctl.d/99-wireguard.conf
sudo sysctl --system
```

---

## Шаг 3. Ключи и интерфейс WireGuard

```bash
sudo mkdir -p /etc/wireguard
cd /etc/wireguard

umask 077
wg genkey | sudo tee server.key | wg pubkey | sudo tee server.pub
sudo chmod 600 server.key
```

Создай конфиг (подставь `YOUR_IFACE` и ключ из `server.key`):

```bash
SERVER_PRIV=$(sudo cat /etc/wireguard/server.key)
IFACE=$(ip -br route show default | awk '{print $5; exit}')

sudo tee /etc/wireguard/wg0.conf >/dev/null <<EOF
[Interface]
PrivateKey = ${SERVER_PRIV}
Address = 10.8.0.1/24
ListenPort = 51820
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -A FORWARD -o wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o ${IFACE} -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -D FORWARD -o wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o ${IFACE} -j MASQUERADE
EOF

sudo chmod 600 /etc/wireguard/wg0.conf
```

Поднять и включить автозапуск:

```bash
sudo systemctl enable --now wg-quick@wg0
sudo wg show wg0
```

Ожидание: `listening port: 51820`, публичный ключ сервера.

---

## Шаг 4. Firewall (UFW)

```bash
sudo ufw allow OpenSSH
sudo ufw allow 51820/udp comment 'WireGuard'
sudo ufw allow 8080/tcp comment 'TunnelGo UI'
sudo ufw allow 8388:8487/tcp comment 'Shadowsocks'
sudo ufw allow 8443/tcp comment 'VLESS Reality'
sudo ufw enable
sudo ufw status
```

---

## Шаг 5. Установить Go

Нужен Go **1.26+** (см. `go.mod`).

```bash
GO_VER=1.26.4
curl -fsSL "https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz" -o /tmp/go.tgz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=/usr/local/go/bin:$PATH' | sudo tee /etc/profile.d/golang.sh
source /etc/profile.d/golang.sh
go version
```

---

## Шаг 6. Установить Xray (для VLESS Reality)

```bash
bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install
```

Либо вручную в `/usr/local/bin/xray`:

```bash
# проверка после установки
xray version
# или
/usr/local/bin/xray version
```

Если бинарник не в `/usr/local/bin/xray`, найди путь:

```bash
which xray
```

и укажи его в `TUNNELGO_VLESS_BINARY`.

> Порт **443** обычно требует `setcap`. По умолчанию в инструкции — **8443** (работает без cap).

Опционально, если хочешь слушать 443:

```bash
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/xray
# затем в env: TUNNELGO_VLESS_PORT=443 и открой 443/tcp в UFW
```

---

## Шаг 7. Пользователь и каталоги

```bash
sudo useradd --system --home /var/lib/tunnelgo --shell /usr/sbin/nologin tunnelgo || true

sudo mkdir -p /var/lib/tunnelgo/data /var/lib/tunnelgo/xray /etc/tunnelgo /opt/tunnelgo
sudo chown -R tunnelgo:tunnelgo /var/lib/tunnelgo
sudo chmod 750 /var/lib/tunnelgo /var/lib/tunnelgo/data /var/lib/tunnelgo/xray

# TunnelGo должен читать/писать wg0.conf и управлять peers
sudo chown root:tunnelgo /etc/wireguard
sudo chmod 750 /etc/wireguard
sudo chown root:tunnelgo /etc/wireguard/wg0.conf
sudo chmod 660 /etc/wireguard/wg0.conf
```

Проверка доступа:

```bash
sudo -u tunnelgo -g tunnelgo cat /etc/wireguard/wg0.conf | head -5
```

---

## Шаг 8. Клонировать проект (git clone)

```bash
cd /opt
sudo git clone https://github.com/AskerovAdil/TunnelGo.git tunnelgo
sudo chown -R root:root /opt/tunnelgo
```

Если репозиторий приватный:

```bash
cd /opt
sudo git clone git@github.com:AskerovAdil/TunnelGo.git tunnelgo
```

---

## Шаг 9. Собрать бинарник

```bash
cd /opt/tunnelgo
sudo -E env PATH="/usr/local/go/bin:$PATH" go build -o /tmp/tunnelgo ./cmd/tunnelgo
sudo mv /tmp/tunnelgo /usr/local/bin/tunnelgo
sudo chmod 755 /usr/local/bin/tunnelgo
/usr/local/bin/tunnelgo -version || true
```

---

## Шаг 10. Файл окружения `/etc/tunnelgo/env`

Образец всех переменных — в репозитории: [`.env.example`](.env.example).

```bash
sudo cp /opt/tunnelgo/.env.example /etc/tunnelgo/env
sudo nano /etc/tunnelgo/env
```

Минимальный рабочий пример (подставь IP и секреты):

```bash
TUNNELGO_LISTEN=:8080
TUNNELGO_LOG_LEVEL=info
TUNNELGO_MASTER_PASSWORD=задай-сложный-пароль
TUNNELGO_SESSION_SECRET=минимум-32-случайных-символа-сюда
TUNNELGO_SECURE_COOKIES=false

TUNNELGO_DB_PATH=/var/lib/tunnelgo/data/tunnelgo.db

TUNNELGO_WG_INTERFACE=wg0
TUNNELGO_WG_CONFIG=/etc/wireguard/wg0.conf
TUNNELGO_WG_ENDPOINT=YOUR_PUBLIC_IP:51820
TUNNELGO_WG_DNS=1.1.1.1
TUNNELGO_WG_ALLOWED_IPS=0.0.0.0/0,::/0
TUNNELGO_WG_KEEPALIVE=25

TUNNELGO_SS_ENABLED=true
TUNNELGO_SS_HOST=YOUR_PUBLIC_IP
TUNNELGO_SS_METHOD=AEAD_AES_256_GCM
TUNNELGO_SS_PORT_MIN=8388
TUNNELGO_SS_PORT_MAX=8487

TUNNELGO_VLESS_ENABLED=true
TUNNELGO_VLESS_HOST=YOUR_PUBLIC_IP
TUNNELGO_VLESS_PORT=8443
TUNNELGO_VLESS_BINARY=/usr/local/bin/xray
TUNNELGO_VLESS_CONFIG=/var/lib/tunnelgo/xray/config.json
TUNNELGO_VLESS_KEYS=/var/lib/tunnelgo/xray/reality.json
TUNNELGO_VLESS_DEST=www.cloudflare.com:443
TUNNELGO_VLESS_SERVER_NAMES=www.cloudflare.com
```

Сгенерировать `SESSION_SECRET`:

```bash
openssl rand -hex 32
```

Права на env:

```bash
sudo chown root:tunnelgo /etc/tunnelgo/env
sudo chmod 640 /etc/tunnelgo/env
```

**Важно:** строки `TUNNELGO_VLESS_BINARY`, `TUNNELGO_VLESS_CONFIG`, `TUNNELGO_VLESS_KEYS` **не удаляй** — они нужны.

---

## Шаг 11. systemd unit

```bash
sudo tee /etc/systemd/system/tunnelgo.service >/dev/null <<'EOF'
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

# Нужно для управления WireGuard peers через wgctrl
AmbientCapabilities=CAP_NET_ADMIN
CapabilityBoundingSet=CAP_NET_ADMIN
NoNewPrivileges=true

# xray пишет логи в stdout сервиса
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now tunnelgo
sudo systemctl status tunnelgo --no-pager
```

Логи:

```bash
sudo journalctl -u tunnelgo -n 80 --no-pager
```

Ожидай строки вроде:

- сервис слушает `:8080`
- при создании клиента: `shadowsocks listening`, `xray started` на порту `8443`

Если Reality-ключи нужно пересоздать с нуля:

```bash
sudo systemctl stop tunnelgo
sudo rm -f /var/lib/tunnelgo/xray/reality.json
sudo systemctl start tunnelgo
```

После смены ключей **старые `vless://` ссылки недействительны** — создай клиента заново.

---

## Шаг 12. Проверка

```bash
curl -sS http://127.0.0.1:8080/healthz
sudo wg show wg0
sudo ss -tlnp | grep -E '8080|8443|8388'
```

В браузере:

```text
http://YOUR_PUBLIC_IP:8080/login
```

1. Войди с `TUNNELGO_MASTER_PASSWORD`
2. Создай клиента (например `phone`)
3. **Download** — `.conf` для официального WireGuard
4. **Hiddify** — `vless://...` для Hiddify

---

## Шаг 13. Клиенты

### WireGuard (телефон / домашний ПК)

1. Скачай `.conf` из панели
2. Импортируй в [официальный WireGuard](https://www.wireguard.com/install/)
3. Connect → в панели статус Online

### Hiddify (рабочий ПК без admin)

1. Скачай / скопируй ссылку **Hiddify** (`vless://...`)
2. Hiddify → **+** → **Add from clipboard**
3. Подключись (режим системного прокси, без TUN)
4. Проверка с ПК:

```powershell
curl.exe -v -x http://127.0.0.1:12334 http://ifconfig.me --max-time 15
```

Должен вернуться `YOUR_PUBLIC_IP`, не `502`.

> Классический `ss://` из панели для Hiddify может не подойти; для прокси-режима ориентируйся на **VLESS Reality**.

---

## Обновление с GitHub

```bash
cd /opt/tunnelgo
sudo git pull
sudo -E env PATH="/usr/local/go/bin:$PATH" go build -o /tmp/tunnelgo ./cmd/tunnelgo
sudo systemctl stop tunnelgo
sudo mv /tmp/tunnelgo /usr/local/bin/tunnelgo
sudo chmod 755 /usr/local/bin/tunnelgo
sudo systemctl start tunnelgo
sudo journalctl -u tunnelgo -n 40 --no-pager
```

Если менялся `.env.example` — сверь с `/etc/tunnelgo/env` и допиши новые переменные.

---

## Частые проблемы

### UI не открывается

```bash
sudo systemctl status tunnelgo --no-pager
sudo journalctl -u tunnelgo -n 50 --no-pager
sudo ufw status | grep 8080
```

### Создание клиента падает / WireGuard sync

```bash
sudo -u tunnelgo -g tunnelgo cat /etc/wireguard/wg0.conf | head -5
ls -la /etc/wireguard/wg0.conf
# должно быть root:tunnelgo, режим 660
# в unit обязательны AmbientCapabilities=CAP_NET_ADMIN
```

### VLESS: Hiddify даёт 502

1. Порт 8443 доступен с ПК: `Test-NetConnection YOUR_PUBLIC_IP -Port 8443`
2. Ссылка свежая после смены `reality.json`
3. В ссылке `pbk`/`sid` совпадают с ключами:

```bash
sudo cat /var/lib/tunnelgo/xray/reality.json
sudo jq '.inbounds[0].streamSettings.realitySettings | {serverNames,shortIds,dest}' \
  /var/lib/tunnelgo/xray/config.json
```

4. Пока Hiddify подключён:

```bash
sudo ss -tnp | grep 8443
sudo journalctl -u tunnelgo -f
```

Нужен `ESTAB` с IP твоего ПК.

### Порт 443 Permission denied

Оставь `TUNNELGO_VLESS_PORT=8443` или выдай xray `cap_net_bind_service` (шаг 6).

---

## Краткий чеклист

- [ ] `wg0` поднят, forwarding включён
- [ ] UFW: 51820/udp, 8080/tcp, 8388–8487/tcp, 8443/tcp
- [ ] Go + xray установлены
- [ ] `git clone` в `/opt/tunnelgo`, бинарник в `/usr/local/bin/tunnelgo`
- [ ] `/etc/tunnelgo/env` заполнен (пароль, secret, IP, VLESS_*)
- [ ] systemd с `CAP_NET_ADMIN` и `EnvironmentFile`
- [ ] UI логин → клиент → WireGuard `.conf` и/или Hiddify `vless://`
