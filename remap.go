package main

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type Remap struct {
	m  map[string]string
	rm map[string]string
	l  *list.List
	mu *sync.RWMutex
}

func (r *Remap) Init() {
	r.m = make(map[string]string)
	r.rm = make(map[string]string)
	r.l = list.New()
	r.mu = &sync.RWMutex{}
}

func (r *Remap) Add(url string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	value, ok := r.m[url]
	if ok {
		return value
	}
	newurl := fmt.Sprintf("%d.ts", time.Now().UnixNano())
	r.m[url] = newurl
	r.rm[newurl] = url
	r.l.PushFront(url)

	r.removeLast()

	return r.m[url]
}

func (r *Remap) Get(url string) string {
	value, ok := r.rm[url]
	if ok {
		return value
	}
	return url
}

func (r *Remap) removeLast() {
	for r.l.Len() >= 10 {
		item := r.l.Back()
		u := r.m[item.Value.(string)]
		url := r.rm[u]
		delete(r.rm, u)
		delete(r.m, url)
		r.l.Remove(item)
	}
}