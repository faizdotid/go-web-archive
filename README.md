# Web Archive Scanner

## Single URL Scan
To scan a single URL, run the following command:
```bash
go run main.go -url http://example.com -output output.txt -subdomain
```
## Mass URLs Scan
To scan multiple URLs, create a file containing the URLs to be scanned and run the following command
```bash
go run main.go -file yourfile.txt -output output.txt -subdomain
```

## Options
The following options are available:
<pre>
-url: URL to be scanned.
-file: File containing URLs to be scanned.
-proxy: Proxies in the format http://username:password@ip:port.
-prefix: Prefix of result URLs.
-output: Output file for the results (default: subdomain.txt).
-subdomain: Include subdomains in the scan
</pre>
