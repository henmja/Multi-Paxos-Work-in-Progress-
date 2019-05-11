package main

import (
	"flag"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":3389", "http service address")

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home3.html")
}

// Main entry point to the external client

func main() {

	flag.Parse()
	hub := newHub()
	go hub.run()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

	done := make(chan bool)     // Boolean channel to end this program
	extCl, _ := InitExtClient() // Create new external client instance

	extCl.Start()      // Start external client
	defer extCl.Stop() // Stop external client

	<-done // Ends this program when this channel receives - true

}
