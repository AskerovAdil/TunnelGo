# TunnelGo

Легковесный менеджер VPN на Go: WireGuard + веб-панель, Shadowsocks и VLESS Reality (xray) для Hiddify.

## Документация

Полная пошаговая установка на Linux-сервер: **[DEPLOY.md](DEPLOY.md)**

Пример переменных окружения: **[.env.example](.env.example)**

## Быстрый старт (после установки)

1. Открой `http://YOUR_PUBLIC_IP:8080/login`
2. Создай клиента
3. **Download** — `.conf` для официального WireGuard
4. **Hiddify** — `vless://` для системного прокси без admin/TUN
