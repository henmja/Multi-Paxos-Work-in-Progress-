package app

import dtr "dat520.github.io/lab3/detector"

// Definitions - DO NOT EDIT

var NATIVE_CONN_ADDR string // This node's IP address

// Dictionary of nodeID and it's hostname
var mapOfNodeIDsIPAddr = map[int]string{
	0: "152.94.1.100",
	1: "152.94.1.110",
	2: "152.94.1.118",
}

// App data
type DistributedApp struct {
	leader int                // Leader value
	ld     dtr.LeaderDetector // Interface to leader detector
	stop   chan struct{}      // Channel for signalling a stop request to the start run loop
}
