package extclient

import (
	"encoding/gob"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"time"

	bank "dat520.github.io/lab6/bank"
	mlpx "dat520.github.io/lab6/multipaxos"
)

/************************* Client section starts *************************/

//Init instance of a client
func InitExtClient() (*DistributedExtCl, error) {
	NATIVE_CONN_ADDR = GetIPAddr()

	extClHandle := DistributedExtCl{
		msgClIn:           make(chan ClMessage, 1000),
		serverIPAddrConns: clusterAddrConns,
		getUsrInput:       make(chan struct{}),
		stop:              make(chan struct{}),
	}

	return &extClHandle, nil
}

// Main start point for external client module
func (c *DistributedExtCl) Start() {
	go c.clHandleOutgoingMsg()
	c.getUsrInput <- struct{}{}
	//go c.clHandleIncomingMsg() // Handle init incoming message

	go func() {
		for {
			select {
			case msgIn := <-c.msgClIn:
				if c.seqNum == msgIn.clCommProto.TxnResponse.ClientSeq {
					txnResponse := msgIn.clCommProto.TxnResponse
					if txnResponse.TxnRes.ErrorString == "" {
						fmt.Printf("\n\tSuccess:\n\t\tAccount num =  %d\n\t\tBalance = %d\n\n",
							txnResponse.TxnRes.AccountNum, txnResponse.TxnRes.Balance)
					} else {
						fmt.Printf("\n\tFailure:\n\t\t[%s]\n\n", txnResponse.TxnRes.ErrorString)
					}
				} else {
					fmt.Printf("\nSequence number problem:\n\tClient Seq Num  = %d\n\tBut obtained Sequence number = %d\n\n",
						c.seqNum, msgIn.clCommProto.Val.ClientSeq)
				}
				c.seqNum++
				c.getUsrInput <- struct{}{}
			case <-c.stop:
				os.Exit(1)
			}
		}
	}()
}

// Stop stops a's main run loop.
func (c *DistributedExtCl) Stop() {
	c.stop <- struct{}{}
}

// Alive forever Go routine to connect to the predefined list of nodes
func (c *DistributedExtCl) clConnToServer(spawnHndInMsg bool) {
	c.ldrConnRetryCnt = 0
	for c.ldrNodeConn == nil && c.ldrConnRetryCnt <= LDR_CONN_NUM_RETRY {
		if c.ldrNodeHost == "" { // Doesn't know about leader info and initial connection
			c.randNodeID = c.genRandNum(0, 3)
			c.ldrNodeHost = mapOfNodeIDsIPAddr[c.randNodeID]
		} else { // Knows the leader node after redirection
			c.randNodeID = MapOfIPAddrNodeIDs[c.ldrNodeHost]
		}

		peerSrvConn, err := net.DialTimeout(NW_CONN_PROTO, c.ldrNodeHost+":"+TCP_CONN_PORT, 3*time.Second)
		if err != nil {
			c.ldrNodeHost = ""
			c.ldrConnRetryCnt++
			continue
		} else {
			err = peerSrvConn.(*net.TCPConn).SetKeepAlive(true)
			if err != nil {
			} else {
				err = peerSrvConn.(*net.TCPConn).SetKeepAlivePeriod(1800 * time.Second)
				if err != nil {
				}
			}

			if spawnHndInMsg {
				go c.clHandleIncomingMsg() // Handle incoming message
			}

			c.ldrNodeConn = peerSrvConn
		}
	}

	if c.ldrConnRetryCnt >= LDR_CONN_NUM_RETRY {
		fmt.Println()
		fmt.Printf("[CML/Client]: The bank system isn't available now.. Please try again later...\n")
		fmt.Printf("[CML/Client]: Sorry for the inconvenience this may have caused you...\n")
		c.stop <- struct{}{}
	}
}

