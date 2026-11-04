package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		fmt.Fprintln(w, "Welcome to GoGate API Gateway!")

	})

	fmt.Println("GoGate running on port 8080")

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		log.Fatal(err)
	}
}
