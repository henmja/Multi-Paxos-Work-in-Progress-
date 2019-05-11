# Multi-Paxos

The External clients provide communications between the user interface (javascript) and the Paxos nodes by sending registered transactions from the web browser to the leader node (the external client writes to the Gorilla websocket, while leader node reads from websocket). The leader node will then use the received value to start Multi-Paxos. After Multi-Paxos is finished, the decided value is sent back to the external client (leader node writes to websocket, while external client reads from it) and finally the javascript updates the user interface using the decided value. 

## Run Example with 3 Paxos nodes and two external clients using UIS Pitter machines

### Start Paxos nodes (repeat for pitter1, 11 and 19):
ssh 'USER'@badne7.ux.uis.no

ssh 'USER'@pitter'X'

cd ../lab6

go run start_application


### External Client1
 ssh -vg -L 3333:localhost:6666 'USER'@badne7.ux.uis.no
 
 ssh -vg -L 6666:localhost:3389 pitter22
 
 cd ../lab6/gorilla/websocket/examples/chat
 
 go run .*
 
 http://localhost:3333/
 
### External Client2:
 ssh -vg -L 3334:localhost:6666 'USER'@badne5.ux.uis.no
 
 ssh -vg -L 6666:localhost:3389 pitter15
 
 cd ../lab6/gorilla/websocket/examples/chat
 
 go run .*
 
 http://localhost:3334/




