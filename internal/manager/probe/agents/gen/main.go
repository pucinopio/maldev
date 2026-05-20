package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/oioio-space/maldev/license/hostid"
)

type result struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	LocalHex     string `json:"local_hex"`
	CompositeHex string `json:"composite_hex"`
	SentAt       string `json:"sent_at"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: probe <result-url>")
		os.Exit(2)
	}
	local, _ := hostid.Local()
	composite, _ := hostid.Composite()
	host, _ := os.Hostname()
	body, _ := json.Marshal(result{
		Hostname:     host,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		LocalHex:     hex.EncodeToString(local),
		CompositeHex: hex.EncodeToString(composite),
		SentAt:       time.Now().UTC().Format(time.RFC3339),
	})
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(os.Args[1], "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		fmt.Fprintf(os.Stderr, "probe: server returned %s\n", resp.Status)
		os.Exit(1)
	}
	fmt.Println("OK")
}
