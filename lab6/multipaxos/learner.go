// +build !solution

package multipaxos

// Learner represents a learner as defined by the Multi-Paxos algorithm.
type Learner struct {
	// TODO(student)
	id     int // This node ID
	n      int // Number of nodes
	quorum int // Quorum value

	recv_learn_msg_channel    chan Learn          // Channel to recv Learn message
	send_decided_val_channel chan<- DecidedValue // Channel to send decided value
	stop       chan struct{}       // Channel for signalling a stop request to the start run loop
	learns     map[string]Learn    // receiving learn holder
}

// NewLearner returns a new single-decree Paxos learner. It takes the
// following arguments:
//
// id: The id of the node running this instance of a Paxos learner.
//
// nrOfNodes: The total number of Paxos nodes.
//
// send_decided_val_channel: A send only channel used to send values that has been learned,
// i.e. decided by the Paxos nodes.
func NewLearner(id int, nrOfNodes int, send_decided_val_channel chan<- DecidedValue) *Learner {
	return &Learner{
		// TODO(student)
		id:     id,
		n:      nrOfNodes,
		quorum: (nrOfNodes / 2) + 1,

		recv_learn_msg_channel:    make(chan Learn),
		send_decided_val_channel: send_decided_val_channel,
		stop:       make(chan struct{}),
		learns:     make(map[string]Learn),
	}
}

// Start starts l's main run loop as a separate goroutine. The main run loop
// handles incoming learn messages.
func (l *Learner) Start() {
	go func() {
		for {
			// TODO(student)
			select {
			case learn := <-l.recv_learn_msg_channel:
				val, slotID, output := l.handleLearn(learn)
				if output {
					l.send_decided_val_channel <- DecidedValue{SlotID: slotID, Value: val}
				}
			case <-l.stop:
				return
			}
		}
	}()
}

// Stop stops l's main run loop.
func (l *Learner) Stop() {
	// TODO(student)
	l.stop <- struct{}{}
}

// DeliverLearn delivers learn learn to learner l.
func (l *Learner) DeliverLearn(learn Learn) {
	// TODO(student)
	l.recv_learn_msg_channel <- learn
}



func (l *Learner) remove_learn(round Round, slt SlotID) {
	for k, learn := range l.learns {
		if learn.Rnd == round && learn.Slot == slt {
			delete(l.learns, k)
		}
	}
}

func (l *Learner) check_if_maj_round(round_slot []SlotID, slt int) bool {
	count := 0
	for _, sl := range round_slot {
		if int(sl) == slt {
			count++
		}
		if count >= l.quorum {
			return true
		}
	}
	return false
}




// Internal: handleLearn processes learn learn according to the Multi-Paxos
// algorithm. If handling the learn results in learner l emitting a
// corresponding decided value, then output will be true, sid the id for the
// slot that was decided and val contain the decided value. If handleLearn
// returns false as output, then val and sid will have their zero value.
func (l *Learner) handleLearn(learn Learn) (val Value, sid SlotID, output bool) {

	round_count := make(map[int]int)
	round_slot := make(map[int][]SlotID)
	slot_count := make(map[int]int)
	round_slot_val := make(map[string]Value)



	key := string(learn.From) + "_" + string(learn.Slot)

	prev_learn, ok := l.learns[key]
	if (ok && learn.Rnd > prev_learn.Rnd) || !ok {
		l.learns[key] = learn
	}


	//return the chosen value
	for _, learn := range l.learns {
		round_count[int(learn.Rnd)]++
		slot_count[int(learn.Slot)]++
		key := string(learn.Rnd) + "_" + string(learn.Slot)
		round_slot_val[key] = learn.Val

		answer, ok := round_slot[int(learn.Rnd)]
		if ok {
			round_slot[int(learn.Rnd)] = append(answer, learn.Slot)
		} else {
			round_slot[int(learn.Rnd)] = []SlotID{learn.Slot}
		}
	}

	for round, count := range round_count {
		if count >= l.quorum {
			for slt, count := range slot_count {
				if count >= l.quorum {
					ok := l.check_if_maj_round(round_slot[round], slt)
					if ok {
						//remove already learned values
						l.remove_learn(Round(round), SlotID(slt))

						key := string(round) + "_" + string(slt)
						return round_slot_val[key], SlotID(slt), true
					}
				}
			}
		}
	}
	return Value{}, 0, false


}


