package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func pinger() {
	hc := http.Client{Timeout: 3 * time.Second}
	for {
		url := fmt.Sprintf("%s/register/backend/%s/%s", *flagFE, *flagName, getPort(*flagBindTo))
		_ = url
		response, err := hc.Post(url, "text/plain", nil)
		if err != nil {
			log.Printf("%v", err)
			time.Sleep(time.Second)
			continue
		}
		if response.StatusCode/100 == 2 {
			response.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
}

func getAddr(addr string) string {
	remote_addr := addr
	idx := strings.LastIndex(remote_addr, ":")
	if idx != -1 {
		remote_addr = remote_addr[0:idx]
		if remote_addr[0] == '[' && remote_addr[len(remote_addr)-1] == ']' {
			remote_addr = remote_addr[1 : len(remote_addr)-1]
		}
	}
	return remote_addr
}

func getPort(addr string) string {
	remote_addr := addr
	idx := strings.LastIndex(remote_addr, ":")
	if idx != -1 {
		remote_addr = remote_addr[idx+1:]
		if remote_addr[0] == '[' && remote_addr[len(remote_addr)-1] == ']' {
			remote_addr = remote_addr[1 : len(remote_addr)-1]
		}
	}
	return remote_addr
}