// Send message to peer node server
func (c *DistributedExtCl) clSendDataToServer() {
	if c.ldrNodeConn != nil {
		usrTxn := bank.Transaction{Op: bank.Operation(c.usrTransType), Amount: c.usrAmnt}
		clVal := mlpx.Value{ClientID: NATIVE_CONN_ADDR, ClientSeq: c.seqNum, Noop: false, AccountNum: c.usrAccntNum, Txn: usrTxn}

		dataToPeerSrv := commProto{MsgType: "EXTCLVALUE", Val: clVal}
		srvWriteGob := gob.NewEncoder(c.ldrNodeConn)
		err := srvWriteGob.Encode(dataToPeerSrv)
		if err == io.EOF {
			c.ldrNodeConn.Close()
			c.ldrNodeConn = nil
			c.ldrNodeHost = ""

			go c.clConnToServer(false) // Connect to the leader node
		} else if err == nil {
			fmt.Println("\nTransaction started ...")
			fmt.Printf("\nProccessing in  %s  :  %d...\n", c.ldrNodeHost, c.randNodeID)
		}
	}
}

// Handle outgoing message from this client module
func (c *DistributedExtCl) clHandleOutgoingMsg() {
	for {
		select {
		case <-c.getUsrInput:
			c.usrAccntNum, c.usrTransType, c.usrAmnt = 0, 0, 0

			fmt.Println("|||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||")

			fmt.Println("\n\nNew Bank transaction:")

			fmt.Print("\n\n\tAccount number: ")
			fmt.Scan(&c.usrAccntNum)
			for c.usrAccntNum < 0 {
				fmt.Println("\n\tNegative  values not allowed")
				fmt.Print("\n\tAccount number: ")
				fmt.Scan(&c.usrAccntNum)
			}

			fmt.Print("\n\tChoose [0: Balance, 1: Deposit, 2: Withdrawal] :   ")
			fmt.Scan(&c.usrTransType)
			for c.usrTransType < 0 || c.usrTransType > 2 {
				fmt.Println("\n\tInvalid choice")
				fmt.Print("\n\tChoose [0: Balance, 1: Deposit, 2: Withdrawal] :   ")
				fmt.Scan(&c.usrTransType)
			}

			c.usrAmnt = 0
			if c.usrTransType != 0 {
				fmt.Print("\n\tAmount: ")
				fmt.Scan(&c.usrAmnt)
				for c.usrAmnt < 0 {
					fmt.Println("\n\tNegative  values not allowed")
					fmt.Print("\n\tAmount: ")
					fmt.Scan(&c.usrAmnt)
				}
				fmt.Printf("\n\tDetails:\n\t\tAccount number = %d\n\t\tTransaction type =  %d\n\t\tAmount = %d \n",
					c.usrAccntNum, c.usrTransType, c.usrAmnt)
			} else {
				fmt.Printf("\n\tDetails:\n\t\tAccount number =%d\n\t\tTransaction type =%d\n",
					c.usrAccntNum, c.usrTransType)
			}

			c.readInput = true

			if c.ldrNodeConn == nil {
				c.clConnToServer(true) // Connect to the leader node
			}

			go c.clSendDataToServer()
			c.readInput = false
		}
	}
}

func (c *DistributedExtCl) clHandleRedirect() {
	c.clConnToServer(true)    // Connect to the leader node
	go c.clSendDataToServer() // Send transaction to leader node
}

// Handle incoming message from the peer node
func (c *DistributedExtCl) clHandleIncomingMsg() {
	for {
		if c.ldrNodeConn != nil {
			var srvData commProto
			srvReadGob := gob.NewDecoder(c.ldrNodeConn)
			err := srvReadGob.Decode(&srvData)
			if err == io.EOF {
				c.ldrNodeConn.Close()
				c.ldrNodeConn = nil
				c.ldrNodeHost = ""

				time.Sleep(8 * time.Second) // Wait until the non-leader nodes detect the right leader
				go c.clConnToServer(true)   // Connect to the leader node
				return
			} else if err == nil {
				if len(srvData.MsgType) > 0 {
					switch srvData.MsgType {
					case "REDIRECT":
						c.ldrNodeConn.Close()
						c.ldrNodeConn = nil
						c.ldrNodeHost = srvData.IPAddr

						c.clHandleRedirect()
						return
					case "TRANSACTION_RESULT":
						msgFromPeer := ClMessage{clCommProto: srvData, clConn: nil}
						c.msgClIn <- msgFromPeer
					}
				}
			}
		}
	}
}

// Generate random number between the provided range of numbers
func (c *DistributedExtCl) genRandNum(min, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return rand.Intn(max-min) + min
}

/************************* Client section ends *************************/
