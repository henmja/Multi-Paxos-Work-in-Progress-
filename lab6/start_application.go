package main

import (
	"log"

	dtr "dat520.github.io/lab3/detector"
	app "dat520.github.io/lab6/application"
	mlpx "dat520.github.io/lab6/multipaxos"
	ntw "dat520.github.io/lab6/network"
)

// Main entry point to the distributed leader detection
func main() {
	log.SetFlags(log.Ldate | log.Ltime) // Set custom paramters to the logger
	channel_size := 1000000

	done := make(chan bool)                          // Boolean channel to end this program
	msgIn := make(chan ntw.ClMessage, channel_size)  // Channel to synchronize HB requests between server/client
	msgOut := make(chan ntw.ClMessage, channel_size) // Channel to synchronize HB requests between server/client

	prepareOut := make(chan mlpx.Prepare, channel_size) // Channel to receive prepare message from proposer
	acceptOut := make(chan mlpx.Accept, channel_size)   // Channel to receieve accept message from proposer

	promiseOut := make(chan mlpx.Promise, channel_size) // Channel to receive promise message from acceptor
	learnOut := make(chan mlpx.Learn, channel_size)     // Channel to receieve learn message from acceptor
	chosenValOut := make(chan mlpx.DecidedValue, 1)     // Channel to receive decided value from learner

	srvLdrIn := make(chan int, 1)
	adu := 0 // TODO

	srvChannel := ntw.SrvChannels{
		HbOut:        nil,
		PrepareOut:   prepareOut,
		AcceptOut:    acceptOut,
		PromiseOut:   promiseOut,
		LearnOut:     learnOut,
		ChosenValOut: chosenValOut,
		SrvLdrIn:     srvLdrIn,
	}

	thisNodeAddr := ntw.GetIPAddr() // Get this node IP address

	ld := dtr.NewMonLeaderDetector(ntw.ClusterOfThree) // Create new instance of Monarchical Eventual Leader Detector

	proposer := mlpx.NewProposer( // Create new instance of Proposer
		ntw.MapOfIPAddrNodeIDs[thisNodeAddr],
		len(ntw.ClusterOfThree),
		adu,
		ld,
		prepareOut,
		acceptOut,
	)
	proposer.Server_Leader = srvChannel.SrvLdrIn // Link server channel to Proposer channel to receive leader information

	acceptor := mlpx.NewAcceptor( // Create new instance of acceptor
		ntw.MapOfIPAddrNodeIDs[thisNodeAddr],
		promiseOut,
		learnOut,
	)

	learner := mlpx.NewLearner(ntw.MapOfIPAddrNodeIDs[thisNodeAddr], len(ntw.ClusterOfThree), chosenValOut) // Create new instance of learner

	srv, _ := ntw.InitServer(ld, msgIn, msgOut, proposer, acceptor, learner, srvChannel) // Create new server instance
	cl, _ := ntw.InitClient(msgIn, msgOut)                                               // Create new client instance

	apl, _ := app.InitApp(ld) // Create new instance of application

	apl.Start()      // Start application
	defer apl.Stop() // Stop application

	srv.Start()      // Start server
	defer srv.Stop() // Stop server

	cl.Start()      // Start client
	defer cl.Stop() // Stop client

	srv.Fd.Start()      // Start Eventually Perfect Failure Detector
	defer srv.Fd.Stop() // Stop Eventually Perfect Failure Detector when this program ends

	srv.Proposer.Start()      // Start proposer agent
	defer srv.Proposer.Stop() // Stop proposer agent

	srv.Acceptor.Start()      // Start acceptor agent
	defer srv.Acceptor.Stop() // Stop acceptor agent

	srv.Learner.Start()      // Start learner agent
	defer srv.Learner.Stop() // Stop learner agent

	<-done // Ends this program when this channel receives - true
}
