package network

import (
	"encoding/gob"
	"io"
	"log"
	"net"
	"time"
)

/************************* Client section starts *************************/

//Init instance of a client
func InitClient(msgIn chan ClMessage, msgOut chan ClMessage) (*DistributedCl, error) {
	log.SetFlags(log.Ldate | log.Ltime)

	NATIVE_CONN_ADDR = GetIPAddr()

	clHandle := DistributedCl{
		id:                MapOfIPAddrNodeIDs[NATIVE_CONN_ADDR],
		msgClIn:           msgIn,
		msgClOut:          msgOut,
		serverIPAddrConns: clusterAddrConns,
		stop:              make(chan struct{}),
	}

	return &clHandle, nil
}

// Main start point for client
func (c *DistributedCl) Start() {
	go c.connToServers()

	go func() {
		for {
			select {
			case msgClOut := <-c.msgClOut:
				c.clHandleOutgoingMsg(msgClOut)
			case <-c.stop:
				return
			}
		}
	}()
}

// Stop stops a's main run loop.
func (c *DistributedCl) Stop() {
	c.stop <- struct{}{}
}

// Alive forever Go routine to connect to the predefined list of nodes
func (c *DistributedCl) connToServers() {
	for {
		for peerSrvHost, conn := range c.serverIPAddrConns {
			if conn == nil {
				if peerSrvHost != NATIVE_CONN_ADDR {
					peerSrvConn, err := net.Dial(NW_CONN_PROTO, peerSrvHost+":"+TCP_CONN_PORT)
					if err != nil {
					} else {
						err = peerSrvConn.(*net.TCPConn).SetKeepAlive(true)
						if err != nil {
						} else {
							err = peerSrvConn.(*net.TCPConn).SetKeepAlivePeriod(1800 * time.Second)
							if err != nil {
							}
						}

						c.serverIPAddrConns[peerSrvHost] = peerSrvConn
						go c.clHandleIncomingMsg(MapOfIPAddrNodeIDs[peerSrvHost], peerSrvHost, peerSrvConn)
					}
				}
			}
		}
		time.Sleep(3000 * time.Millisecond)
	}
}

// Send message to peer node server
func (c *DistributedCl) clSendDataToServer(peerSrvHost string, peerSrvConn net.Conn, dataToPeerSrv commProto) {
	if peerSrvConn != nil {
		clData := new(commProto)
		clData = &dataToPeerSrv
		srvWriteGob := gob.NewEncoder(peerSrvConn)
		err := srvWriteGob.Encode(clData)
		if err == io.EOF {
			//log.Printf("[NTW/Client]: clSendDataToServer() [%v] Host [%s]\n", err.Error(), peerSrvHost)
			peerSrvConn.Close()
			c.serverIPAddrConns[peerSrvHost] = nil
		}
	}
}

// Handle outgoing message from this node server
func (c *DistributedCl) clHandleOutgoingMsg(msgClOut ClMessage) {
	outMsg := msgClOut.clCommProto
	switch outMsg.MsgType {
	case "REQ_HB": // Deliver HB request to peer node
		peerSrvHost := mapOfNodeIDsIPAddr[outMsg.ToNodeID]
		peerSrvConn := c.serverIPAddrConns[peerSrvHost]
		c.clSendDataToServer(peerSrvHost, peerSrvConn, outMsg)
	case "PREPARE": // Deliver prepare message to peer node acceptor
		for peerSrvHost, peerSrvConn := range c.serverIPAddrConns {
			outMsg.ToNodeID = MapOfIPAddrNodeIDs[peerSrvHost]
			c.clSendDataToServer(peerSrvHost, peerSrvConn, outMsg)
		}
	case "PROMISE": // Deliver promise message from acceptor to peer node proposer
		peerSrvHost := mapOfNodeIDsIPAddr[outMsg.ToNodeID]
		peerSrvConn := c.serverIPAddrConns[peerSrvHost]
		c.clSendDataToServer(peerSrvHost, peerSrvConn, outMsg)
	case "ACCEPT": // Deliver accept message from proposer to peer node acceptor
		for peerSrvHost, peerSrvConn := range c.serverIPAddrConns {
			outMsg.ToNodeID = MapOfIPAddrNodeIDs[peerSrvHost]
			c.clSendDataToServer(peerSrvHost, peerSrvConn, outMsg)
		}
	case "LEARN": // Deliver learn message from acceptor to peer node learner
		for peerSrvHost, peerSrvConn := range c.serverIPAddrConns {
			c.clSendDataToServer(peerSrvHost, peerSrvConn, outMsg)
		}
	}
}

// Handle incoming message from the peer node
func (c *DistributedCl) clHandleIncomingMsg(peerSrvNodeID int, peerSrvHost string, peerSrvConn net.Conn) {
	for {
		if peerSrvConn != nil {
			srvData := new(commProto)
			srvReadGob := gob.NewDecoder(peerSrvConn)
			err := srvReadGob.Decode(&srvData)
			if err == io.EOF {
				//log.Printf("[NTW/Client]: clHandleIncomingMsg() [%v] Host [%s]\n", err.Error(), peerSrvHost)
				peerSrvConn.Close()
				c.serverIPAddrConns[peerSrvHost] = nil
				return
			} else if err == nil {
				if len(srvData.MsgType) > 0 {
					c.msgClIn <- ClMessage{clCommProto: *srvData, clConn: nil}
				}
			}
		}
	}
}

/************************* Client section ends *************************/
