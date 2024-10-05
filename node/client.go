package node

import (
	"encoding/json"
	"log"
)

// METHOD IN THIS FILE HANDLE REQUESTS OF THE CLIENT

func (node *NodeConfig) ClientCreateFile(fileName string) *MessageBody {
	if f, ok := node.getFile(fileName); ok {
		return messageBodyFormat(CodeCreateFile, StatusFileExist, f.Owner)
	}

	err := createFile(node, fileName)
	if err != nil {
		log.Printf("ClientCreateFile writeFile %q\n", err)
		return messageBodyFormat(CodeCreateFile, StatusInternalError, err.Error())
	}

	f := clientMakeCUD(node, File{Name: fileName}, updateTimeNow(CodeCreateFile, node.Node.Oauth.UserName, ""))
	node.createFile(f)
	return messageBodyFormat(CodeCreateFile, StatusOk, fileName)
}

func (node *NodeConfig) ClientUpdateFile(updateFileContent UpdateFileContent) *MessageBody {
	f, ok := node.getFile(updateFileContent.Name)
	if !ok {
		return messageBodyFormat(CodeUpdateFile, StatusFileNotFound, updateFileContent.Name)
	}

	remoteNode, ok := node.getNode(f.Owner)
	if !ok {
		return messageBodyFormat(CodeUpdateFile, StatusNodeNotOnline, f.Owner)
	}

	updateFileRaw, _ := json.Marshal(&updateFileContent)
	reqMssg := Message{
		Header: MessageHeader{
			Node:        node.Node,
			Destination: f.Owner,
		},
		Body: *messageBodyFormat(CodeUpdateFile, "", string(updateFileRaw)),
	}

	if f.Owner == node.Node.Oauth.UserName {
		return &node.HandleCodeUpdateFile(&reqMssg).Body
	}

	resMssg, err := node.NetClient(remoteNode.Address, &reqMssg)
	if err != nil {
		log.Printf("ClientUpdateFile network error: %q\n", err)
		return messageBodyFormat(CodeUpdateFile, StatusInternalError, err.Error())
	}
	return &resMssg.Body
}

func (node *NodeConfig) ClientReadFile(fileName string) *MessageBody {
	f, ok := node.getFile(fileName)
	if !ok {
		return messageBodyFormat(CodeReadFile, StatusFileNotFound, fileName)
	}

	remoteNode, ok := node.getNode(f.Owner)
	if !ok {
		return messageBodyFormat(CodeReadFile, StatusNodeNotOnline, f.Owner)
	}

	getFileRaw, _ := json.Marshal(CodeInfoContent{
		Code:    CodeReadFile,
		Content: fileName,
	})
	reqMssg := Message{
		Header: MessageHeader{
			Node:        node.Node,
			Destination: f.Owner,
		},
		Body: *messageBodyFormat(CodeGetInfo, "", string(getFileRaw)),
	}

	if f.Owner == node.Node.Oauth.UserName {
		return &node.HandleCodeGetInfo(&reqMssg).Body
	}

	resMssg, err := node.NetClient(remoteNode.Address, &reqMssg)
	if err != nil {
		log.Printf("ClientReadFile network error: %q\n", err)
		return messageBodyFormat(CodeUpdateFile, StatusInternalError, err.Error())
	}

	return &resMssg.Body
}

func (node *NodeConfig) ClientDeleteFile(fileName string) *MessageBody {
	f, ok := node.getFile(fileName)
	if !ok {
		return messageBodyFormat(CodeDeleteFile, StatusFileNotFound, fileName)
	}

	remoteNode, ok := node.getNode(f.Owner)
	if !ok {
		return messageBodyFormat(CodeDeleteFile, StatusNodeNotOnline, f.Owner)
	}

	reqMssg := Message{
		Header: MessageHeader{
			Node:        node.Node,
			Destination: f.Owner,
		},
		Body: *messageBodyFormat(CodeDeleteFile, "", string(fileName)),
	}

	if remoteNode.Oauth.UserName == node.Node.Oauth.UserName {
		return &node.HandleCodeDeleteFile(&reqMssg).Body
	}

	resMssg, err := node.NetClient(remoteNode.Address, &reqMssg)
	if err != nil {
		log.Printf("ClientReadFile network error: %q\n", err)
		return messageBodyFormat(CodeResponse, StatusInternalError, err.Error())
	}

	return &resMssg.Body
}

func (node *NodeConfig) ClientRecord() *MessageBody {
	recJson, _ := node.marshalJSONRecord()
	resBody, _ := json.Marshal(nodesClearPasswordRecordJson(string(recJson)))
	return messageBodyFormat(CodeNone, StatusOk, string(resBody))
}

func (node *NodeConfig) ClientDir() *MessageBody {
	resBody, _ := node.marshalJSONDirectory()
	return messageBodyFormat(CodeNone, StatusOk, string(resBody))
}

func (node *NodeConfig) ClientNodes() *MessageBody {
	nodesJson, _ := node.marshalJSONNodes()
	resBody, _ := json.Marshal(nodesClearPasswordJson(string(nodesJson)))
	return messageBodyFormat(CodeNone, StatusOk, string(resBody))
}

func (node *NodeConfig) ClientWebDir(mssg *Message) *Message {
	return node.NodeAuthorized(mssg)
}

func clientMakeCUD(node *NodeConfig, file File, update UpdateTime) File {
	update.Content = ""
	file.RecentUpdate = update
	if update.Code == CodeCreateFile {
		file.CreatedAt = update.At
		file.Owner = update.By
	}

	fileJson, _ := json.Marshal(&file)
	update.Content = string(fileJson)
	node.updatesChan <- &update
	return file
}

func nodesClearPasswordJson(nodesRaw string) OnlineNodes {
	var nodes OnlineNodes
	json.Unmarshal([]byte(nodesRaw), &nodes)
	for k, v := range nodes.NodesList {
		v.Oauth.Password = "******"
		nodes.NodesList[k] = v
	}
	return nodes
}

func nodesClearPasswordRecordJson(recordRaw string) Record {
	var record Record
	json.Unmarshal([]byte(recordRaw), &record)
	nodesRaw, _ := json.Marshal(record.OnlineNodes)
	record.OnlineNodes = nodesClearPasswordJson(string(nodesRaw))
	return record
}
