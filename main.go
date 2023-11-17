package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	urlparse "net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

func filterUrl(url string) string {
	if strings.Contains(url, "://") {
		url = strings.Split(url, "/")[2]
	}
	if strings.HasSuffix(url, "/") {
		url = strings.Join(
			strings.Split(url, "")[:(len(strings.Split(url, ""))-1)], "",
		)
	}
	return strings.ReplaceAll(url, "\r", "")
}

func handleError(err error, msg string) {
	if err != nil {
		fmt.Printf("%s: %s\n", msg, err.Error())
	}
}

func ScanUrl(client *http.Client, url string, subdomain bool, regex *regexp.Regexp, file *os.File, sem chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		<-sem
		if r := recover(); r != nil {
			fmt.Println("Recovered in ScanUrl", r)
		}
	}()

	var suffixsub string
	var output []string
	var wrapper [][]string

	if subdomain {
		suffixsub = "*."
	} else {
		suffixsub = ""
	}

	api := fmt.Sprintf("http://web.archive.org/web/timemap/json?url=%s%s/*&output=json&collapse=urlkey", suffixsub, url)
	resp, err := client.Get(api)
	handleError(err, fmt.Sprintf("%s error making request", url))

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	handleError(err, fmt.Sprintf("%s error reading body", url))

	err = json.Unmarshal(body, &wrapper)
	handleError(err, fmt.Sprintf("%s error decoding json", url))

	if len(wrapper) == 0 {
		return
	}

	for _, wrap := range wrapper[1:] {
		if regex.Match([]byte(wrap[2])) {
			output = append(output, wrap[2])
			file.WriteString(wrap[2] + "\n")
		}
	}

	fmt.Printf("%s => Found %d urls\n", url, len(output))
}

func main() {
	var urls []string
	var url, fileInput, proxy, suffix, output string
	var subdomain bool
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	flag.StringVar(&url, "url", "", "Url to be scan")
	flag.StringVar(&fileInput, "file", "", "File urls to be scan")
	flag.StringVar(&proxy, "proxy", "", "Proxies format http://ip:port")
	flag.StringVar(&suffix, "suffix", "", "suffix of results url")
	flag.StringVar(&output, "output", "subdomain.txt", "Output of results")
	flag.BoolVar(&subdomain, "subdomain", false, "Subdomain url to be included")

	flag.Parse()
	if url == "" && fileInput == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	suffix = strings.ReplaceAll(suffix, ",", "|")
	suffix = fmt.Sprintf(
		"(%s)$",
		suffix,
	)
	regex := regexp.MustCompile(suffix)
	if url != "" {
		urls = append(urls, filterUrl(url))
	}

	if fileInput != "" {
		file, err := os.ReadFile(fileInput)
		handleError(err, "file err")
		for _, url := range strings.Split(string(file), "\n") {
			urls = append(urls, filterUrl(url))
		}
	}
	if proxy != "" {
		proxyUrl, err := urlparse.Parse(proxy)
		handleError(err, "error setting proxy")
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
	}
	fmt.Printf("Starting scan %d urls\n\n", len(urls))
	var wg sync.WaitGroup
	sem := make(chan bool, 20) // limit to 20 concurrent goroutines
	fileOutput, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	handleError(err, "error opening file")
	defer fileOutput.Close()
	for _, url := range urls {
		wg.Add(1)
		sem <- true
		go ScanUrl(client, url, subdomain, regex, fileOutput, sem, &wg)
	}
	wg.Wait()
}
