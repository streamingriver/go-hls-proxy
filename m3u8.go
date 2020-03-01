package main

import (
	"bufio"
	"bytes"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func NewM3U8(remap *Remap) *M3U8 {
	m := &M3U8{
		mu:    &sync.RWMutex{},
		cache: &Response{},
		remap: remap,
	}
	return m
}

type M3U8 struct {
	url     string
	cache   *Response
	seen    int64
	running int32
	mu      *sync.RWMutex
	remap   *Remap
}

func (m3u8 *M3U8) Get(url string) *Response {
	if atomic.CompareAndSwapInt32(&m3u8.running, 0, 1) {
		log.Printf("Starting worker....")
		m3u8.url = url
		atomic.StoreInt64(&m3u8.seen, time.Now().Unix()+30)
		m3u8.worker(true, false)
		go m3u8.worker(false, true)
	}
	atomic.StoreInt64(&m3u8.seen, time.Now().Unix()+30)
	m3u8.mu.RLock()
	defer m3u8.mu.RUnlock()
	return m3u8.cache
}

func (m3u8 *M3U8) worker(start, delay bool) {
	for {
		if delay {
			time.Sleep(3 * time.Second)
		}
		if atomic.LoadInt64(&m3u8.seen) < time.Now().Unix() {
			log.Printf("m3u8 worker exiting...")
			atomic.StoreInt32(&m3u8.running, 0)
			return
		}
		response := fetch(m3u8.url)
		if response.err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		m3u8.mu.Lock()
		m3u8.fixTs(response)
		m3u8.cache = response
		m3u8.mu.Unlock()
		if start {
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func (m3u8 *M3U8) fixTs(response *Response) {

	m3u8url1, _ := url.Parse(m3u8.url)

	reader := bytes.NewReader(response.body)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := strings.Trim(string(scanner.Text()), "\n")
		if strings.Contains(line, ".ts") {
			tsurl, _ := m3u8url1.Parse(line)
			tsurl.RawQuery = m3u8url1.RawQuery
			newname, isNew := m3u8.remap.Add(tsurl.String())
			response.body = bytes.ReplaceAll(response.body, []byte(line), []byte(newname))
			if isNew {
				go func(newname string) {
					_, err := (&http.Client{Timeout: 3 * time.Second}).Head("http://" + *flagBindTo + "/" + newname)
					if err != nil {
						log.Printf("%v", err)
					}
					log.Printf("schedule download: %v", tsurl.String())
				}(newname)
			}
		}
	}

}

func bcopy(src []byte) []byte {
	b := make([]byte, len(src))
	copy(b, src)
	return b
}
