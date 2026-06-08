// lego-dns-paneldns — exec-compatible ACME DNS-01 hook for PanelDNS.
//
// Used as the EXEC_PATH binary for lego's exec DNS provider:
//
//	PANELDNS_URL=https://app.paneldns.com \
//	PANELDNS_TOKEN=dnsm_xxxx \
//	EXEC_PATH=/usr/local/bin/lego-dns-paneldns \
//	lego --dns exec --dns.resolvers 1.1.1.1 --email you@example.com \
//	     --domains "*.example.com" run
//
// Environment variables (set by lego):
//
//	LEGO_CA_CERTIFICATES   path to CA bundle (optional)
//
// Arguments passed by lego:
//
//	present <fqdn> <value>   create the TXT record
//	cleanup <fqdn> <value>   delete the TXT record
package main

import (
	"fmt"
	"os"

	"github.com/Veeau/lego-dns-paneldns/paneldns"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: lego-dns-paneldns <present|cleanup> <fqdn> <value>")
		os.Exit(1)
	}

	action := os.Args[1]
	fqdn := os.Args[2]
	value := os.Args[3]

	p, err := paneldns.NewDNSProvider()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch action {
	case "present":
		if err := p.ExecPresent(fqdn, value); err != nil {
			fmt.Fprintf(os.Stderr, "present failed: %v\n", err)
			os.Exit(1)
		}
	case "cleanup":
		if err := p.ExecCleanup(fqdn, value); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown action: %s\n", action)
		os.Exit(1)
	}
}
