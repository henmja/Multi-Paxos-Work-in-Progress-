// +build !solution

package detector

import (
	//"fmt"
	"sync"
)

var rwlock sync.RWMutex

// A MonLeaderDetector represents a Monarchical Eventual Leader Detector as
// described at page 53 in:
// Christian Cachin, Rachid Guerraoui, and Luís Rodrigues: "Introduction to
// Reliable and Secure Distributed Programming" Springer, 2nd edition, 2011.

type MonLeaderDetector struct {

	// TODO(student): Add needed fields
	nodeIDs        []int        // node ids for every node in cluster
	suspected      map[int]bool // map of node ids considered alive
	leaderID       int          //leader ID
	subscriberChan []chan int
}

// NewMonLeaderDetector returns a new Monarchical Eventual Leader Detector
// given a list of node ids.
func NewMonLeaderDetector(nodeIDs []int) *MonLeaderDetector {
	suspected := make(map[int]bool)
	leaderID := -1

	//find highest rank
	for _, nodeID := range nodeIDs {
		if nodeID > leaderID {
			leaderID = nodeID
		}
	}
	rwlock.Lock()
	m := &MonLeaderDetector{
		nodeIDs:   nodeIDs,
		suspected: suspected,
		leaderID:  leaderID,
	}
	rwlock.Unlock()
	return m
}

// Leader returns the current leader. Leader will return UnknownID if all nodes
// are suspected.
func (m *MonLeaderDetector) Leader() int {

	return m.leaderID
}

// Suspect instructs m to consider the node with matching id as suspected. If
// the suspect indication result in a leader change the leader detector should
// this publish this change its subscribers.
func (m *MonLeaderDetector) Suspect(id int) {
	// TODO(student): Implement
	rwlock.Lock()
	m.suspected[id] = true
	rwlock.Unlock()

	// if suspected is the leader, chose new leader
	//if m.leaderID == id {
	/* leaderID := -1
	for _, nodeID := range m.nodeIDs {
		//skip node if its the suspected
		//if nodeID == id {
		//	continue
		//}
		for susID, _ := range m.suspected {
			if nodeID == susID  {
				continue
			}
		}
		//find new leader
		if nodeID > leaderID {
			leaderID = nodeID
			for _, ch := range m.subscriberChan {
				ch <- m.Leader()
				//m.subscriberChan[m.id] <- m.Leader()
			}
		}
	}
	m.leaderID = leaderID */
	m.leaderElection()
	//TODO: notify leader change to subscribers

	//fmt.Println("fuk2")
	//}
}

// Restore instructs m to consider the node with matching id as restored. If
// the restore indication result in a leader change the leader detector should
// this publish this change its subscribers.
func (m *MonLeaderDetector) Restore(id int) {
	// TODO(student): Implement
	rwlock.Lock()
	delete(m.suspected, id)
	rwlock.Unlock()

	//check if leader still has highest rank, if not elect new leader
	m.leaderElection()

	//TODO: notify subscribers of leader change
	/*for _, c := range m.subscriberChan {
		//publish to channel
		fmt.Println(c)
	}*/
	//if (m.Leader()!=oldLeader) {

	//}

}

// Subscribe returns a buffered channel that m on leader change will use to
// publish the id of the highest ranking node. The leader detector will publish
// UnknownID if all nodes become suspected. Subscribe will drop publications to
// slow subscribers. Note: Subscribe returns a unique channel to every
// subscriber; it is not meant to be shared.
func (m *MonLeaderDetector) Subscribe() <-chan int {
	// TODO(student): Implement
	//buffer, non blocking, select send hvis mulig, ellers

	newChan := make(chan int, 8)
	rwlock.Lock()
	m.subscriberChan = append(m.subscriberChan, newChan) //legg til i liste
	rwlock.Unlock()
	return newChan
}

// TODO(student): Add other unexported functions or methods if needed.
func (m *MonLeaderDetector) leaderElection() {
	highestRank := -1
	rwlock.RLock()
	for _, nodeID := range m.nodeIDs {

		if m.suspected[nodeID] {
			continue
		}

		if nodeID > highestRank {
			highestRank = nodeID
		}
	}

	//var oldLeader int
	//oldLeader = m.Leader()
	if m.leaderID != highestRank {
		m.leaderID = highestRank

		for _, ch := range m.subscriberChan {
			ch <- m.Leader()
			//m.subscriberChan[m.id] <- m.Leader()
		}
	}
	rwlock.RUnlock()

}
