package node

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
)

func (node *NodeConfig) Stop() {
	close(node.stopNode)
}

func MustInitServer(temp NodeConfig, meshInitiator string, netClient NetClient) *NodeConfig {

	newNode := temp
	newNode.Init()
	if netClient == nil {
		log.Fatalf("Node NetClient required")
	}
	newNode.NetClient = netClient
	err := initializeNode(&newNode)
	if err != nil {
		log.Fatalf("Failed to initialize node: %q\n", err)
	}

	// If node is a network inititator don't advertise on network
	if meshInitiator != "" {
		err = advertiseOnNetwork(&newNode, meshInitiator)
		if err != nil {
			log.Fatalf("Failed to advertise node on network: %q\n", err)
		}
	} else {
		log.Printf("Node(%s) is mesh initiator %q\n", newNode.Node.Oauth.UserName, newNode.Node.Address)
	}

	err = addOwnedFiles(&newNode)
	if err != nil {
		log.Println("WalkDir failed with ", err)
	}

	go nodeDequeUpdates(&newNode)
	go nodePing(&newNode)
	return &newNode
}

func initializeNode(node *NodeConfig) error {
	err := initBaseDir(node)
	if err != nil {
		return err
	}

	if node.Node.Oauth.UserName == "" {
		node.Node.Oauth.UserName = randomText()
	}
	if node.Node.Oauth.Password == "" {
		node.Node.Oauth.Password = randomText()
	}

	if node.PublicAddr == nil {
		return errors.New("public address is required")
	}

	if node.Node.Address == "" {
		node.Node.Address = node.PublicAddr.String()
	}

	node.createNode(node.Node, updateTimeNow(CodeRegister, node.Node.Oauth.UserName, ""))

	return nil
}

// join the mesh network through mesh initiator by sending a CodeRegister
func advertiseOnNetwork(node *NodeConfig, initiator string) error {
	log.Printf("Registering node through %q\n", initiator)
	message := Message{
		Header: MessageHeader{
			Node:        node.Node,
			Destination: "",
		},
		Body: MessageBody{
			Code: CodeRegister,
		},
	}

	resBody, err := node.NetClient(initiator, &message)
	if err != nil {
		log.Printf("Failed to dial mesh initiator message")
		return err
	}

	if string(resBody.Body.Status) != string(StatusOk) {
		return errors.New("mesh initiator responded with " + string(resBody.Body.Status))
	}

	record := Record{}
	err = json.Unmarshal([]byte(resBody.Body.Content), &record)
	if err != nil {
		log.Printf("Failed to unmarshall mesh initiator record")
		return err
	}
	node.Record = record
	node.SetMeshInitiator(resBody.Header.Node)
	return nil
}

func nodeDequeUpdates(node *NodeConfig) {
	for {
		select {
		case updates := <-node.updatesChan:
			sendUpdates(node, updates)
		case <-node.stopNode:
			return
		}
	}
}

func sendUpdates(node *NodeConfig, updates *UpdateTime) {
	log.Printf("Sending new updates(%s)\n", updates.Code)
	for _, _node := range copyNodesAddress(node) {
		updateTimeRaw, _ := json.Marshal(updates)
		mssg := Message{
			Header: MessageHeader{
				Node: node.Node,
			},
			Body: MessageBody{
				Code:    CodeUpdate,
				Content: string(updateTimeRaw),
			},
		}
		resMssg, err := node.NetClient(_node.Address, &mssg)
		if err != nil {
			log.Printf("(sendUpdates) dialing node(%s) error: %q\n", _node.Oauth.UserName, err)
			continue
		}

		if resMssg.Body.Status != StatusOk {
			log.Printf("Node(%s) update code(%s) responded with %q\n", _node.Oauth.UserName, resMssg.Body.Code, resMssg.Body.Status)
		}
	}
}

func copyNodesAddress(node *NodeConfig) []Node {
	// if _node := node.meshInitiator(); _node.Address != "" {
	// 	// FOR NODES THAT ARE NOT MESH INITIATOR, UPDATES ARE ONLY SENT TO THE MESH INITIATOR
	// 	return []Node{_node}
	// }

	addrs := []Node{}
	node.nodesRwMx.RLock()
	defer node.nodesRwMx.RUnlock()
	for _, n := range node.Record.OnlineNodes.NodesList {
		if n.Oauth.UserName != node.Node.Oauth.UserName {
			addrs = append(addrs, n)
		}
	}
	return addrs
}

func nodePing(node *NodeConfig) {
	if node.meshInitiator().Address != "" {
		// ONLY MESH INITIATOR CAN SEND PINGS TO OTHER NODES
		return
	}
	for {
		select {
		case <-node.initiatorPing.C:
			mssg := Message{
				Header: MessageHeader{
					Node: node.Node,
				},
				Body: MessageBody{
					Code: CodePing,
				},
			}
			for _, _node := range copyNodesAddress(node) {
				resMssg, err := node.NetClient(_node.Address, &mssg)
				if e, ok := checkCantReachAddrError(err); ok {
					// NODE COULD NOT BE REACHED
					log.Printf("(nodePing) node(%s) could not be reached at(%s)\nError: %q\n", _node.Oauth.UserName, _node.Address, e)
					removeNodes(node, _node)
				}

				if resMssg.Body.Status != StatusOk {
					log.Printf("(nodePing) unexpected response status %q from %q\n", resMssg.Body.Status, _node.Oauth.UserName)
				}
			}

		case <-node.stopNode:
			return
		}
	}
}

func checkCantReachAddrError(err error) (error, bool) {
	// FOR NOW TO KNOW IF THE NODE IS NOT ONLINE WE CHECK
	if err == nil {
		return nil, false
	}
	switch e := err.(type) {
	case *net.ParseError, *net.AddrError, net.UnknownNetworkError, net.InvalidAddrError:
		return e, true
	case *net.DNSError:
		if e.IsNotFound {
			return e, true
		}
	}
	if strings.Contains(err.Error(), "connection refused") {
		return err, true
	}
	return err, false
}

func removeNodes(node *NodeConfig, n Node) {
	node.deleteNode(n.Oauth.UserName, updateTimeNow(CodeNodes, node.Node.Oauth.UserName, ""))
	nodesJson, _ := node.marshalJSONNodes()
	now := updateTimeNow(CodeDrop, node.Node.Oauth.UserName, string(nodesJson))
	node.updatesChan <- &now
}
