package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/grafov/m3u8"
	lru "github.com/hashicorp/golang-lru"
	"golang.org/x/sync/singleflight"
)

var (
	flagURL    = flag.String("url", "", "m3u8 url")
	flagFE     = flag.String("frontend", "", "frontend")
	flagBindTo = flag.String("bind-to", ":0", "bind to port")
	flagName   = flag.String("name", "test", "channel name")

	re = regexp.MustCompile(".*?.ts")

	lrucache, _ = lru.New(10)

	sfl = singleflight.Group{}

	m3u8mu    = &sync.RWMutex{}
	m3u8cache = make(map[string]*M3U8)
)

func CacheGet(url string, remap *Remap) *Response {
	m3u8mu.Lock()
	m3u8fetcher, ok := m3u8cache[url]
	if !ok {

		m3u8fetcher = NewM3U8(remap)
		m3u8cache[url] = m3u8fetcher
	}
	m3u8mu.Unlock()
	return m3u8fetcher.Get(url)
}

func main() {
	flag.Parse()
	if *flagURL == "" {
		println("set --url http://url.here")
		return
	}
	if *flagFE != "" {
		go pinger()
	}

	if *flagBindTo == ":0" {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			panic(err)
		}
		*flagBindTo = l.Addr().String()
		l.Close()
	}

	remap := new(Remap)
	remap.Init()
	m3u8fetcher := NewM3U8(remap)
	_ = m3u8fetcher

	remap2 := new(Remap)
	remap2.Init()
	m3u8fetcher2 := NewM3U8(remap2)

	_url := *flagURL
	m3u8url1, _ := url.Parse(_url)
	// restart:
	response := fetch(m3u8url1.String())
	buf := bytes.NewBuffer(response.body)
	pl, pt, err := m3u8.Decode(*buf, true)
	if err != nil {
		log.Fatalf("Cant parse input m3u8 playlist: %v", err)
	}
	isMasterPlaylist := false
	if pt == m3u8.MASTER {
		masterpl := pl.(*m3u8.MasterPlaylist)
		if len(masterpl.Variants) == 0 {
			log.Fatalf("Cant parse input m3u8 playlist")
		}
		// m3u8url1, _ = m3u8url1.Parse(masterpl.Variants[0].URI)
		isMasterPlaylist = true
		// goto restart
	}
	//check if we have final playlist
	// response := fetch(m3u8url1.String())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.EscapedPath(), "/")
		parts = parts[1:]
		newurl := strings.Join(parts, "/")

		if strings.HasSuffix(r.URL.EscapedPath(), "stream.m3u8") {
			// println(m3u8url1.String())
			var response *Response
			if !isMasterPlaylist {
				response = CacheGet(m3u8url1.String(), remap)
			} else {
				response = m3u8fetcher2.GetSimple(m3u8url1.String(), r.URL.Query().Get("token"))
			}
			if response.err != nil {
				log.Printf("%v", response.err)
				if response.err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET")
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(response.body)))
			w.Write(response.body)

		} else if strings.HasSuffix(r.URL.EscapedPath(), ".m3u8") {
			m3u8url2, _ := m3u8url1.Parse(newurl)
			m3u8url2.RawQuery = m3u8url1.RawQuery
			response := CacheGet(m3u8url2.String(), remap)
			if response.err != nil {
				log.Printf("%v", response.err)
				if response.err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET")
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
			if r.Method == "HEAD" {
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET")
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
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET")
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(response.body)))
			w.Write(response.body)
		}
	})
	log.Printf("Starting server on " + *flagBindTo)
	http.ListenAndServe(*flagBindTo, nil)
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
	request.Header.Set("User-Agent", "streamingriveriptv/1.0")

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
