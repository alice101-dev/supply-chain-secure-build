// Minimal HTTP service — deliberately boring. The point of this repo is the
// supply chain wrapped around it: SBOM, vulnerability gate, keyless signing,
// SLSA provenance, and signature verification at admission.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "hello from a signed, attested, SBOM-carrying image 📦🔏")
	})

	log.Printf("listening on :%s", port)
	// Plain HTTP by design: in-cluster traffic; TLS terminates at the
	// ingress/mesh (see k8s/networkpolicy — only same-namespace clients).
	// nosemgrep: go.lang.security.audit.net.use-tls.use-tls
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
