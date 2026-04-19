# Statis

A lightweight, self-hosted dashboard for your homelab services. Built with Go and Skeleton CSS for minimal resource usage.

![Dashboard Screenshot](docs/screenshot.png)

## Features

- 🪶 **Lightweight** - Single binary, ~10MB memory usage
- ⚙️ **Configurable** - YAML config file or web UI
- 🎨 **Themeable** - Customize colors, fonts, and favicon via config
- 📱 **Responsive** - Works on mobile and desktop
- 🔌 **Widgets** - Uptime Kuma, RSS, System Stats, Clock, iFrame, Header, TrueNAS SCALE
- 🖼️ **Dashboard Icons** - Browse and search 2800+ icons from homarr-labs/dashboard-icons
- 🐳 **Docker Ready** - Easy deployment

## Quick Start

### Option 1: Docker Compose (Recommended)

```bash
# Clone or download
git clone https://github.com/hyhenry/statis.git
cd statis

# Edit config
cp config.yaml.template config.yaml
nano config.yaml

# Run
docker-compose up -d
```

### Option 2: Go Binary

```bash
# Build
go build -o statis .

# Run
./statis
```

### Option 3: Build from Source

```bash
# Install dependencies
go mod download

# Run in development
go run .
```

Access the dashboard at `http://localhost:8080`

## Configuration

Edit `config.yaml` or use the web UI at `/settings`.

### Basic Structure

```yaml
title: "My Homelab"
subtitle: "Welcome home"

theme:
  primary_color: "#33C3F0"
  secondary_color: "#33C3F0"
  background_color: "#1a1a2e"
  card_color: "#16213e"
  text_color: "#eaeaea"
  font_family: "system"              # Or any Google Font name
  favicon: ""                        # Auto-populated if favicon_name is set
  favicon_name: "home-assistant"     # Dashboard icon name - auto-downloads

layout:
  widget_columns: 4                  # Width of widget column (2-6)
  service_columns: 8                 # Width of service column (12 - widget_columns)
  cards_per_row: 3                   # Service cards per row (1-5)

services:
  - name: "Infrastructure"
    items:
      - name: "Proxmox"
        url: "https://proxmox.local:8006"
        icon: ""                     # Auto-populated if icon_name is set
        icon_name: "proxmox"         # Dashboard icon name - auto-downloads
        icon_text: "🖥️"             # Emoji fallback
        description: "Hypervisor"
        target: "_blank"             # Optional: _blank, _self

widgets:
  - type: "uptime-kuma"
    title: "Service Status"
    config:
      url: "https://uptime.local:3001"
      slug: "status"
      collapsed: "true"
```

### Icons

