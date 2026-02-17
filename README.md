# VPN Client

Кроссплатформенный VPN-клиент с поддержкой split tunneling и несколькими протоколами.

**Платформы:** Windows, Linux, macOS

## Возможности

- **Split Tunneling** — через VPN идёт только трафик к указанным ресурсам
- **Протоколы** — WireGuard, OpenVPN, SSH-over-TUN
- **Kill Switch** — блокировка трафика при разрыве VPN
- **System Tray** — минимальный UI в системном трее
- **Настройки** — GUI на Windows, редактор конфига на Linux/macOS

## Архитектура

```
vpn-client
├── cmd/vpn-client/         # Точка входа
├── internal/
│   ├── config/             # Конфигурация (YAML), пути per-platform
│   ├── core/               # Основная логика VPN-сервиса
│   ├── dns/                # DNS-менеджер (NRPT / resolvectl / scutil)
│   ├── killswitch/         # Kill Switch (WFP / iptables / pf)
│   ├── logger/             # Логирование
│   ├── protocols/          # Реализации VPN-протоколов
│   │   ├── wireguard/
│   │   ├── openvpn/
│   │   └── ssh/
│   ├── routing/            # Split Tunneling (route / ip route / route add)
│   ├── tun/                # TUN-интерфейс (wintun / native)
│   └── ui/                 # System Tray UI + настройки
└── configs/
    └── config.example.yaml
```

Каждый пакет `tun`, `routing`, `dns`, `killswitch` разделён на файлы по платформам через build tags (`_windows.go`, `_linux.go`, `_darwin.go`).

## Сборка

### Требования

- Go 1.21+
- CGO (для systray: GTK на Linux, Cocoa на macOS)
- Права администратора/root для запуска

### Windows

```powershell
# Генерация manifest (один раз, требуется github.com/akavel/rsrc)
cd cmd/vpn-client
go install github.com/akavel/rsrc@latest
rsrc -manifest vpn-client.manifest -o rsrc_windows.syso
cd ../..

# Сборка
go build -ldflags "-H=windowsgui" -o vpn-client.exe ./cmd/vpn-client
```

### Linux

```bash
# Требуется: libgtk-3-dev libappindicator3-dev
sudo apt install libgtk-3-dev libappindicator3-dev
go build -o vpn-client ./cmd/vpn-client
```

### macOS

```bash
go build -o vpn-client ./cmd/vpn-client
```

## Запуск

```bash
# Просто запуск — появится иконка в трее
./vpn-client
```

Без аргументов. Подключение и отключение — через меню в трее.

## Конфигурация

| Платформа | Путь к конфигу |
|-----------|----------------|
| Windows   | `%PROGRAMDATA%\VPNClient\config.yaml` |
| Linux     | `$XDG_CONFIG_HOME/vpn-client/config.yaml` (по умолчанию `~/.config/vpn-client/config.yaml`) |
| macOS     | `~/Library/Application Support/VPNClient/config.yaml` |

См. [configs/config.example.yaml](configs/config.example.yaml) для примера.

### Split Tunneling

Только трафик к указанным IP/доменам маршрутизируется через VPN:

```yaml
routing:
  include_ips:
    - "10.0.0.0/8"
    - "172.16.0.0/12"
  include_domains:
    - "internal.company.com"
```

### Протоколы

**WireGuard** — самый быстрый:

```yaml
protocol: wireguard
wireguard:
  private_key: "..."
  address: "10.255.0.2/24"
  peer:
    public_key: "..."
    endpoint: "vpn.example.com:51820"
```

**OpenVPN** — широко поддерживается:

```yaml
protocol: openvpn
openvpn:
  config_path: "/path/to/client.ovpn"
```

**SSH Tunnel** — VPN через SSH с TUN (требуется `PermitTunnel yes` на сервере):

```yaml
protocol: ssh
ssh:
  host: "ssh.example.com"
  user: "vpnuser"
  key_path: "~/.ssh/id_ed25519"
```

## Зависимости

| Библиотека | Назначение |
|------------|------------|
| `golang.zx2c4.com/wireguard` | WireGuard протокол |
| `golang.org/x/crypto/ssh` | SSH клиент |
| `github.com/getlantern/systray` | System Tray (кроссплатформенный) |
| `github.com/lxn/walk` | GUI настроек (только Windows) |
| `gopkg.in/yaml.v3` | YAML конфигурация |

## Лицензия

MIT
