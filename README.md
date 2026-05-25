# Web Archive Scanner

Modern Go tool to extract archived URLs from the Wayback Machine (web.archive.org).

## Features

- Fast concurrent scanning with configurable worker pool
- Single URL or mass URL list scanning
- Suffix filtering with regex support
- Proxy support
- Buffered file output for better performance
- Clean structured code with `context.Context`

## Installation

```bash
go build -o go-web-archive main.go
```

## Usage

### Single URL

```bash
go run main.go -url example.com -output results.txt -subdomain
```

### Mass URLs

```bash
go run main.go -file urls.txt -output results.txt -subdomain
```

## Options

| Flag | Default | Description |
|------|---------|-------------|
| `-url` | `""` | Single URL to scan |
| `-file` | `""` | File containing URLs to scan |
| `-proxy` | `""` | Proxy URL (`http://ip:port`) |
| `-suffix` | `""` | Suffix filter, comma separated (e.g. `js,css,png`) |
| `-output` | `subdomain.txt` | Output file |
| `-subdomain` | `false` | Include subdomains |
| `-workers` | `20` | Number of concurrent workers |
| `-timeout` | `10s` | HTTP timeout per request |

## Examples

Scan with suffix filter and 50 workers:

```bash
go run main.go -file targets.txt -suffix js,css,json -workers 50 -output archives.txt
```

Scan single domain through proxy:

```bash
go run main.go -url example.com -proxy http://127.0.0.1:8080 -subdomain
```

## Disclaimer

This tool is intended for authorized security research and bug bounty purposes only.
