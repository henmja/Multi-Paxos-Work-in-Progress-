package detector

import (
	"time"
)

// EvtFailureDetector represents a Eventually Perfect Failure Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.
type EvtFailureDetector struct {
	id        int          // this node's id
	nodeIDs   []int        // node ids for every node in cluster
	alive     map[int]bool // map of node ids considered alive
	suspected map[int]bool // map of node ids  considered suspected

	sr SuspectRestorer // Provided SuspectRestorer implementation

	delay         time.Duration // the current delay for the timeout procedure
	delta         time.Duration // the delta value to be used when increasing delay
	timeoutSignal *time.Ticker  // the timeout procedure ticker

	hbSend chan<- Heartbeat // channel for sending outgoing heartbeat messages
	hbIn   chan Heartbeat   // channel for receiving incoming heartbeat messages
	stop   chan struct{}    // channel for signaling a stop request to the main run loop

	testingHook func() // DO NOT REMOVE THIS LINE. A no-op when not testing.
}

// NewEvtFailureDetector returns a new Eventual Failure Detector. It takes the
// following arguments:
//
// id: The id of the node running this instance of the failure detector.
//
// nodeIDs: A list of ids for every node in the cluster (including the node
// running this instance of the failure detector).
//
// ld: A leader detector implementing the SuspectRestorer interface.
//
// delta: The initial value for the timeout interval. Also the value to be used
// when increasing delay.
//
// hbSend: A send only channel used to send heartbeats to other nodes.
func NewEvtFailureDetector(id int, nodeIDs []int, sr SuspectRestorer, delta time.Duration, hbSend chan<- Heartbeat) *EvtFailureDetector {
	suspected := make(map[int]bool)
	alive := make(map[int]bool)

	// TODO(student): perform any initialization necessary

	//all nodes should be initialized as Alive
	for _, nID := range nodeIDs {
		alive[nID] = true
	}

	return &EvtFailureDetector{
		id:        id,
		nodeIDs:   nodeIDs,
		alive:     alive,
		suspected: suspected,

		sr: sr,

		delay: delta,
		delta: delta,

		hbSend: hbSend,
		hbIn:   make(chan Heartbeat, 8),
		stop:   make(chan struct{}),

		testingHook: func() {}, // DO NOT REMOVE THIS LINE. A no-op when not testing.
	}
}

// Start starts e's main run loop as a separate goroutine. The main run loop
// handles incoming heartbeat requests and responses. The loop also trigger e's
// timeout procedure at an interval corresponding to e's internal delay
// duration variable.
func (e *EvtFailureDetector) Start() {
	e.timeoutSignal = time.NewTicker(e.delay)
	go func() {
		for {
			e.testingHook() // DO NOT REMOVE THIS LINE. A no-op when not testing.
			select {
			case hbRequest := <-e.hbIn:
				// TODO(student): Handle incoming heartbeat

				if hbRequest.Request == true { //Reply was requested
					//construct reply heartbeat
					hbResponse := Heartbeat{To: hbRequest.From, From: e.id, Request: false}
					e.hbSend <- hbResponse

				} else if hbRequest.Request == false { //Reply was recieved, set node alive
					e.alive[hbRequest.From] = true
				}

				//if hb request == false, recieved a reply to a requested heartbead, do neccessary timeout/alive updates
			case <-e.timeoutSignal.C:
				e.timeout()
			case <-e.stop:
				return
			}
		}
	}()
}

// DeliverHeartbeat delivers heartbeat hb to failure detector e.
func (e *EvtFailureDetector) DeliverHeartbeat(hb Heartbeat) {
	e.hbIn <- hb
}

// Stop stops e's main run loop.
func (e *EvtFailureDetector) Stop() {
	e.stop <- struct{}{}
}

// Internal: timeout runs e's timeout procedure.
func (e *EvtFailureDetector) timeout() {
	// TODO(student): Implement timeout procedure

	//Algorithm 2.7: Increasing Timeout
	for aId, _ := range e.alive { //if alive union suspected is not empty
		if _, sState := e.suspected[aId]; sState {
			e.delay += e.delta
			break
		}
	}

	//for all processes p in the set of processes PI, do
	for _, p := range e.nodeIDs {
		//	if p is not an element in alive and p is not an element in sispected, then
		if !e.alive[p] && !e.suspected[p] {
			// p is added to the set of suspected
			e.suspected[p] = true
			// trigger Suspect process on p
			e.sr.Suspect(p)

			// else if p is element in alive and p is element in suspected, then
		} else if e.alive[p] && e.suspected[p] {
			//p is removed from the set of suspected
			delete(e.suspected, p)
			// trigger Restore process on p
			e.sr.Restore(p)
		}

		//trigger send heartbeat request to proccess p
		hbRequest := Heartbeat{To: p, From: e.id, Request: true}
		e.hbSend <- hbRequest

	}

	//set of alive is set to empty
	e.alive = make(map[int]bool)

	//start timer with new delay
	e.timeoutSignal = time.NewTicker(e.delay)
}