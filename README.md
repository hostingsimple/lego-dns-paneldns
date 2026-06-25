# lego DNS provider for PanelDNS

[![Latest Release](https://img.shields.io/github/v/release/hostingsimple/lego-dns-paneldns)](https://github.com/hostingsimple/lego-dns-paneldns/releases/latest) [![Release](https://github.com/hostingsimple/lego-dns-paneldns/actions/workflows/release.yml/badge.svg)](https://github.com/hostingsimple/lego-dns-paneldns/actions/workflows/release.yml) [![License](https://img.shields.io/github/license/hostingsimple/lego-dns-paneldns)](LICENSE) [![CI](https://github.com/hostingsimple/lego-dns-paneldns/actions/workflows/ci.yml/badge.svg)](https://github.com/hostingsimple/lego-dns-paneldns/actions/workflows/ci.yml) ![lego](https://img.shields.io/badge/lego-DNS--01-green)

Go library and exec binary that lets [lego](https://github.com/go-acme/lego) — and any tool built on it (Traefik, Nginx Proxy Manager, Coolify, Caprover) — complete ACME DNS-01 challenges using [PanelDNS](https://paneldns.com).

## Traefik

``Fyaml
# traefik.yml
certificatesResolvers:
  paneldns:
    acme:
      email: admin@example.com
      storage: /data/acme.json
      dnsChallenge:
        provider: exec
        delayBeforeCheck: 10

# Environment variables (docker-compose or .env)
# EXEC_PATH=/usr/local/bin/lego-dns-paneldns
# PANELDNS_URL=https://app.paneldns.com
# PANELDNS_TOKEN=dnsm_xxxx
```

```yaml
# docker-compose.yml labels
labels:
  - "traefik.http.routers.app.tls.certresolver=paneldns"
  - "traefik.http.routers.app.tls.domains[0].main=*.example.com"
  - "traefik.http.routers.app.tls.domains[0].sans=example.com"
```

## Download the exec binary

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/Veeau/lego-dns-paneldns/releases) page.

```sh
# Linux amd64 example
curl -L https://github.com/Veeau/lego-dns-paneldns/releases/latest/download/lego-dns-paneldns_linux_amd64.tar.gz \
  | tar xz -C /usr/local/bin/lego-dns-paneldns
chmod +x /usr/local/bin/lego-dns-paneldns
```

## Use as a Go library

```go
import "github.com/Veeau/lego-dns-paneldns/paneldns"

cfg := paneldns.NewDefaultConfig()
cfg.APIToken = "dnsm_xxxx"

provider, err := paneldns.NewDNSProviderConfig(cfg)
// provider implements challenge.Provider
```

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `PANELDNS_URL` | No | `https://app.paneldns.com` | PanelDNS instance URL |
| `PANELDCS_TOKEN` | **Yes** | — | API Bearer token |
| `PANELDNS_TTL` | No | `0` | TXT record TTL in seconds |
| `PANELDNS_PROPAGATION_TIMEOUT` | No | `120s` | How long to wait for DNS propagation |
| `PANELDNS_POLLING_INTERVAL` | No | `5s` | Polling interval for propagation checks |

## License

MIT
