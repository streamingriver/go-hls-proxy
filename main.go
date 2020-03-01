package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/sync/singleflight"
)

var (
	flagURL = flag.String("url", "", "m3u8 url")

	re = regexp.MustCompile(".*?.ts")

	lrucache, _ = lru.New(10)

	sfl = singleflight.Group{}
)

func main() {
	flag.Parse()
	if *flagURL == "" {
		println("set --url http://url.here")
		return
	}

	remap := new(Remap)
	remap.Init()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.EscapedPath(), "/")
		parts = parts[1:]
		newurl := strings.Join(parts, "/")
		if strings.HasSuffix(r.URL.EscapedPath(), ".m3u8") {
			m3u8url, _ := url.Parse(*flagURL)
			m3u8url, _ = m3u8url.Parse(newurl)
			response := fetch(m3u8url.String())
			if response.err != nil {
				log.Printf("%v", response.err)
				if response.err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}

			reader := bytes.NewReader(response.body)
			scanner := bufio.NewScanner(reader)

			for scanner.Scan() {
				line := strings.Trim(string(scanner.Text()), "\n")
				if strings.Contains(line, ".ts") {
					tsurl, _ := m3u8url.Parse(line)
					tsurl.RawQuery = m3u8url.RawQuery
					newname := remap.Add(tsurl.String())
					response.body = bytes.ReplaceAll(response.body, []byte(line), []byte(newname))
				}
			}

			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(response.body)))
			w.Write(response.body)

		} else if strings.HasSuffix(r.URL.EscapedPath(), ".ts") {
			m3u8url, _ := url.Parse(*flagURL)
			tsurl, _ := m3u8url.Parse(remap.Get(newurl))
			rsp, _, _ := sfl.Do(tsurl.String(), func() (interface{}, error) {
				value, ok := lrucache.Get(tsurl.String())
				if ok {
					return value.(*Response), value.(*Response).err
				}
				response := fetch(tsurl.String())
				lrucache.Add(tsurl.String(), response)
				return response, response.err

			})
			response := rsp.(*Response)
			if response.err != nil {
				log.Printf("%v", response.err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/vnd.trolltech.linguist")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(response.body)))
			w.Write(response.body)
		} else {
			response := fetch(*flagURL)
			if response.err != nil {
				log.Printf("%v", response.err)
				if response.err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(response.body)))
			w.Write(response.body)
		}
	})
	log.Printf("Starting server on :8080")
	http.ListenAndServe(":8080", nil)
}

type Response struct {
	body    []byte
	headers http.Header
	err     error
}

func fetch(url string) *Response {

	log.Printf("fetching: %v", url)

	hc := http.Client{Timeout: 10 * time.Second}

	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Set("User-Agent", "iptv/1.0")

	response, err := hc.Do(request)
	if err != nil {
		return &Response{
			err: err,
		}
	}

	if response.StatusCode/100 != 2 {
		return &Response{
			err: fmt.Errorf("Invalid status code: %v", response.StatusCode),
		}
	}
	defer response.Body.Close()
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return &Response{
			err: err,
		}
	}
	return &Response{
		body: b,
	}
}
