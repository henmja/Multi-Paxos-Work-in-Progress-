package ben

import (
	"net"
	"os"

	bank "dat520.github.io/glab4/lab6/bank"
	mlpx "dat520.github.io/glab4/lab6/multipaxos"
)

// Definitions - DO NOT EDIT

var NATIVE_CONN_ADDR string // This node's IP address

const (
	NW_CONN_PROTO      = "tcp"   // Network connection protocol
	TCP_CONN_PORT      = "29170" // Network connection protocol port
	LDR_CONN_NUM_RETRY = 3       // Number of retries for connecting to the leader
)

// List of NodeIDs of cluster
var ClusterOfThree = []int{0, 1, 2}

// Dictionary of node and it's conn handle
var clusterAddrConns = map[string]net.Conn{
	"152.94.1.100": nil,
	"152.94.1.110": nil,
	"152.94.1.118": nil,
}

// Dictionary of hostname and it's nodeid
var MapOfIPAddrNodeIDs = map[string]int{
	"152.94.1.100": 0,
	"152.94.1.110": 1,
	"152.94.1.118": 2,
}

// Dictionary of nodeID and it's hostname
var mapOfNodeIDsIPAddr = map[int]string{
	0: "152.94.1.100",
	1: "152.94.1.110",
	2: "152.94.1.118",
}

// Client data
type DistributedExtCl struct {
	serverIPAddrConns         map[string]net.Conn // Dictionary of node and it's conn handle
	randNodeID                int                 // Random node ID
	ldrNodeHost               string              // IP address of connected leader node
	ldrNodeConn               net.Conn            // Connection of leader node
	ldrConnRetryCnt           int                 // Number of times to try to connect to the leader node
	msgClIn                   chan ClMessage      // Channel to synchronize incoming message from peer node
	usrAccntNum               int                 // Account number entered by the user
	usrTransType              int                 // Transaction type entered by the user
	usrAmnt                   int                 // Amount entered by the user
	seqNum                    int                 // Client sequence number for client value
	getUsrInput               chan struct{}       // Channel to synchronize reading user input and receiving the transaction result
	readInput                 bool                // Synchronize reading user input and sending the transaction to leader node
	stop                      chan struct{}       // Channel for signalling a stop request to the start run loop
	benchmark_iteration_count int                 // benchmark_iteration_count
}

// Communication protocol between server and client
type commProto struct {
	MsgType     string                  // REQ_HB, RES_HB, PREPARE, PROMISE, ACCEPT, LEARN, EXTCLVALUE
	FromNodeID  int                     // Node ID of client
	ToNodeID    int                     // Node ID of server
	SlotID      int                     // Slot ID
	Rnd         int                     // Round number
	Val         mlpx.Value              // Client value to bank
	Slots       []mlpx.PromiseSlot      // Promise slot
	IPAddr      string                  // IP Address of leader
	TxnResponse mlpx.Response           // Transaction response from bank
	StateType   int                     // Server state type
	UsrAccnt    map[string]bank.Account // Map of external client addr and account details
	Request     bool                    // true -> request, false -> reply
}

// Client side data structure
type ClMessage struct {
	clCommProto commProto
	clConn      net.Conn
}

// Get localhost IPv4 address
func GetIPAddr() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		os.Stderr.WriteString("Oops: " + err.Error() + "\n")
		os.Exit(1)
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
