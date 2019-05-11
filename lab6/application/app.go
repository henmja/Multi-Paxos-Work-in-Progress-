package app

import (
	"fmt"
	"log"

	dtr "dat520.github.io/lab3/detector"
)

// Init instance of an application
func InitApp(ld *dtr.MonLeaderDetector) (*DistributedApp, error) {
	appHandle := DistributedApp{
		leader: -1,
		ld:     ld,
		stop:   make(chan struct{}),
	}

	return &appHandle, nil
}

// Main start point for application
func (a *DistributedApp) Start() {
	log.SetFlags(log.Ldate | log.Ltime)
	var gotLeader int
	ldrSubscriber := a.ld.Subscribe()

	go func() {
		for {
			select {
			case gotLeader = <-ldrSubscriber: // Listen for leader change
				fmt.Printf("\nNew Leader : Id = %d & Ip = %s", gotLeader, mapOfNodeIDsIPAddr[gotLeader])
				a.leader = gotLeader
			case <-a.stop:
				return
			}
		}
	}()
}

// Stop stops application's main run loop.
func (a *DistributedApp) Stop() {
	a.stop <- struct{}{}
}
