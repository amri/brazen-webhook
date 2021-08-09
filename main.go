
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func handler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	fmt.Println(err)
	fmt.Println(string(body))
	fmt.Fprintf(w, string(body), r.URL.Path[1:])
}

func main() {
  
  port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}
  
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
