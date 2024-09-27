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

	err := writeFile(node, fileName, nil)
	if err != nil {
		log.Printf("ClientCreateFile writeFile %q\n", err)
		return messageBodyFormat(CodeCreateFile, StatusInternalError, err.Error())
	}

	clientMakeCUD(node, File{Name: fileName}, updateTimeNow(CodeCreateFile, node.Node.Oauth.UserName, ""))
	return messageBodyFormat(CodeCreateFile, StatusOk, fileName)
}

func (node *NodeConfig) ClientUpdateFile(updateFileContent UpdateFileContent) *MessageBody {
	f, ok := node.getFile(updateFileContent.Name)
	if !ok {
		return messageBodyFormat(CodeUpdateFile, StatusFileNotFound, updateFileContent.Name)
	} else if f.Owner == node.Node.Oauth.UserName {
		if err := writeFile(node, f.Name, []byte(updateFileContent.Content)); err != nil {
			log.Printf("ClientUpdateFile write file error %q\n", err)
			return messageBodyFormat(CodeUpdateFile, StatusInternalError, err.Error())
		}
		clientMakeCUD(node, f, updateTimeNow(CodeUpdateFile, node.Node.Oauth.UserName, ""))
		return messageBodyFormat(CodeUpdateFile, StatusOk, f.Name)
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

	if f.Owner == node.Node.Oauth.UserName {
		body, err := readFile(node, fileName)
		if err != nil {
			log.Printf("ClientReadFile read file(%s) failed: %q\n", fileName, err)
			return messageBodyFormat(CodeReadFile, StatusInternalError, err.Error())
		}
		return messageBodyFormat(CodeReadFile, StatusOk, string(body))
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

	if f.Owner == node.Node.Oauth.UserName {
		err := deleteFile(node, fileName)
		if err != nil {
			log.Printf("ClientDeleteFile read file(%s) failed: %q\n", fileName, err)
			return messageBodyFormat(CodeDeleteFile, StatusInternalError, err.Error())
		}
		clientMakeCUD(node, f, updateTimeNow(CodeDeleteFile, node.Node.Oauth.UserName, ""))
		return messageBodyFormat(CodeDeleteFile, StatusOk, f.Name)
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
	resMssg, err := node.NetClient(remoteNode.Address, &reqMssg)
	if err != nil {
		log.Printf("ClientReadFile network error: %q\n", err)
		return messageBodyFormat(CodeUpdateFile, StatusInternalError, err.Error())
	}

	return &resMssg.Body
}

func (node *NodeConfig) ClientRecord() *MessageBody {
	resBody, _ := node.marshalJSONRecord()
	return messageBodyFormat(CodeNone, StatusOk, string(resBody))
}

func (node *NodeConfig) ClientDir() *MessageBody {
	resBody, _ := node.marshalJSONDirectory()
	return messageBodyFormat(CodeNone, StatusOk, string(resBody))
}

func (node *NodeConfig) ClientNodes() *MessageBody {
	resBody, _ := node.marshalJSONNodes()
	return messageBodyFormat(CodeNone, StatusOk, string(resBody))
}

func (node *NodeConfig) ClientWebDir(mssg *Message) *Message {
	return node.NodeAuthorized(mssg)
}

func clientMakeCUD(node *NodeConfig, file File, update UpdateTime) {
	update.Content = ""
	file.RecentUpdate.At = update.At
	if update.Code == CodeCreateFile {
		file.CreatedAt = update.At
		file.Owner = update.By
	}

	fileJson, _ := json.Marshal(&file)
	update.Content = string(fileJson)
	node.updatesChan <- &update

	if update.Code == CodeCreateFile || update.Code == CodeUpdateFile {
		node.createFile(file)
	} else if update.Code == CodeDeleteFile {
		updateCopy := update
		updateCopy.Content = ""
		node.deleteFile(file.Name, updateCopy)
	}
}
