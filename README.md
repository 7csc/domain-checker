# Doch - Domain Checker

`doch` is a command-line tool to check the status, IP address, cloud service, and port availability of specified domains.  
The target domains and ports can be configured in a **TOML file**, allowing for flexible customization.

## Features

- Check if a domain is reachable (HTTP/HTTPS connectivity)  
- Resolve the domain's IPv4 address (CNAME resolution supported)  
- Detect the cloud provider (AWS, GCP, Azure) based on IP ranges  
- Scan for open ports based on the configured list in the TOML file  
- Display results in a clear table format  
- Detailed logs available with `--verbose`  
- Custom TOML file path supported via `--file` option

---

## Installation

```sh
git clone https://github.com/7csc/domain-checker.git
cd domain-checker
go build -o doch
```

## Usage

```sh
./doch check --file=domains.toml --verbose
```

## Options

|option|description|
|----|----|
|--file|Path to the TOML file listing domains and ports (default: domains.toml)|
|--verbose|Show detailed logs|

## Example domains.toml

```toml
[[domains]]
name = "example.com"
ports = { http = 80, https = 443, ssh = 22 }

[[domains]]
name = "www.sample.net"
ports = { http = 80, https = 443, ssh = 22  }
```

## Sample Output

```text
fetching example.com /
fetching www.sample.net /
+-------------------+----------+---------+---------+----------------+------+------+-------+-----+
|      DOMAIN       |  STATUS  |  CLOUD  | SERVICE |       IP       | SMTP | HTTP | HTTPS | SSH |
+-------------------+----------+---------+---------+----------------+------+------+-------+-----+
| example.com       | active   | AWS     | shared  | xx.xxx.xx.xxx  | -    | open | open  | -   |
| www.sample.net    | active   | GCP     | unknown | xx.xxx.xx.xxx  | open | open | open  | -   |
| service.io        | active   | Azure   | unknown | xx.xxx.xx.xxx  | open | open | open  | -   |
| unknown.space     | deactive | unknown | unknown | N/A            | open | -    | -     | -   |
+-------------------+----------+---------+---------+----------------+------+------+-------+-----+
```
