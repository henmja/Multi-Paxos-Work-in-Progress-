package network

import (
	"container/list"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	dtr "dat520.github.io/lab3/detector"
	bank "dat520.github.io/lab6/bank"
	mlpx "dat520.github.io/lab6/multipaxos"
)

/************************** Server section starts **************************/

// Init instance of a server
func InitServer(ld *dtr.MonLeaderDetector, msgIn chan ClMessage, msgOut chan ClMessage,
	proposer *mlpx.Proposer, acceptor *mlpx.Acceptor, learner *mlpx.Learner, srvChannel SrvChannels) (*DistributedSrv, error) {

	log.SetFlags(log.Ldate | log.Ltime)

	NATIVE_CONN_ADDR = GetIPAddr()

	hbOut := make(chan dtr.Heartbeat, 16)
	fd := dtr.NewEvtFailureDetector(MapOfIPAddrNodeIDs[NATIVE_CONN_ADDR], ClusterOfThree, ld, DELTA, hbOut)
	srvChannel.HbOut = hbOut

	srvResAddr, err := net.ResolveTCPAddr(NW_CONN_PROTO, NATIVE_CONN_ADDR+":"+TCP_CONN_PORT)
	if err != nil {
		return nil, err
	}

	srvListener, err_lis := net.ListenTCP(NW_CONN_PROTO, srvResAddr)
	if err_lis != nil {
		return nil, err_lis
	}

	srvHandle := DistributedSrv{
		id:            MapOfIPAddrNodeIDs[NATIVE_CONN_ADDR],
		srvListener:   srvListener,
		Fd:            fd,
		msgSrvIn:      msgIn,
		msgSrvOut:     msgOut,
		clNodeIDConn:  make(map[int]net.Conn),
		extClAddrConn: make(map[string]net.Conn),
		Proposer:      proposer,
		Acceptor:      acceptor,
		Learner:       learner,
		sendClVal:     true,
		curLeader:     MapOfIPAddrNodeIDs[NATIVE_CONN_ADDR],
		adu:           0,
		srvCh:         srvChannel,
		usrAccnt:      make(map[int]bank.Account),
		clValInList:   list.New(),
		dcdValInList:  list.New(),
		stop:          make(chan struct{}),
	}

	return &srvHandle, nil
}

// Main start point for server
func (srv *DistributedSrv) Start() {
	go srv.srvHandleNewClientConn()
	var clNewMsg ClMessage

	go func() {
		for {
			select {
			case clNewMsg = <-srv.msgSrvIn:
				go srv.srvHandlePeerInMsg(clNewMsg)
			case <-srv.stop:
				return
			}
		}
	}()
}

// Stop stops a's main run loop.
func (srv *DistributedSrv) Stop() {
	srv.stop <- struct{}{}
}

// Handle new client connection
func (srv *DistributedSrv) srvHandleNewClientConn() {
	fmt.Println("srvHandleNewClientConn")
	go srv.srvHandleThisNodeOutMsg()

	for {
		clConn, err := srv.srvListener.Accept()
		if err != nil {
			clConn = nil
		} else {
			clIPAddr := strings.Split(clConn.RemoteAddr().String(), ":")
			fmt.Printf("CLIENT IP = ", clIPAddr[0])

			if _, ok := MapOfIPAddrNodeIDs[clIPAddr[0]]; !ok {
				srv.extClAddrConn[clIPAddr[0]] = clConn
				go srv.srvRecvExtClMsg(clIPAddr[0], clConn)
			} else {
				srv.clNodeIDConn[MapOfIPAddrNodeIDs[clIPAddr[0]]] = clConn
				go srv.srvRecvPeerMsg(clIPAddr[0], clConn)
			}
		}
	}
}

