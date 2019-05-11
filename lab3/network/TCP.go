package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	logger "github.com/uis-dat520-s2019/Paxosfans/lab3/logging"
	"github.com/uis-dat520-s2019/Paxosfans/lab4/singlepaxos"
)

type ConfigInstance struct {
	nodeIndex struct {
		Id            int
		Ip            string
		ServerPort    int
		ClientPortMin int
		ClientPortMax int
	}
	Keepalive          bool
	Timeout            int
	Transport_proto    string
	Reconnect_interval int
	Nodes              []struct {
		Id            int
		Ip            string
		ServerPort    int
		ClientPortMin int
		ClientPortMax int
	}
}

type Config struct {
	Localhost ConfigInstance
	Pitter    ConfigInstance
}

type Node struct {
	Id            int
	Addr          *net.TCPAddr
	ClientPortMin int
	ClientPortMax int
	UsedPorts     []int
}

type Network struct {
	NodeIndex        Node
	Nodes            []Node
	Connections      map[int]net.Conn
	Config           ConfigInstance
	SChan            chan Message
	RChan            chan Message
	UnmodRChan       chan Message
	ServerListener   *net.TCPListener
	Logger           logger.Logger
	AddClientConn    chan net.Conn
	RemoveClientConn chan net.Conn
}

type Message struct {
	To      int
	From    int
	Msg     string
	Type    string
	Request bool
	Value   singlepaxos.Value
	Promise singlepaxos.Promise
	Accept  singlepaxos.Accept
	Prepare singlepaxos.Prepare
	Learn   singlepaxos.Learn
}

var mutex = &sync.Mutex{}
var sendMutex = &sync.Mutex{}
var resMutex = &sync.Mutex{}

func InitNetwork(config ConfigInstance, nodeIndex int, mode string, log logger.Logger) (Network, error) {
	sChan := make(chan Message, 16) //send
	rChan := make(chan Message, 16) //rcv
	addClientConn := make(chan net.Conn, 16)
	removeClientConn := make(chan net.Conn, 16)
	unmodRChan := make(chan Message, 16)
	network := Network{
		Config:           config,
		SChan:            sChan,
		RChan:            rChan,
		UnmodRChan:       unmodRChan,
		AddClientConn:    addClientConn,
		RemoveClientConn: removeClientConn,
		Nodes:            []Node{},
		Connections:      map[int]net.Conn{},
		Logger:           log,
	}

	for _, node := range config.Nodes {
		address := node.Ip + ":" + strconv.Itoa(node.ServerPort)
		tcpAddr, err := net.ResolveTCPAddr(config.Transport_proto, address)
		if err != nil {

			log.Log("system", "debug", "#Network: Failed to resolve TCPAddr for address: "+address+", with error: "+err.Error())
			continue
		}
		if node.Id == nodeIndex {
			network.NodeIndex = Node{
				Id:            node.Id,
				Addr:          tcpAddr,
				ClientPortMin: node.ClientPortMin,
				ClientPortMax: node.ClientPortMax,
				UsedPorts:     []int{},
			}
		} else {
			network.Nodes = append(network.Nodes, Node{
				Id:            node.Id,
				Addr:          tcpAddr,
				ClientPortMin: node.ClientPortMin,
				ClientPortMax: node.ClientPortMax,
				UsedPorts:     []int{},
			})
		}
	}
	if network.NodeIndex.Id != nodeIndex {
		return Network{}, errors.New("Failed to set nodeIndex from configuration. Can not proceed with creation of Network")
	}

	return network, nil
}

func (n *Network) ConnectTCP() error {
	for _, node := range n.Nodes {
		err := n.ConnectToNode(node)
		if err != nil {
			n.Logger.Log("system", "debug", "#Network: Failed to connect to peer when establishing peer2peer network. Error: "+err.Error())
		}
	}
	err := n.InitServer()
	if err != nil {
		return err
	}

	go func() {
		for {
			message := <-n.SChan
			err := n.Send(message)
			if err != nil {
				n.Logger.Log("system", "debug", "#Network: Failed to send message. Message: "+fmt.Sprintf("%+v", message)+" - Error: "+err.Error())
			}
		}
	}()

	//try to reconnect after a node disconnected
	go func() {
		for {
			<-time.After(5 * time.Second)
			n.CleanAndAttemptReconnect()
		}
	}()

	return nil
}

func (n *Network) ConnectToNode(node Node) error {
	//check if connected to the other nodes
	if n.Connections[node.Id] != nil {
		return errors.New("Allready connected to node " + strconv.Itoa(node.Id))
	}
	useAddr, err := n.getClientAddr()
	if err != nil {
		n.RemovePort(useAddr)
		return err
	}
	tcpConn, err := net.DialTCP(n.Config.Transport_proto, useAddr, node.Addr)
	if err != nil {
		n.RemovePort(useAddr)
		return errors.New("Dial failed to node " + strconv.Itoa(node.Id) + " with error: " + err.Error())
	}
	n.Logger.Log("system", "debug", "#Network: Successfully connected to Node "+strconv.Itoa(node.Id))
	mutex.Lock()
	n.Connections[node.Id] = tcpConn
	mutex.Unlock()

	go n.ListenMsg(tcpConn, node.Id)

	return nil
}

