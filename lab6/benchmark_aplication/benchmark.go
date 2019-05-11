package ben

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
	//"log"
)

/************************* Client section starts *************************/

//Init instance of a client
func InitBenchClient() (*DistributedExtCl, error) {
	NATIVE_CONN_ADDR = GetIPAddr()

	extClHandle := DistributedExtCl{
		msgClIn:           make(chan ClMessage, 1000000),
		serverIPAddrConns: clusterAddrConns,
		getUsrInput:       make(chan struct{}),
		stop:              make(chan struct{}),
	}

	return &extClHandle, nil
}

// Main start point for external client module
func (c *DistributedExtCl) Start() {
	c.benchmark_iteration_count = 0
	go c.clHandleOutgoingMsg()
	c.getUsrInput <- struct{}{}
	//go c.clHandleIncomingMsg() // Handle init incoming message

	//var individual_elapsed [500] time.Duration
	//var individual_start [500] time.Time
	//var min_time time.Duration
	//var max_time time.Duration
	go func() {
		for {
			select {
			case msgIn := <-c.msgClIn:
				//i:=0
				//main_start := time.Now()
				//for i < 400 {
				//individual_start[i] = time.Now()
				c.getUsrInput <- struct{}{}

				if c.seqNum == msgIn.clCommProto.TxnResponse.ClientSeq {
					txnResponse := msgIn.clCommProto.TxnResponse
					if txnResponse.TxnRes.ErrorString == "" {
						fmt.Printf("[CML/Client]: Transaction successful: Account num [%d] Balance [%d]\n\n",
							txnResponse.TxnRes.AccountNum, txnResponse.TxnRes.Balance)
					} else {
						fmt.Printf("[CML/Client]: Transaction failed: [%s]\n\n", txnResponse.TxnRes.ErrorString)
					}
				} else {
					fmt.Printf("[CML/Client]: Cient sequence number mismatch: Client Seq Num [%d] Got seq num [%d]\n\n",
						c.seqNum, msgIn.clCommProto.Val.ClientSeq)
				}
				//individual_elapsed[i] = time.Since(individual_start[i])
				//if(i>0) {
				//	if(individual_elapsed[i] < individual_elapsed[i-1]) {
				//		min_time = individual_elapsed[i]
				//	} else {
				//		max_time = individual_elapsed[i-1]
				//	}

				//}
				//log.Printf("Individual %s", individual_elapsed[i])

				c.seqNum++
				//c.getUsrInput <- struct{}{}
				//i = i+1
				//fmt.Printf("COUNT = %d",i)
				//fmt.Println(" ");

				//}
				//main_elapsed := time.Since(main_start)
				//log.Printf("Overall %s", main_elapsed)
				//log.Printf("min per time %s", main_elapsed)
				//log.Printf("max per time %s", main_elapsed)

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
			fmt.Println("[CML/Client]: Started transaction... Awaiting for transaction to complete... Please wait...")
			fmt.Printf("[CML/Client]: Processing transaction on [%s:%d]...\n", c.ldrNodeHost, c.randNodeID)
		}
	}
}

// Handle outgoing message from this client module
func (c *DistributedExtCl) clHandleOutgoingMsg() {
	for {
		select {
		case <-c.getUsrInput:
			c.usrAccntNum, c.usrTransType, c.usrAmnt = 0, 0, 0

			i := 0
			if c.benchmark_iteration_count < 500 {
				fmt.Println("==============================================================================================")
				c.usrAccntNum, c.usrTransType, c.usrAmnt = 0, 0, 0

				c.usrAccntNum = c.genRandNum(1, 30)
				c.usrTransType = c.genRandNum(0, 2)
				c.usrAmnt = 0

				if c.usrTransType != 0 {
					c.usrAmnt = c.genRandNum(100, 100000)
					fmt.Printf("[CML/Client]: Transaction details provided: Account number [%d] Transaction type [%d] Amount [%d]\n", c.usrAccntNum, c.usrTransType, c.usrAmnt)
				} else {
					fmt.Printf("[CML/Client]: Transaction details provided: Account number [%d] Transaction type [%d]\n", c.usrAccntNum, c.usrTransType)
				}

				c.readInput = true

				if c.ldrNodeConn == nil {
					c.clConnToServer(true) // Connect to the leader node
				}

				go c.clSendDataToServer()
				c.benchmark_iteration_count = c.benchmark_iteration_count + 1
				c.readInput = false
				i = i + 1
				fmt.Printf("ITERATION COUNT = %d", c.benchmark_iteration_count)
				fmt.Println(" ")

			}

			/*fmt.Println("==============================================================================================")
			fmt.Println("[CML/Client]: Please enter the client value for the below respective items.")

			fmt.Print("[CML/Client]:                                           Account number: ")
			fmt.Scan(&c.usrAccntNum)
			for c.usrAccntNum < 0 {
				fmt.Println("[CML/Client]: OMG! Incorrect account number provided... Enter a meaningful account number...")
				fmt.Print("[CML/Client]:                                           Account number: ")
				fmt.Scan(&c.usrAccntNum)
			}

			fmt.Print("[CML/Client]: Transaction type [0: Balance, 1: Deposit, 2: Withdrawal]: ")
			fmt.Scan(&c.usrTransType)
			for c.usrTransType < 0 || c.usrTransType > 2 {
				fmt.Println("[CML/Client]: OMG! Incorrect transaction type provided... Enter the transaction type as shown below...")
				fmt.Print("[CML/Client]: Transaction type [0: Balance, 1: Deposit, 2: Withdrawal]: ")
				fmt.Scan(&c.usrTransType)
			}

			c.usrAmnt = 0
			if c.usrTransType != 0 {
				fmt.Print("[CML/Client]:                                                   Amount: ")
				fmt.Scan(&c.usrAmnt)
				for c.usrAmnt < 0 {
					fmt.Println("[CML/Client]: OMG! Incorrect amount provided... Enter a meaningful amount value...")
					fmt.Print("[CML/Client]:                                                   Amount: ")
					fmt.Scan(&c.usrAmnt)
				}
				fmt.Printf("[CML/Client]: Transaction details provided: Account number [%d] Transaction type [%d] Amount [%d]\n",
					c.usrAccntNum, c.usrTransType, c.usrAmnt)
			} else {
				fmt.Printf("[CML/Client]: Transaction details provided: Account number [%d] Transaction type [%d]\n",
					c.usrAccntNum, c.usrTransType)
			}

			c.readInput = true

			if c.ldrNodeConn == nil {
				c.clConnToServer(true) // Connect to the leader node
			}

			go c.clSendDataToServer()
			c.readInput = false*/
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
