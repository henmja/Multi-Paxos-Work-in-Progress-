package network

import (
	"container/list"

	dtr "dat520.github.io/lab3/detector"
	bank "dat520.github.io/lab6/bank"
	mlpx "dat520.github.io/lab6/multipaxos"

	"net"
	"os"
	"time"
)

// Definitions - DO NOT EDIT

var NATIVE_CONN_ADDR string // This node's IP address

const (
	NW_CONN_PROTO = "tcp"       // Network connection protocol
	TCP_CONN_PORT = "29170"     // Network connection protocol port
	DELTA         = time.Second // delta for timeout
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

// Server data
type DistributedSrv struct {
	id            int                     // This node ID
	srvListener   *net.TCPListener        // Server listener handle
	Fd            *dtr.EvtFailureDetector // Instance of eventually perfect failure detector
	msgSrvIn      chan ClMessage          // Channel to synchronize incoming message from client to server
	msgSrvOut     chan ClMessage          // Channel to synchronize outgoing message from server to client
	clNodeIDConn  map[int]net.Conn        // Dictionary of client nodeID and conns
	extClAddrConn map[string]net.Conn     // Dictionary of external client address and conns
	Proposer      *mlpx.Proposer          // Instance of proposer
	Acceptor      *mlpx.Acceptor          // Instance of acceptor
	Learner       *mlpx.Learner           // Instance of learner
	sendClVal     bool                    // Flag to deliver client value synchronously to Proposer
	curLeader     int                     // Current leader value
	adu           int                     // Current slot ID
	srvCh         SrvChannels             // All the channels external to network layer
	usrAccnt      map[int]bank.Account    // Map of external client account number and account details
	clValInList   *list.List              // List of client value received from external client
	dcdValInList  *list.List              // List of decided value received from Learner that are awaiting to be processed by bank
	stop          chan struct{}           // Channel for signalling a stop request to the start run loop
}

type SrvChannels struct {
	HbOut        chan dtr.Heartbeat     // Channel for receiving outgoing heartbeat messages
	PrepareOut   chan mlpx.Prepare      // Channel to recv prepare message
	AcceptOut    chan mlpx.Accept       // Channel to recv accept message
	PromiseOut   chan mlpx.Promise      // Channel to recv promise message
	LearnOut     chan mlpx.Learn        // Channel to recv learn value message
	ChosenValOut chan mlpx.DecidedValue // Channel to recv chosen value from learner
	SrvLdrIn     chan int               // Get leader subscription via app layer
}

// Client data
type DistributedCl struct {
	id                int                 // This node ID
	serverIPAddrConns map[string]net.Conn // Dictionary of node and it's conn handle
	msgClIn           chan ClMessage      // Channel to synchronize incoming message from server
	msgClOut          chan ClMessage      // Channel to synchronize outgoing message to server
	stop              chan struct{}       // Channel for signalling a stop request to the start run loop
}

// Communication protocol between server and client
type commProto struct {
	MsgType     string             // REQ_HB, RES_HB, PREPARE, PROMISE, ACCEPT, LEARN, EXTCLVALUE
	FromNodeID  int                // Node ID of client
	ToNodeID    int                // Node ID of server
	SlotID      int                // Slot ID
	Rnd         int                // Round number
	Val         mlpx.Value         // Client value to bank
	Slots       []mlpx.PromiseSlot // Promise slot
	IPAddr      string             // IP Address of leader
	TxnResponse mlpx.Response      // Transaction response from bank
	StateType   int                // Server state type
	Request     bool               // true -> request, false -> reply
}

// Server side data structure
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