func (n *Network) InitServer() error {
	listener, err := net.ListenTCP("tcp", n.NodeIndex.Addr)
	if err != nil {
		return errors.New("Failed to make listener with error: " + err.Error())
	}
	n.ServerListener = listener

	go func(listener *net.TCPListener) {
		defer listener.Close()

		for {
			n.Logger.Log("system", "debug", "#Network: Server running, waiting to accept new connections...")
			conn, err := listener.Accept()
			if err != nil {
				n.Logger.Log("system", "debug", "#Network: Server listener.Accept() error: "+err.Error())
				break
			}
			raddr := conn.RemoteAddr()
			nodeID, err := n.getNodeIdFromRemoteAddr(raddr)
			if err != nil {
				n.AddClientConn <- conn
				go n.ListenClient(conn)
				continue
			}
			n.Logger.Log("system", "debug", "#Network: Accepted connection from node "+strconv.Itoa(nodeID))
			mutex.Lock()
			n.Connections[nodeID] = conn
			mutex.Unlock()
			go n.ListenMsg(conn, nodeID)
		}
	}(listener)

	return nil
}

func (n *Network) ListenClient(tcpConn net.Conn) {
	defer func() {
		n.RemoveClientConn <- tcpConn
		tcpConn.Close()
	}()

	for {
		var buf [20480]byte
		message := new(Message)
		num, err := tcpConn.Read(buf[0:])
		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to decode message from tcpConn, exiting goroutine and cleaning up connection. The error from the json decoder: "+err.Error())
			n.Logger.Log("system", "err", "#Network: "+fmt.Sprintf("%+v", err))
			break
		}

		err = json.Unmarshal(buf[0:num], &message)
		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to UNMARSHAL message. Error: "+err.Error())
		}
		n.Logger.Log("system", "debug", "#Network: SUCCESS, the buf is: "+string(buf[0:]))

		n.RChan <- *message
	}
}

func (n *Network) ListenMsg(tcpConn net.Conn, nodeID int) {
	defer n.cleanupConn(tcpConn, nodeID)

	for {
		var buf [20480]byte
		message := new(Message)
		num, err := tcpConn.Read(buf[0:])

		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to decode message from tcpConn, exiting goroutine and cleaning up connection. The error from the json decoder: "+err.Error())
			n.Logger.Log("system", "err", "#Network: "+fmt.Sprintf("%+v", err))
			break
			if err == io.EOF {
				break
			}

		}
		err = json.Unmarshal(buf[0:num], &message)
		if err != nil {
			n.Logger.Log("system", "err", "#Network: Failed to UNMARSHAL message. Error: "+err.Error())
		}
		n.Logger.Log("system", "debug", "#Network: SUCCESS, the buf is: "+string(buf[0:]))
		//read channel for given network instance:
		n.RChan <- *message
	}
}

func (n *Network) Send(msg Message) error {
	sendMutex.Lock()
	defer sendMutex.Unlock()
	if msg.To == n.NodeIndex.Id {
		n.RChan <- msg
		return nil
	}
	receiverConn := n.Connections[msg.To]
	if receiverConn == nil {
		return errors.New("Connection to node: " + strconv.Itoa(msg.To) + " is not present in n.Connections. Aborting sending of message.")
	}

	jsonByteSlice, err := json.Marshal(msg)
	if err != nil {
		return errors.New("Failed to MARSHAL message to node: " + strconv.Itoa(msg.To) + ". Aborting sending of message. Error: " + err.Error())
	}

	length := len(jsonByteSlice)
	_, err = receiverConn.Write(jsonByteSlice[0:length])
	if err != nil {
		return errors.New("Failed to write/send message to node: " + strconv.Itoa(msg.To) + ". Aborting sending of message.")
	}

	n.Logger.Log("network", "debug", fmt.Sprintf("Sent message: %+v", msg))
	return nil
}

func (n *Network) Broadcast(msg Message) error {
	for _, node := range n.Nodes {
		msg.To = node.Id
		err := n.Send(msg)
		if err != nil {
			n.Logger.Log("system", "notice", fmt.Sprintf("#Network: %+v", err))
		}
	}
	return nil
}