// Notify the chosen value to the external client
func (srv *DistributedSrv) srvSendTxnResponse(txnResponse mlpx.Response) {
	fmt.Println("srvSendTxnResponse")
	extClConn := srv.extClAddrConn[txnResponse.ClientID]
	if extClConn != nil {

		fmt.Printf("\n\tSuccess:\n\t\tAccount num =  %d\n\t\tBalance = %d\n\n", txnResponse.TxnRes.AccountNum, txnResponse.TxnRes.Balance)

		extClData := commProto{MsgType: "TRANSACTION_RESULT", FromNodeID: srv.id, TxnResponse: txnResponse}
		clWriteGob := gob.NewEncoder(extClConn)
		err := clWriteGob.Encode(extClData)
		if err == io.EOF {
			extClConn.Close()
			srv.extClAddrConn[txnResponse.ClientID] = nil
			fmt.Printf("[NTW/Server]: srvSendTxnResponse() [%v] Host [%s]\n", err.Error(), txnResponse.ClientID)
		}
	}
}

// Receive message from peer client
func (srv *DistributedSrv) srvRecvPeerMsg(peerClAddr string, peerClConn net.Conn) {
	fmt.Println("srvRecvPeerMsg")
	for {
		if peerClConn != nil {
			clData := new(commProto)
			clReadGob := gob.NewDecoder(peerClConn)
			err := clReadGob.Decode(&clData)
			if err == io.EOF {
				//log.Printf("[NTW/Server]: srvRecvPeerMsg() [%v] Host [%s]\n", err.Error(), peerClAddr)
				peerClConn.Close()
				srv.clNodeIDConn[MapOfIPAddrNodeIDs[peerClAddr]] = nil
				return
			} else if err == nil {
				if len(clData.MsgType) > 0 {
					srv.msgSrvIn <- ClMessage{clCommProto: *clData, clConn: nil}
				}
			}
		}
	}
}

// Receive message from external client
func (srv *DistributedSrv) srvRecvExtClMsg(peerClAddr string, peerClConn net.Conn) {
	fmt.Println("srvRecvExtClMsg")
	for {
		if peerClConn != nil {
			clData := new(commProto)
			clReadGob := gob.NewDecoder(peerClConn)
			err := clReadGob.Decode(&clData)
			if err == io.EOF {
				peerClConn.Close()
				srv.extClAddrConn[peerClAddr] = nil
				//log.Printf("[NTW/Server]: srvRecvExtClMsg() [%v] Host [%s]\n", err.Error(), peerClAddr)
				return
			} else if err == nil {
				if srv.curLeader != srv.id {
					extClData := commProto{MsgType: "REDIRECT", FromNodeID: srv.id}
					extClData.IPAddr = mapOfNodeIDsIPAddr[srv.curLeader]
					clWriteGob := gob.NewEncoder(peerClConn)
					err := clWriteGob.Encode(extClData)
					if err == io.EOF || err == nil {
						time.Sleep(5 * time.Millisecond)
						peerClConn.Close()
						srv.extClAddrConn[peerClAddr] = nil
						return
					}
				} else {
					if len(clData.MsgType) > 0 {
						srv.msgSrvIn <- ClMessage{clCommProto: *clData, clConn: nil}
					}
				}
			}
		}
	}
}

