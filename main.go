package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// filterURL extracts the host from a raw URL string and trims trailing slashes.
func filterURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err == nil && u.Host != "" {
			return strings.TrimSuffix(u.Host, "/")
		}
		// Fallback for malformed URLs.
		parts := strings.SplitN(raw, "://", 2)
		if len(parts) == 2 {
			raw = parts[1]
		}
	}
	return strings.TrimSuffix(raw, "/")
}

type archiveScanner struct {
	client    *http.Client
	regex     *regexp.Regexp
	subdomain bool
	outMu     sync.Mutex
	writer    *bufio.Writer
}

func newArchiveScanner(client *http.Client, re *regexp.Regexp, subdomain bool, writer *bufio.Writer) *archiveScanner {
	return &archiveScanner{
		client:    client,
		regex:     re,
		subdomain: subdomain,
		writer:    writer,
	}
}

func (s *archiveScanner) scanURL(ctx context.Context, target string) (int, error) {
	var prefix string
	if s.subdomain {
		prefix = "*."
	}

	apiURL := fmt.Sprintf(
		"http://web.archive.org/cdx/search/cdx?url=%s%s/*&output=json&collapse=urlkey",
		prefix, target,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}

	var wrapper [][]string
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return 0, fmt.Errorf("decode json: %w", err)
	}

	if len(wrapper) == 0 {
		return 0, nil
	}

	var count int
	for _, wrap := range wrapper[1:] {
		if len(wrap) < 3 {
			continue
		}
		link := wrap[2]
		if s.regex.MatchString(link) {
			count++
			s.outMu.Lock()
			_, _ = fmt.Fprintln(s.writer, link)
			s.outMu.Unlock()
		}
	}

	return count, nil
}

func run(
	urls []string,
	suffix string,
	output string,
	proxy string,
	subdomain bool,
	workers int,
	timeout time.Duration,
) error {
	// Compile regex for URL suffix filtering.
	suffix = strings.ReplaceAll(suffix, ",", "|")
	if suffix == "" {
		suffix = ".*"
	}
	pattern := fmt.Sprintf("(%s)$", suffix)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid suffix regex: %w", err)
	}

	// Configure HTTP client.
	client := &http.Client{Timeout: timeout}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return fmt.Errorf("invalid proxy: %w", err)
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	// Open output file with buffered writer.
	f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	defer writer.Flush()

	sc := newArchiveScanner(client, re, subdomain, writer)
	ctx := context.Background()

	jobs := make(chan string, workers)
	var wg sync.WaitGroup

	// Start worker pool.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				if target == "" {
					continue
				}
				count, err := sc.scanURL(ctx, target)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[-] %s error: %v\n", target, err)
					continue
				}
				fmt.Printf("[+] %s => Found %d urls\n", target, count)
			}
		}()
	}

	// Dispatch jobs.
	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	wg.Wait()
	return nil
}

func main() {
	var (
		urlStr    string
		fileInput string
		proxy     string
		suffix    string
		output    string
		subdomain bool
		workers   int
		timeout   time.Duration
	)

	flag.StringVar(&urlStr, "url", "", "URL to scan")
	flag.StringVar(&fileInput, "file", "", "File containing URLs to scan")
	flag.StringVar(&proxy, "proxy", "", "Proxy URL (http://ip:port)")
	flag.StringVar(&suffix, "suffix", "", "Suffix filter for result URLs (comma separated)")
	flag.StringVar(&output, "output", "subdomain.txt", "Output file")
	flag.BoolVar(&subdomain, "subdomain", false, "Include subdomains")
	flag.IntVar(&workers, "workers", 20, "Number of concurrent workers")
	flag.DurationVar(&timeout, "timeout", 10*time.Second, "HTTP timeout per request")
	flag.Parse()

	if urlStr == "" && fileInput == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	var urls []string
	if urlStr != "" {
		urls = append(urls, filterURL(urlStr))
	}

	if fileInput != "" {
		f, err := os.Open(fileInput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if u := filterURL(scanner.Text()); u != "" {
				urls = append(urls, u)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "read file: %v\n", err)
			os.Exit(1)
		}
	}

	if err := run(urls, suffix, output, proxy, subdomain, workers, timeout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
