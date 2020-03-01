package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

func NewM3U8() *M3U8 {
	m := &M3U8{
		mu:    &sync.RWMutex{},
		cache: &Response{},
	}
	return m
}

type M3U8 struct {
	url     string
	cache   *Response
	seen    int64
	running int32
	mu      *sync.RWMutex
}

func (m3u8 *M3U8) Get(url string) *Response {
	if atomic.CompareAndSwapInt32(&m3u8.running, 0, 1) {
		log.Printf("Starting worker....")
		m3u8.url = url
		atomic.StoreInt64(&m3u8.seen, time.Now().Unix()+10)
		m3u8.worker(true, false)
		go m3u8.worker(false, true)
	}
	atomic.StoreInt64(&m3u8.seen, time.Now().Unix()+5)
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
		m3u8.cache = response
		m3u8.mu.Unlock()
		if start {
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func bcopy(src []byte) []byte {
	b := make([]byte, len(src))
	copy(b, src)
	return b
}