// Handle decided value
func (srv *DistributedSrv) srvHandleDecidedValue(dcdVal mlpx.DecidedValue) {
	fmt.Println("srvHandleDecidedValue")
	if int(dcdVal.SlotID) > (srv.adu + 1) { // Works with alpha > 1
		srv.dcdValInList.PushBack(dcdVal) // TODO: Lab 6
		return
	}

	if !dcdVal.Value.Noop {
		usrAccnt, ok := srv.usrAccnt[dcdVal.Value.AccountNum]
		if !ok { // If the account doesn't exist, then create a new account with the given account number
			usrAccnt = bank.Account{Number: dcdVal.Value.AccountNum, Balance: 0}
			srv.usrAccnt[dcdVal.Value.AccountNum] = usrAccnt
		}

		txnResult := usrAccnt.Process(dcdVal.Value.Txn) // Send transaction to bank for processing
		srv.usrAccnt[dcdVal.Value.AccountNum] = usrAccnt

		txnResponse := mlpx.Response{dcdVal.Value.ClientID, dcdVal.Value.ClientSeq, txnResult}
		fmt.Printf("SENDING TRNSACTION RESULT TO CLIENT")
		if srv.curLeader == srv.id {
			fmt.Printf("SENDING TRNSACTION RESPONSE TO CLIENT")
			fmt.Printf("\n\tSuccess:\n\t\tAccount num =  %d\n\t\tBalance = %d\n\t\t CLient id = %d\n\n\n", txnResponse.TxnRes.AccountNum, txnResponse.TxnRes.Balance, txnResponse.ClientID)
			srv.srvSendTxnResponse(txnResponse) // Send transaction response to external client
		}
	}

	srv.adu++                              // Increment server adu
	srv.Proposer.IncrementAllDecidedUpTo() // Increment Proposer adu

	//Process any pending transaction
	if srv.dcdValInList.Len() > 0 {
		newDcdVal := srv.dcdValInList.Front().Value.(mlpx.DecidedValue)
		srv.dcdValInList.Remove(srv.dcdValInList.Front())
		srv.srvHandleDecidedValue(newDcdVal)
	}
}

// Process peer node incoming message
func (srv *DistributedSrv) srvHandlePeerInMsg(clNewMsg ClMessage) {
	//fmt.Println("srvHandlePeerInMsg")
	switch clNewMsg.clCommProto.MsgType {
	case "REQ_HB": // Deliver HB request from peer to this node
		hbReq := dtr.Heartbeat{From: clNewMsg.clCommProto.FromNodeID, To: clNewMsg.clCommProto.ToNodeID, Request: true}
		srv.Fd.DeliverHeartbeat(hbReq)
	case "RES_HB": // Deliver HB reply from peer to this node
		hbReply := dtr.Heartbeat{From: clNewMsg.clCommProto.FromNodeID, To: clNewMsg.clCommProto.ToNodeID, Request: false}
		srv.Fd.DeliverHeartbeat(hbReply)
	case "PREPARE": // Deliver prepare message from peer proposer to this node acceptor
		prepareIn := mlpx.Prepare{From: clNewMsg.clCommProto.FromNodeID, Slot: mlpx.SlotID(clNewMsg.clCommProto.SlotID), Crnd: mlpx.Round(clNewMsg.clCommProto.Rnd)}
		srv.Acceptor.DeliverPrepare(prepareIn)
	case "PROMISE": // Deliver promise message from peer acceptor to this node proposer
		promiseIn := mlpx.Promise{
			To:    clNewMsg.clCommProto.ToNodeID,
			From:  clNewMsg.clCommProto.FromNodeID,
			Rnd:   mlpx.Round(clNewMsg.clCommProto.Rnd),
			Slots: clNewMsg.clCommProto.Slots,
		}
		srv.Proposer.DeliverPromise(promiseIn)
	case "ACCEPT": // Deliver accept message from peer proposer to this node acceptor
		acceptIn := mlpx.Accept{From: clNewMsg.clCommProto.FromNodeID, Slot: mlpx.SlotID(clNewMsg.clCommProto.SlotID), Rnd: mlpx.Round(clNewMsg.clCommProto.Rnd), Val: mlpx.Value(clNewMsg.clCommProto.Val)}
		srv.Acceptor.DeliverAccept(acceptIn)
	case "LEARN": // Deliver learn message from peer acceptor to this node learner
		learnIn := mlpx.Learn{From: clNewMsg.clCommProto.FromNodeID, Slot: mlpx.SlotID(clNewMsg.clCommProto.SlotID), Rnd: mlpx.Round(clNewMsg.clCommProto.Rnd), Val: mlpx.Value(clNewMsg.clCommProto.Val)}
		srv.Learner.DeliverLearn(learnIn)
	case "EXTCLVALUE": // Deliver external client value to Proposer
		srv.clValInList.PushBack(clNewMsg.clCommProto.Val) // Cache the client value to server state
		srv.Proposer.DeliverClientValue(mlpx.Value(clNewMsg.clCommProto.Val))
	}
}

