<p align="center">
  <img src="docs/assets/logo.png" alt="Dockafé logo" width="180">
</p>

<h1 align="center">Dockafé</h1>

<p align="center">
  <strong>Docker + café — brewed for the terminal</strong><br>
  Interactive TUI for Compose, containers, images, volumes &amp; networks
</p>

<p align="center">
  <a href="https://github.com/coffee/docker-tui/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/coffee/docker-tui/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT">
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macOS-lightgrey?style=flat-square" alt="Platform">
</p>

<p align="center">
  <a href="#-screenshots">Screenshots</a> ·
  <a href="#-features">Features</a> ·
  <a href="#-install">Install</a> ·
  <a href="#-keybindings">Keys</a> ·
  <a href="#-configuration">Config</a>
</p>

---

## 📸 Screenshots

<details open>
<summary><strong>Splash</strong> — whale + coffee, then you’re in</summary>
<br>
<p align="center">
  <img src="docs/assets/screenshot-splash.png" alt="Dockafé splash screen" width="900">
</p>
</details>

<details open>
<summary><strong>Compose</strong> — projects, CPU/MEM, ports at a glance</summary>
<br>
<p align="center">
  <img src="docs/assets/screenshot-compose.png" alt="Dockafé Compose tab" width="900">
</p>
</details>

<details open>
<summary><strong>Volumes</strong> — in-use status and mountpoints</summary>
<br>
<p align="center">
  <img src="docs/assets/screenshot-volumes.png" alt="Dockafé Volumes tab" width="900">
</p>
</details>

<details open>
<summary><strong>Volume files</strong> — browse, preview, edit with LSP</summary>
<br>
<p align="center">
  <img src="docs/assets/screenshot-volume-files.png" alt="Dockafé volume file browser" width="900">
</p>
</details>

> Screenshots captured from a live session on Fedora + Docker 29.

---

## ✨ Features

| | |
|---|---|
| **Five tabs** | Compose · Containers · Images · Volumes · Networks |
| **Lifecycle** | Start / stop / restart / rebuild / remove / prune |
| **Logs** | Follow mode, text + regex search, colored output |
| **Volume browser** | File tree, syntax highlight, lint, Tab focus mode |
| **Editors** | Opens Helix / Neovim / VS Code (`--wait`) with LSP |
| **Filters** | `/` text filter, `o`/`O` sort, `F` running / in-use only |
| **Safe writes** | Confirm before saving volume files; remote Docker never touches local host paths |

---

## 🚀 Install

### Requirements

- Go **1.22+** (to build)
- Running **Docker daemon** (`DOCKER_HOST` / `docker.sock`)
- `docker` **CLI** (compose up / rebuild)

> [!WARNING]
> Access to the Docker socket is effectively **root** on the host. Only connect to daemons you trust.

### Build & run

```bash
git clone https://github.com/coffee/docker-tui.git
cd docker-tui
make run          # build + start
```

```bash
make build        # → ./dockafe
make install      # → $(go env GOPATH)/bin/dockafe
make test vet fmt
```

```bash
go install github.com/coffee/docker-tui@latest
```

**Fedora:** `make build` / `make install`. A Copr package may follow later.

---

## ⌨️ Keybindings

Press <kbd>?</kbd> inside the app for the full reference.

| Area | Keys |
|------|------|
| Tabs | <kbd>1</kbd>–<kbd>5</kbd> · <kbd>[</kbd> <kbd>]</kbd> |
| Filter / sort | <kbd>/</kbd> · <kbd>o</kbd>/<kbd>O</kbd> · <kbd>F</kbd> running / in-use |
| Lifecycle | <kbd>s</kbd> start · <kbd>x</kbd> stop · <kbd>R</kbd> restart · <kbd>b</kbd> rebuild · <kbd>d</kbd>/<kbd>D</kbd> remove |
| Logs | <kbd>l</kbd> · <kbd>/</kbd> find · <kbd>Ctrl</kbd>+<kbd>G</kbd> regex |
| Volume files | <kbd>f</kbd> tree · <kbd>Tab</kbd> focus · <kbd>e</kbd> edit · <kbd>L</kbd> lint · <kbd>o</kbd> open dir |
| Quit | <kbd>q</kbd> |

---

## ⚙️ Configuration

| Variable | Purpose |
|----------|---------|
| `DOCKER_HOST` | Docker daemon address |
| `DOCKAFE_EDITOR` / `EDITOR` / `VISUAL` | External editor for volume files. `code` / `codium` get `--wait` automatically |
| `DOCKAFE_BUSYBOX_IMAGE` | Helper image when host mount is unavailable (default `busybox:1.36.1`) |

### Volume file access

- **Local** readable mountpoint → `via host`
- **Remote** daemon or locked mount → `busybox` helper → `via docker`
- Saving from the editor always asks for confirmation

---

## 🛠️ Development

```bash
make test   # unit tests
make vet
make build
```

Optional Docker smoke test:

```bash
DOCKER_INTEGRATION=1 go test ./internal/docker -run Integration
```

CI runs `gofmt`, `vet`, `test`, and `build` on every push / PR — see [`.github/workflows/ci.yml`](.github/workflows/ci.yml).

---

## 📄 License

[MIT](LICENSE) © Dockafé contributors
