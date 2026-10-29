package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/graydraft/gocommon/sealed"
)

func main() {
	out := flag.String("out", "deploy/nginx/ssl/graydraft.com.key", "path to write the TLS private key")
	flag.Parse()
	pem := sealed.TLSKey()
	if pem == "" {
		fmt.Fprintln(os.Stderr, "tls key decrypt failed")
		os.Exit(1)
	}
	if err := os.WriteFile(*out, []byte(pem), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
