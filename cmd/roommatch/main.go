package main

import (
	"log"
	"net/http"
	"os"

	roommatch "example.com/tenant-roommatch"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	service := &roommatch.Service{Matches: roommatch.NewMatchmaker(), Infrai: roommatch.NewClient(key)}
	log.Println("roommatch listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", service.Handler()))
}
