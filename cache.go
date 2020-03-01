package main

import (
	"container/list"
	"sync"
)

type FIFOCache struct {
	l      *list.List
	m      map[string]struct{}
	length int
	mu     *sync.RWMutex
}

func (fc *FIFOCache) Init(length int) {
	fc.l = list.New()
	fc.m = make(map[string]struct{})
	fc.length = length
	fc.mu = &sync.RWMutex{}
}

func (fc *FIFOCache) Set(url string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.l.PushFront(url)
	fc.m[url] = struct{}{}
}

func (fc *FIFOCache) removeLast() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	for fc.l.Len() >= fc.length {
		item := fc.l.Back()
		fc.l.Remove(item)
	}
}

func (fc *FIFOCache) Check(url string) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	_, ok := fc.m[url]
	return ok
}
