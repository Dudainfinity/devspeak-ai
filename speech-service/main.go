package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func healthCheck(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "DevSpeak AI Speech Service Running")
}

func main() {
	http.HandleFunc("/health", healthCheck)

	http.Handle("/metrics", promhttp.Handler())

	fmt.Println("Server running on port 8080")

	http.ListenAndServe(":8080", nil)
}
