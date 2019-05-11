// +build !solution

package multipaxos

import "sort"
// Acceptor represents an acceptor as defined by the Multi-Paxos algorithm.
type Acceptor struct {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	// Add needed fields

        accepted_rounds Round // Accepted round so far
        prev_voted_val   Value // Previous voted value
	prev_voted_round   Round // Previous voted round
	slot_id   SlotID // Slot id
	prev_slot_id  SlotID // previous Slot id
	slots map[SlotID]PromiseSlot

	id         int
        stop       chan bool      // Channel for signalling a stop request to the start run loop
        send_learn_msg_chan   chan<- Learn   // Channel to send Learn message
        recv_accept_msg_chan   chan Accept    // Channel to recv Accept message
        send_pomise_msg_chan chan<- Promise // Channel to send Promise message
        recv_prepare_msg_chan  chan Prepare   // Channel to recv Prepare message



}

// NewAcceptor returns a new single-decree Paxos acceptor.
// It takes the following arguments:
//
// id: The id of the node running this instance of a Paxos acceptor.
//
// send_pomise_msg_chan: A send only channel used to send promises to other nodes.
//
// send_learn_msg_chan: A send only channel used to send learns to other nodes.
func NewAcceptor(id int, send_pomise_msg_chan chan<- Promise, send_learn_msg_chan chan<- Learn) *Acceptor {
	//TODO(student): Task 2 and 3 - algorithm and distributed implementation
	return &Acceptor{


                accepted_rounds:     NoRound,
                prev_voted_round:       NoRound,
		slots : make(map[SlotID]PromiseSlot),
		id:         id,
                stop:       make(chan bool),
                send_learn_msg_chan:   send_learn_msg_chan,
                recv_accept_msg_chan:   make(chan Accept),
                send_pomise_msg_chan: send_pomise_msg_chan,
                recv_prepare_msg_chan:  make(chan Prepare),



		}
}

// Start starts a's main run loop as a separate goroutine. The main run loop
// handles incoming prepare and accept messages.
func (a *Acceptor) Start() {
	go func() {
		for {
			//TODO(student): Task 3 - distributed implementation

			select {
 			case <-a.stop:
                                return

		   	case accept_msg := <-a.recv_accept_msg_chan:
                                learner_msg, out := a.handleAccept(accept_msg)
                                if out {
                                        a.send_learn_msg_chan <- learner_msg
                                }
                        case prepare_msg := <-a.recv_prepare_msg_chan:
                                promise_msg, out := a.handlePrepare(prepare_msg)
                                if out {
                                        a.send_pomise_msg_chan <- promise_msg
                                }
                       
                        }

		}
	}()
}

// Stop stops a's main run loop.
func (a *Acceptor) Stop() {
	//TODO(student): Task 3 - distributed implementation
	 a.stop <- true
}

// DeliverPrepare delivers prepare prepare_msg to acceptor a.
func (a *Acceptor) DeliverPrepare(prepare_msg Prepare) {
	//TODO(student): Task 3 - distributed implementation
	a.recv_prepare_msg_chan <- prepare_msg
}

// DeliverAccept delivers accept acc to acceptor a.
func (a *Acceptor) DeliverAccept(acc Accept) {
	//TODO(student): Task 3 - distributed implementation
	a.recv_accept_msg_chan <- acc
}

// Internal: handlePrepare processes prepare prp according to the Multi-Paxos
// algorithm. If handling the prepare results in acceptor a emitting a
// corresponding promise, then output will be true and prm contain the promise.
// If handlePrepare returns false as output, then prm will be a zero-valued
// struct.
func (a *Acceptor) handlePrepare(prepare_msg Prepare) (prm Promise, output bool) {
	//TODO(student): Task 2 - algorithm implementation
	//return Promise{To: -1, From: -1, prev_voted_round: -2, prev_voted_val: "FooBar"}, true




	 if prepare_msg.Crnd > a.accepted_rounds {
                a.accepted_rounds = prepare_msg.Crnd
                a.slot_id = prepare_msg.Slot





		accepted_slots := []PromiseSlot{}
                for slot_id, promised_slot := range a.slots {
                        if slot_id >= prepare_msg.Slot {
				//find all previous promises
                                accepted_slots = append(accepted_slots, promised_slot )
                        }
                }

                sort.Sort(slot_data(accepted_slots)) 

		//Sort the based on SlotID to maintain sequence in state
                promise := Promise{To: prepare_msg.From, From: a.id, Rnd: a.accepted_rounds}

                if len(accepted_slots) > 0 {
                        promise.Slots = accepted_slots
                }


                return promise, true
        } else {
                return Promise{}, false
        }

}

// Internal: handleAccept processes accept acc according to the Multi-Paxos
// algorithm. If handling the accept results in acceptor a emitting a
// corresponding learn, then output will be true and lrn contain the learn.  If
// handleAccept returns false as output, then lrn will be a zero-valued struct.
func (a *Acceptor) handleAccept(acc Accept) (lrn Learn, output bool) {
	//TODO(student): Task 2 - algorithm implementation
	//return Learn{From: -1, Rnd: -2, Val: "FooBar"}, true
	if acc.Rnd >= a.accepted_rounds { // New round, or same round (incase of duplicate accept message)
		a.accepted_rounds = acc.Rnd                
                a.prev_voted_val = acc.Val
		a.prev_voted_round = acc.Rnd
		a.slot_id = acc.Slot
		a.prev_slot_id = acc.Slot
		
		accepted_slot := PromiseSlot{ID: acc.Slot, Vrnd: a.prev_voted_round, Vval: a.prev_voted_val}
			
		prev_slot, ok := a.slots[acc.Slot]

                if (ok && prev_slot.Vrnd < acc.Rnd) || !ok { 
		// Add new accept, 
		//or update old accept with higher round for a SlotID
                        a.slots[acc.Slot] = accepted_slot
                }


	

                learn := Learn{
				a.id,
				a.slot_id,
				a.prev_voted_round,
				a.prev_voted_val,
			}

                return learn, true
        } else {
                return Learn{}, false
        }


}







// Sort the Promise Slot by ID
type slot_data []PromiseSlot

// This function is used by sort interface to find the length of slice
func (slice slot_data) Len() int {
        return len(slice)
}

// This function is used by sort interface to find minimum ID
func (slice slot_data) Less(i, j int) bool {
	return slice[i].ID < slice[j].ID
}

// This function is used by sort interface to swap entries
func (slice slot_data) Swap(i, j int) {
        slice[i], slice[j] = slice[j], slice[i]
}






