package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/uis-dat520-s2019/Paxosfans/lab3/detector"
	logger "github.com/uis-dat520-s2019/Paxosfans/lab3/logging"
	"github.com/uis-dat520-s2019/Paxosfans/lab3/network"
)

var (
	servers = flag.String(
		"servers",
		"localhost",
		"run application in localhost, or pitter machines",
	)
	nodeIndex = flag.Int(
		"nodeIndex",
		-1,
		"Positive",
	)
	cond = flag.String(
		"cond",
		"debug",
		"Logging condition",
	)
	help = flag.Bool(
		"help",
		false,
		"Usage",
	)
	print = flag.Bool(
		"print",
		true,
		"Print messages",
	)
	notsys = flag.Bool(
		"notsys",
		false,
		"Only print network msgs.",
	)
)

func main() {
	/*TEST program:
	go build
	./main -nodeIndex 1
	..
	..
	./main -nodeIndex 4
	*/

	flag.Usage = usage
	flag.Parse()
	if *help {
		flag.Usage()
		os.Exit(0)
	}
	if *nodeIndex < 0 {
		flag.Usage()
		os.Exit(0)
	}
	//channel for os signals:
	osSignalChan := make(chan os.Signal, 1)
	//notify when process/node killed:
	signal.Notify(osSignalChan, os.Interrupt)
	//init logger
	logger, err := logger.InitLogger("./logs", "systemLog", "networkLog", *cond, *print, *nodeIndex, *notsys)
	if err != nil {
		fmt.Println("Creating logger failed with error: ", err.Error())
		os.Exit(0)
	}
	logger.Log("system", "debug", "\n\n\n\n#Application: Started at "+time.Now().Format(time.RFC3339))
	logger.Log("network", "debug", "\n\n\n\n#Application: Started at "+time.Now().Format(time.RFC3339))

	//retrieve config
	config, err := loadJson()
	if err != nil {
		logger.Log("system", "crit", "#Application: Failed loading config with error: "+err.Error())
		os.Exit(0)
	}

	//init network
	n, err := network.InitNetwork(config, *nodeIndex, *servers, logger)
	if err != nil {
		logger.Log("system", "crit", "#Application: Failed to create network with error: "+err.Error())
		os.Exit(0)
	}

	//ids for leader detector
	nodeIDs := []int{n.NodeIndex.Id}
	for _, node := range n.Nodes {
		nodeIDs = append(nodeIDs, node.Id)
	}
	//init leader detector
	ld := detector.NewMonLeaderDetector(nodeIDs)

	//init failure detector
	hbChan := make(chan detector.Heartbeat, 16)
	fd := detector.NewEvtFailureDetector(n.NodeIndex.Id, nodeIDs, ld, 1*time.Second, hbChan) //how to get things sent on the hbChan onto the network???

	//start TCP server
	err = n.ConnectTCP()
	if err != nil {
		n.Logger.Log("system", "err", "#Application: Error when establishing peer to peer network. Error: "+err.Error())
	}

	//subscribe to leader changes
	leaderChangeChan := ld.Subscribe()

	go func() {
		for {
			select {
			case hb := <-hbChan:

				netMsg := network.Message{
					Type:    "heartbeat",
					To:      hb.To,
					From:    hb.From,
					Msg:     "",
					Request: hb.Request,
				}
				n.SChan <- netMsg
			case msg := <-n.RChan:
				logger.Log("network", "debug", fmt.Sprintf("Got message: %+v", msg))
				switch {
				case msg.Type == "heartbeat":
					hb := detector.Heartbeat{
						To:      msg.To,
						From:    msg.From,
						Request: msg.Request,
					}
					fd.DeliverHeartbeat(hb)
				default:
					n.UnmodRChan <- msg
				}
			}
		}
	}()

	fd.Start()

	//print leader changes
	for {
		select {
		case newLeader := <-leaderChangeChan:
			fmt.Println("\n#Application: New Leader -> ", strconv.Itoa(newLeader), "\n")
			logger.Log("system", "debug", "#Application: New Leader -> "+strconv.Itoa(newLeader))

		//clean up when nodes/processes are killed:
		case <-osSignalChan:
			logger.Log("system", "debug", "#Application: Exiting...")
			logger.Log("system", "debug", "#Application: Cleaning up network layer...")
			err := n.RemoveDisconnected()
			if err != nil {
				logger.Log("system", "debug", "#Application: Error from n.RemoveDisconnected(): "+err.Error())
			}
			logger.Log("system", "debug", "#Application: Exiting!")

			os.Exit(0)
		}
	}

}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
}

func loadJson() (network.ConfigInstance, error) {
	file, err := os.Open("config.json")
	if err != nil {
		return network.ConfigInstance{}, err
	}
	decoder := json.NewDecoder(file)
	config := network.Config{}
	err = decoder.Decode(&config) //invalid argument
	if err != nil {
		return network.ConfigInstance{}, err
	}
	var conf network.ConfigInstance
	if *servers == "pitter" {
		conf = config.Pitter
	} else {
		conf = config.Localhost
	}
	return conf, nil
}