// Process outgoing message generated on this node
func (srv *DistributedSrv) srvHandleThisNodeOutMsg() {
	fmt.Println("srvHandleThisNodeOutMsg")
	var gotLeader int

	for {
		select {
		case hbFD := <-srv.srvCh.HbOut:
			if hbFD.Request { // If HB request to this node
				if hbFD.From == hbFD.To {
					hbReply := dtr.Heartbeat{From: hbFD.From, To: hbFD.To, Request: false}
					srv.Fd.DeliverHeartbeat(hbReply) // Deliver HB reply to this node
				} else {
					clData := commProto{MsgType: "REQ_HB", FromNodeID: hbFD.From, ToNodeID: hbFD.To}
					srv.msgSrvOut <- ClMessage{clCommProto: clData, clConn: nil} // Deliver HB request to peer node
				}
			} else { // HB response to peer node
				clData := commProto{MsgType: "RES_HB", FromNodeID: hbFD.From, ToNodeID: hbFD.To}
				if srv.clNodeIDConn[hbFD.To] != nil {
					clWriteGob := gob.NewEncoder(srv.clNodeIDConn[hbFD.To])
					err := clWriteGob.Encode(clData)
					if err == io.EOF {
						log.Printf("[NTW/Server]: srvHandleThisNodeOutMsg() [%v] NodeID [%d]\n", err.Error(), hbFD.To)
						srv.clNodeIDConn[hbFD.To].Close()
						srv.clNodeIDConn[hbFD.To] = nil
					}
				}
			}
		case prepareOut := <-srv.srvCh.PrepareOut:
			srv.Acceptor.DeliverPrepare(prepareOut) // Deliver prepare to this node acceptor
			clData := commProto{MsgType: "PREPARE", FromNodeID: prepareOut.From, SlotID: int(prepareOut.Slot), Rnd: int(prepareOut.Crnd)}
			srv.msgSrvOut <- ClMessage{clCommProto: clData, clConn: nil} // Deliver prepare to peer node acceptor
		case promiseOut := <-srv.srvCh.PromiseOut:
			if srv.id == promiseOut.To {
				srv.Proposer.DeliverPromise(promiseOut) // Deliver promise to this node proposer
			} else {
				clData := commProto{MsgType: "PROMISE", FromNodeID: promiseOut.From, ToNodeID: promiseOut.To, Rnd: int(promiseOut.Rnd), Slots: promiseOut.Slots}
				srv.msgSrvOut <- ClMessage{clCommProto: clData, clConn: nil} // Deliver promise to peer node proposer
			}
		case acceptOut := <-srv.srvCh.AcceptOut:
			srv.Acceptor.DeliverAccept(acceptOut) // Deliver accept to this node acceptor
			clData := commProto{MsgType: "ACCEPT", FromNodeID: acceptOut.From, SlotID: int(acceptOut.Slot), Rnd: int(acceptOut.Rnd), Val: acceptOut.Val}
			srv.msgSrvOut <- ClMessage{clCommProto: clData, clConn: nil} // Deliver accept to peer node acceptor
		case learnOut := <-srv.srvCh.LearnOut:
			srv.Learner.DeliverLearn(learnOut) // Deliver learn message from acceptor to this node learner
			clData := commProto{MsgType: "LEARN", FromNodeID: learnOut.From, SlotID: int(learnOut.Slot), Rnd: int(learnOut.Rnd), Val: learnOut.Val}
			srv.msgSrvOut <- ClMessage{clCommProto: clData, clConn: nil} // Deliver learn to peer node learner
		case dcdVal := <-srv.srvCh.ChosenValOut: // Receive decided value from Learner
			srv.srvHandleDecidedValue(dcdVal)
		case gotLeader = <-srv.srvCh.SrvLdrIn: // Receive leader information from Proposer and update to server
			srv.curLeader = gotLeader
		}
	}
}

/************************** Server section ends **************************/
