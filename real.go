package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/jackdanger/collectlinks"
)

type Fetcher interface {
	// Fetch returns the body of URL and
	// a slice of URLs found on that page.
	Fetch(url string) (body string, urls []string, err error)
}

// fetched tracks URLs that have been (or are being) fetched.
// The lock must be held while reading from or writing to the map.
var fetched = struct {
	m   map[string]error
	mux sync.Mutex
}{m: make(map[string]error)}

var loading = errors.New("url load in progress") // sentinel value

// Crawl uses fetcher to recursively crawl
// pages starting with url, to a maximum of depth.
func Crawl(url string, depth int, fetcher Fetcher) {
	if depth <= 0 {
		fmt.Printf("<- Done with %v, depth 0.\n", url)
		return
	}

	fetched.mux.Lock()
	if _, ok := fetched.m[url]; ok {
		fetched.mux.Unlock()
		fmt.Printf("<- Done with %v, already fetched.\n", url)
		return
	}
	// mark the url to be loading to avoid others reloading it at the same time.
	fetched.m[url] = loading
	fetched.mux.Unlock()

	// load it concurrently.
	body, urls, err := fetcher.Fetch(url)

	// And update the status in a synced zone.
	fetched.mux.Lock()
	fetched.m[url] = err
	fetched.mux.Unlock()

	if err != nil {
		fmt.Printf("<- Error on %v: %v\n", url, err)
		return
	}
	fmt.Printf("Found: %s %q\n", url, body)
	done := make(chan bool)
	for i, u := range urls {
		fmt.Printf("-> Crawling child %v/%v of %v : %v.\n", i, len(urls), url, u)
		go func(url string) {
			Crawl(url, depth-1, fetcher)
			done <- true
		}(u)
	}
	for i, u := range urls {
		fmt.Printf("<- [%v] %v/%v Waiting for child %v.\n", url, i, len(urls), u)
		<-done
	}
	fmt.Printf("<- Done with %v\n", url)
}


type myFetcher int

func (f myFetcher) Fetch(url string) (string, []string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", nil, fmt.Errorf("not found: %s", url)
	}
	defer resp.Body.Close()
	// body, _ := ioutil.ReadAll(resp.Body)
	urls := collectlinks.All(resp.Body)

	// fmt.Println("urls", urls)
	return "", urls, nil
}

var fetcher = myFetcher(1)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Please specify start page")
		os.Exit(1)
	}
	Crawl(args[0], 3, fetcher)

	fmt.Println("Fetching stats\n--------------")
	for url, err := range fetched.m {
		if err != nil {
			fmt.Printf("%v failed: %v\n", url, err)
		} else {
			fmt.Printf("%v was fetched\n", url)
		}
	}
}