Statis integrates with [homarr-labs/dashboard-icons](https://github.com/homarr-labs/dashboard-icons), providing access to 2800+ service icons.

**Option 1: Icon Name (Recommended)**
Use `icon_name` to reference icons by name. They are automatically downloaded and cached locally:
```yaml
- name: "Proxmox"
  icon_name: "proxmox"       # Downloads /icons/proxmox.svg automatically
```

**Option 2: Browse Icons**
Use the icon picker in the settings UI to search and select icons visually.

**Option 3: Upload Custom Icons**
Drag and drop your own icons (SVG, PNG, JPG, GIF, WebP) via the settings UI.

**Option 4: Emoji Fallback**
```yaml
- name: "Custom Service"
  icon_text: "🖥️"           # Emoji shown if no icon is set
```

### Favicon

Set a custom favicon for the browser tab:

```yaml
theme:
  favicon_name: "home-assistant"   # Uses dashboard icon - auto-downloads
  # OR
  favicon: "/icons/custom.svg"     # Use a custom uploaded icon
```

You can also set the favicon via the Settings UI in the General section.

### Available Widgets

#### Header
Visual separator between widget groups:
```yaml
- type: "header"
  title: "Status"
  config: {}
```

#### Uptime Kuma
```yaml
- type: "uptime-kuma"
  title: "Service Status"
  config:
    url: "https://your-uptime-kuma-instance"
    slug: "your-status-page-slug"
    collapsed: "true"              # Optional: start collapsed
```

#### RSS Feed
```yaml
- type: "rss"
  title: "Tech News"
  config:
    url: "https://feeds.example.com/rss"
    items_per_page: "3"            # Items shown per page
    refresh: "300"                 # Refresh interval in seconds
```

#### System Stats (Linux only)
```yaml
- type: "system-stats"
  title: "System Usage"
  config: {}
```

#### Clock
```yaml
- type: "clock"
  title: "Local Time"
  config:
    timezone: "local"              # Or "America/New_York", "Europe/London", etc.
    format: "24h"                  # Or "12h"
```

#### iFrame
```yaml
- type: "iframe"
  title: "Embedded Page"
  config:
    url: "https://example.com"
    height: "300px"
```

#### TrueNAS SCALE
Monitors a TrueNAS SCALE instance via its v2.0 REST API. Each section can
be toggled independently, and per-section failures degrade gracefully so
the rest of the widget still renders.
```yaml
- type: "truenas-scale"
  title: "TrueNAS"
  config:
    url: "https://truenas.local"
    api_key: "1-xxxxxxxxxxxxxxxx"    # From TrueNAS UI → Settings → API Keys
    show_system: "true"              # Hostname, uptime, CPU, memory
    show_pools: "true"               # Pool status + capacity bars (ZFS/scan errors surfaced)
    show_disks: "true"               # Per-disk health + temperature
    show_backups: "true"             # Cloud sync + rsync task status
```
Self-signed TLS certificates are accepted. The API key is proxied through
the server — suitable for trusted networks only.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `CONFIG_PATH` | `./config.yaml` | Path to config file |

## Reverse Proxy

### Nginx
```nginx
location / {
    proxy_pass http://localhost:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Traefik (Docker labels)
```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.statis.rule=Host(`dash.yourdomain.com`)"
```

## Development

### Project Structure
```
statis/
├── main.go              # Entry point, route registration, embedded assets
├── internal/
│   ├── config.go        # Config structs, loading, saving, file watching
│   ├── handlers.go      # HTTP handlers for index, settings, API config
│   ├── widgets.go       # Widget proxies (Uptime Kuma, RSS, system stats)
│   ├── icons.go         # Icon search, download, upload
│   ├── assets.go        # Font management, asset cleanup
│   ├── response.go      # JSON response helpers
│   └── util.go          # Directory creation, file download utilities
├── tests/               # Test files
├── config.yaml          # Configuration
├── icons/               # Downloaded/uploaded icons
├── fonts/               # Downloaded Google Fonts
├── templates/
│   ├── index.html       # Dashboard page
│   └── settings.html    # Settings page
├── static/
│   ├── css/
│   │   ├── custom.css   # Imports all CSS modules
│   │   ├── base.css     # CSS variables, typography
│   │   ├── layout.css   # Header, footer, responsive
│   │   ├── components.css # Service cards, widgets
│   │   ├── settings.css # Settings page styles
│   │   ├── modals.css   # Modals, icon picker
│   │   ├── skeleton.css # Grid framework
│   │   └── normalize.css
│   └── js/
│       ├── widgets.js   # Widget initialization and rendering
│       ├── settings.js  # Settings UI, form handling
│       ├── icon-picker.js # Icon picker module
│       ├── yaml-editor.js # YAML editor modal
│       └── utils.js     # Shared utilities
├── Dockerfile
└── docker-compose.yaml
```

### Adding Widgets

1. Add handler function in `internal/widgets.go` (for widgets that need a proxy)
2. Add initialization in `static/js/widgets.js`
3. Add to widget type dropdown in `static/js/settings.js`

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Dashboard |
| `/settings` | GET | Settings page |
| `/api/config` | GET | Get current config |
| `/api/config` | PUT | Update config |
| `/api/widget/uptime-kuma` | GET | Proxy to Uptime Kuma |
| `/api/widget/system-stats` | GET | Local CPU/RAM usage (Linux only) |
| `/api/widget/rss` | GET | Fetch and parse RSS/Atom feeds |
| `/api/widget/truenas-scale` | GET | Proxy to TrueNAS SCALE v2.0 REST API |
| `/api/icons/search` | GET | Search dashboard icons |
| `/api/icons/download` | POST | Download icon from CDN |
| `/api/icons/upload` | POST | Upload custom icon |
| `/api/assets/clean-unused` | DELETE | Remove unused fonts and icons |

## Comparison

| Feature | Statis | Dashy | Homer |
|---------|--------|-------|-------|
| Binary Size | ~8MB | N/A | N/A |
| Memory Usage | ~10MB | ~100MB+ | ~50MB+ |
| Config | YAML + UI | YAML + UI | YAML |
| Widgets | ✓ | ✓✓✓ | Limited |
| Themes | ✓ | ✓✓✓ | ✓ |
| Dependencies | None | Node.js | Node.js |

## License

MIT License - feel free to use, modify, and distribute.

## Contributing

Pull requests welcome! Please keep the lightweight philosophy in mind.
