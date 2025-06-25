package main

import (
	"net/http"

	"github.com/rescp17/lanFileSharer/api" // Adjust the import path as necessary
)

func main() {

	http.HandleFunc("/ask", api.AskHandler) // Register the AskHandler
	http.ListenAndServe(":8080", nil)
}
