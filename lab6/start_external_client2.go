package main

import cm "dat520.github.io/lab6/gorilla/websocket/examples/chat"

// Main entry point to the external client
func main() {
	done := make(chan bool)        // Boolean channel to end this program
	extCl, _ := cm.InitExtClient() // Create new external client instance

	extCl.Start()      // Start external client
	defer extCl.Stop() // Stop external client

	<-done // Ends this program when this channel receives - true
}
