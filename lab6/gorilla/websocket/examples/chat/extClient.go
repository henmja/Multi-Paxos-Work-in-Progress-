package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	bank "dat520.github.io/lab6/bank"
	mlpx "dat520.github.io/lab6/multipaxos"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub
	// The websocket connection.
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
//func (c *Client, dc *DistributedExtCl) readPump() {
func (c *DistributedExtCl) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		fmt.Println("readpump before byting = ", message)
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		result := strings.Split(string(message), ",")

		// Display all elements.
		for i := range result {
			fmt.Println(result[i])
		}
		// Length is 3.
		fmt.Println(len(result))

		fmt.Println("Reaadpump message = ", message)
		c.hub.broadcast <- message

		//	select {
		//	case <-c.getUsrInput:
		c.usrAccntNum, c.usrTransType, c.usrAmnt = 0, 0, 0

		c.usrAccntNum, err = strconv.Atoi(result[0])
		c.usrTransType, err = strconv.Atoi(result[1])

		c.usrAmnt = 0

		if c.usrTransType != 0 {
			c.usrAmnt, err = strconv.Atoi(result[2])
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
		fmt.Println("CALLING SEND DATA TO SERVER FROM READPUMP")
		c.readInput = false
		//	}

	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
//func (c *Client) writePump() {
func (c *DistributedExtCl) writePump() {
	ticker := time.NewTicker(pingPeriod)
	var bup_message []byte
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:

			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			fmt.Println("Writepump message = ", message, ok)
			bup_message = message
			w.Write(message)

			//Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case msgIn := <-c.msgClIn:
			if c.seqNum == msgIn.clCommProto.TxnResponse.ClientSeq {
				txnResponse := msgIn.clCommProto.TxnResponse
				msg_type := "BANK_RESULT"
				if txnResponse.TxnRes.ErrorString == "" {
					fmt.Printf("\n\tWRITEPUMPPPPPPP  Success:\n\t\tAccount num =  %d\n\t\tBalance = %d\n\n",
						txnResponse.TxnRes.AccountNum, txnResponse.TxnRes.Balance)

					result := strings.Split(string(bup_message), ",")
					/*
						// Display all elements.
							for i := range result {
								fmt.Println(result[i])
							}
						// Length is 3.
						fmt.Println(len(result))
					*/

					//final_message := fmt.Sprintf("%d,%s,%s,%d",txnResponse.TxnRes.AccountNum,string(bup_message[2]),string(bup_message[4]),txnResponse.TxnRes.Balance )
					final_message := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s", msg_type, strconv.Itoa(txnResponse.TxnRes.AccountNum), result[1], result[2], strconv.Itoa(txnResponse.TxnRes.Balance), c.ldrNodeHost, strconv.Itoa(c.randNodeID))
					final_message_byte := []byte(final_message)
					c.conn.SetWriteDeadline(time.Now().Add(writeWait))

					w, err := c.conn.NextWriter(websocket.TextMessage)
					if err != nil {
						return
					}
					fmt.Println("Writepump final message byte = ", final_message_byte)
					fmt.Println("Writepump message byte = ", bup_message)
					w.Write(final_message_byte)
					if err := w.Close(); err != nil {
						return
					}

				} else {
					fmt.Printf("\n\tFailure:\n\t\t[%s]\n\n", txnResponse.TxnRes.ErrorString)
					fail_message := fmt.Sprintf("%s,%s,%s,%s", "FAILED", txnResponse.TxnRes.ErrorString, c.ldrNodeHost, strconv.Itoa(c.randNodeID))
					fail_message_byte := []byte(fail_message)
					c.conn.SetWriteDeadline(time.Now().Add(writeWait))

					w, err := c.conn.NextWriter(websocket.TextMessage)
					if err != nil {
						return
					}
					fmt.Println("Writepump fail message byte = ", fail_message_byte)
					fmt.Println("Writepump message byte = ", bup_message)
					w.Write(fail_message_byte)
					if err := w.Close(); err != nil {
						return
					}
				}
			} else {
				fmt.Printf("\nSequence number problem:\n\tClient Seq Num  = %d\n\tBut obtained Sequence number = %d\n\n",
					c.seqNum, msgIn.clCommProto.Val.ClientSeq)

				error_string := fmt.Sprintf("%s%s%s%s", "Sequence number problem: Current Client Seq Num  = ",
					strconv.Itoa(c.seqNum), " But got Sequence number = ", strconv.Itoa(msgIn.clCommProto.Val.ClientSeq))

				fail_message2 := fmt.Sprintf("%s,%s,%s,%s", "FAILED", error_string, c.ldrNodeHost, strconv.Itoa(c.randNodeID))
				fail_message2_byte := []byte(fail_message2)
				c.conn.SetWriteDeadline(time.Now().Add(writeWait))

				w, err := c.conn.NextWriter(websocket.TextMessage)
				if err != nil {
					return
				}
				fmt.Println("Writepump fail message2 byte = ", fail_message2_byte)
				fmt.Println("Writepump message byte = ", bup_message)
				w.Write(fail_message2_byte)
				if err := w.Close(); err != nil {
					return
				}
			}
			c.seqNum++

		case <-c.stop:
			os.Exit(1)

		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	NATIVE_CONN_ADDR = GetIPAddr()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &DistributedExtCl{
		hub:               hub,
		conn:              conn,
		send:              make(chan []byte, 256),
		msgClIn:           make(chan ClMessage, 1000),
		serverIPAddrConns: clusterAddrConns,
		getUsrInput:       make(chan struct{}),
		stop:              make(chan struct{}),
	}

	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	//client.getUsrInput <- struct{}{}
	go client.writePump()
	go client.readPump()
}

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
	//go c.clHandleOutgoingMsg()
	//go c.readPump()
	//go c.writePump()
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
				fmt.Println("HANDLE INCOMING MESSAGE CALLED SPAWNHNDINMSG")
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
		fmt.Println("ClientID = ", NATIVE_CONN_ADDR)
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
			//go c.clHandleIncomingMsg() // Handle init incoming message
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

			fmt.Println("CALLING SEND DATA TO SERVER FROM HANDLE OUTGOING MESSAGE")
			go c.clSendDataToServer()
			c.readInput = false
		}
	}
}

func (c *DistributedExtCl) clHandleRedirect() {
	c.clConnToServer(true) // Connect to the leader node
	fmt.Println("CALLING SEND DATA TO SERVER FROM HANDLE REDIRECT")
	go c.clSendDataToServer() // Send transaction to leader node
}

// Handle incoming message from the peer node
func (c *DistributedExtCl) clHandleIncomingMsg() {
	for {
		//fmt.Println("HANDLE INCOMING MESSAGE FUNCTION FOR LOOP")
		if c.ldrNodeConn != nil {
			fmt.Println("LEADER CON NOT NULL")
			fmt.Println("c.ldrNodeConn = ", c.ldrNodeConn)
			var srvData commProto
			srvReadGob := gob.NewDecoder(c.ldrNodeConn)
			fmt.Println("AFTER DECODER OBJECT")
			err := srvReadGob.Decode(&srvData)
			fmt.Println("AFTER DECODING ", srvData.MsgType)
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
						fmt.Println("REDIRECT")
						c.ldrNodeConn.Close()
						c.ldrNodeConn = nil
						c.ldrNodeHost = srvData.IPAddr
						fmt.Print("c.ldrNodeHost = ", c.ldrNodeHost)
						c.clHandleRedirect()
						return
					case "TRANSACTION_RESULT":
						fmt.Println("TRANSACTION RESULT GOTTTTTTT")
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