func (n *Network) Multicast(nodeIds []int, message Message) error {
	for _, id := range nodeIds {
		message.To = id
		err := n.Send(message)
		if err != nil {
			n.Logger.Log("system", "notice", fmt.Sprintf("#Network: %+v", err))
		}
	}
	return nil
}
func (n *Network) BroadcastIDX(nodeIds []int, message Message) error {
	for _, id := range nodeIds {
		message.To = id
		err := n.Send(message)
		if err != nil {
			n.Logger.Log("system", "notice", fmt.Sprintf("#Network: %+v", err))
		}
	}
	return nil
}

func (n *Network) cleanupConn(tcpConn net.Conn, nodeID int) {
	networkString := tcpConn.LocalAddr().String()
	split := strings.Split(networkString, ":")
	localPort, _ := strconv.Atoi(split[len(split)-1])
	mutex.Lock()
	delete(n.Connections, nodeID)
	for i, usedPort := range n.NodeIndex.UsedPorts {
		if localPort == usedPort {
			n.NodeIndex.UsedPorts = append(n.NodeIndex.UsedPorts[:i], n.NodeIndex.UsedPorts[i+1:]...)
		}
	}
	mutex.Unlock()
	tcpConn.Close()
}

func (n *Network) getClientAddr() (*net.TCPAddr, error) {
	usePort := -1
	for i := n.NodeIndex.ClientPortMin; i <= n.NodeIndex.ClientPortMax; i++ {
		inUsedPorts := false
		for _, port := range n.NodeIndex.UsedPorts {
			if port == i {
				inUsedPorts = true
			}
		}
		if inUsedPorts == false {
			usePort = i
			mutex.Lock()
			n.NodeIndex.UsedPorts = append(n.NodeIndex.UsedPorts, i)
			mutex.Unlock()
			break
		}
	}

	if usePort > -1 {
		useAddr, err := net.ResolveTCPAddr("tcp", n.NodeIndex.Addr.IP.String()+":"+strconv.Itoa(usePort))
		if err != nil {
			return nil, err
		}
		return useAddr, nil
	}
	return nil, errors.New("No available client port")
}

func (n *Network) getNodeIdFromRemoteAddr(raddr net.Addr) (int, error) {
	split := strings.Split(raddr.String(), ":")
	remotePort, err := strconv.Atoi(split[len(split)-1])
	if err != nil {
		return -1, errors.New("Failed to identify nodeID from raddr")
	}

	for _, node := range n.Nodes {
		if remotePort >= node.ClientPortMin && remotePort <= node.ClientPortMax {
			return node.Id, nil
		}
	}

	return -1, errors.New("Failed to get nodeId from rAddr port")
}

func (n *Network) CleanAndAttemptReconnect() {
	n.Clean()
	for _, node := range n.Nodes {

		if n.Connections[node.Id] != nil {
			continue
		}

		err := n.ConnectToNode(node)
		if err != nil {
			n.Logger.Log("system", "debug", "Reconnect failed to node "+strconv.Itoa(node.Id)+" with error: "+err.Error())
		}
	}
}

func (n *Network) Clean() {
	for key, conn := range n.Connections {
		if ConnTerminated(conn) == true {
			n.RemovePort(conn.LocalAddr())
			conn.Close()
			mutex.Lock()
			delete(n.Connections, key)
			mutex.Unlock()
		}
	}
}

func (n *Network) RemovePort(addr net.Addr) error {
	networkString := addr.String()
	split := strings.Split(networkString, ":")
	portToRemove, _ := strconv.Atoi(split[len(split)-1])
	for i, usedPort := range n.NodeIndex.UsedPorts {
		if portToRemove == usedPort {
			mutex.Lock()
			n.NodeIndex.UsedPorts = append(n.NodeIndex.UsedPorts[:i], n.NodeIndex.UsedPorts[i+1:]...)
			mutex.Unlock()
		}
	}
	return nil
}

func (n *Network) RemoveDisconnected() error {
	for _, conn := range n.Connections {
		n.RemovePort(conn.LocalAddr())
		conn.Close()
	}
	n.ServerListener.Close()
	mutex.Lock()
	n.Connections = map[int]net.Conn{}
	n.ServerListener = nil
	mutex.Unlock()

	return nil
}

func ConnTerminated(c net.Conn) bool {
	one := make([]byte, 1)
	c.SetReadDeadline(time.Time{})
	_, err := c.Read(one)
	if err == io.EOF {
		return true
	}
	var zero time.Time
	//set read deadline high enough to avoid timeouts
	c.SetReadDeadline(zero)
	return false
}

func (n *Network) SendConn(conn net.Conn, message Message) error {
	slice, err := json.Marshal(message)
	if err != nil {
		return errors.New("Failed to MARSHAL message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message. Error: " + err.Error())
	}
	length := len(slice)
	_, err = conn.Write(slice[0:length])
	if err != nil {
		return errors.New("Failed to write/send message to node: " + strconv.Itoa(message.To) + ". Aborting sending of message.")
	}

	n.Logger.Log("network", "debug", fmt.Sprintf("Sent message: %+v", message))
	return nil
}