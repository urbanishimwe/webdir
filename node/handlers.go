package node

import (
	"encoding/json"
	"log"
	"time"
)

// METHOD IN THIS FILE HANDLE MESSAGES SENT FROM ANOTHER NODE

func (node *NodeConfig) NodeAuthorized(mssg *Message) *Message {
	cl, ok := node.getNode(mssg.Header.Node.Oauth.UserName)
	if mssg.Body.Code == CodeRegister {
		return node.HandleCodeRegister(mssg)
	}
	if !ok || cl.Oauth.Password != mssg.Header.Node.Oauth.Password {
		return responseFormat(node, mssg, StatusNotOauth, false, "")
	}
	return node.Handle(mssg)
}

func (node *NodeConfig) Handle(mssg *Message) *Message {
	switch mssg.Body.Code {
	case CodeRegister:
		return node.HandleCodeRegister(mssg)
	case CodeGetInfo:
		return node.HandleCodeGetInfo(mssg)
	case CodeUpdateFile:
		return node.HandleCodeUpdateFile(mssg)
	case CodeDeleteFile:
		return node.HandleCodeDeleteFile(mssg)
	case CodeUpdate:
		return node.HandleCodeUpdate(mssg)
	case CodePing:
		return node.HandleCodePing(mssg)
	default:
		return responseFormat(node, mssg, StatusBadFormat, false, "")
	}
}

func (node *NodeConfig) HandleCodeRegister(mssg *Message) *Message {
	cl, ok := node.getNode(mssg.Header.Node.Oauth.UserName)
	// check if this node is trying to register with existing username
	if ok && cl.Oauth.Password != mssg.Header.Node.Oauth.Password {
		return responseFormat(node, mssg, StatusNodeExist, false, "")
	}

	// Otherwise its a new node or existing node with a changed IP address
	updates := updateTimeNow(CodeRegister, node.Node.Oauth.UserName, "")
	newNode := mssg.Header.Node
	node.createNode(newNode, updates)
	content, _ := node.marshalJSONNodes()
	updates.Content = string(content)
	node.updatesChan <- &updates
	resContent, _ := node.marshalJSONRecord()
	return responseFormat(node, mssg, StatusOk, true, string(resContent))
}

func (node *NodeConfig) HandleCodeGetInfo(mssg *Message) *Message {
	var cont CodeInfoContent
	err := json.Unmarshal([]byte(mssg.Body.Content), &cont)
	if err != nil {
		log.Printf("(HandleCodeGetInfo) marshalling failed%q\n", err)
		// Internal server error, JSON marshal failed
		return responseFormat(node, mssg, StatusInternalError, true, err.Error())
	}

	var resBody []byte
	switch cont.Code {
	case CodeNodes, CodeRegister:
		resBody, _ = node.marshalJSONNodes()

	case CodeDirectory, CodeCreateFile, CodeUpdateFile, CodeDeleteFile:
		resBody, _ = node.marshalJSONDirectory()

	case CodeReadFile:
		f, ok := node.getFile(cont.Content)
		if !ok || f.Owner != node.Node.Oauth.UserName {
			return responseFormat(node, mssg, StatusFileNotFound, true, "")
		}

		resBody, err = readFile(node, cont.Content)

	case 0:
		// Assume node just want all record
		resBody, _ = node.marshalJSONRecord()

	default:
		return responseFormat(node, mssg, StatusBadFormat, true, "")
	}

	if err != nil {
		log.Printf("HandleCodeGetInfo error %q\n", err)
		return responseFormat(node, mssg, StatusInternalError, true, err.Error())
	}

	return responseFormat(node, mssg, StatusOk, true, string(resBody))
}

func (node *NodeConfig) HandleCodeUpdateFile(mssg *Message) *Message {
	var content UpdateFileContent
	err := json.Unmarshal([]byte(mssg.Body.Content), &content)
	if err != nil {
		log.Printf("HandleCodeUpdateFile unmarshal error %q\n", err)
		return responseFormat(node, mssg, StatusInternalError, true, err.Error())
	}
	f, ok := node.getFile(content.Name)
	if !ok || f.Owner != node.Node.Oauth.UserName {
		return responseFormat(node, mssg, StatusFileNotFound, true, "")
	}
	if err := writeFile(node, content.Name, []byte(content.Content)); err != nil {
		log.Printf("HandleCodeUpdateFile write file error %q\n", err)
		return responseFormat(node, mssg, StatusInternalError, true, err.Error())
	}

	f.RecentUpdate.At = time.Now()
	f.RecentUpdate.By = mssg.Header.Node.Oauth.UserName
	f.RecentUpdate.Code = CodeUpdateFile

	node.createFile(f)

	return responseFormat(node, mssg, StatusOk, true, "")
}

func (node *NodeConfig) HandleCodeDeleteFile(mssg *Message) *Message {
	f, ok := node.getFile(mssg.Body.Content)
	if !ok || f.Owner != node.Node.Oauth.UserName {
		return responseFormat(node, mssg, StatusFileNotFound, true, "")
	}

	updates := UpdateTime{
		By:   mssg.Header.Node.Oauth.UserName,
		At:   time.Now(),
		Code: CodeDeleteFile,
	}

	node.deleteFile(mssg.Body.Content, updates)
	return responseFormat(node, mssg, StatusOk, true, "")
}

func (node *NodeConfig) HandleCodeUpdate(mssg *Message) *Message {
	var updateContent UpdateTime
	err := json.Unmarshal([]byte(mssg.Body.Content), &updateContent)
	if err != nil {
		log.Printf("(HandleCodeUpdate) unmarshalling UpdateTime failed%q\n", err)
		// Internal server error, JSON marshal failed
		return responseFormat(node, mssg, StatusInternalError, true, err.Error())
	}

	switch updateContent.Code {
	case CodeRegister, CodeNodes:
		var nodes OnlineNodes
		err := json.Unmarshal([]byte(updateContent.Content), &nodes)
		if err != nil {
			log.Printf("(HandleCodeUpdate) unmarshalling(%q) OnlineNodes failed%q\n", updateContent.Content, err)
			break
		}
		if nodes.RecentUpdate.At.After(node.getRecentUpdateNodes().At) {
			node.setOnlineNodes(nodes)
		}

	case CodeDirectory:
		var dir Directory
		err := json.Unmarshal([]byte(updateContent.Content), &dir)
		if err != nil {
			log.Printf("(HandleCodeUpdate) unmarshalling Directory failed%q\n", err)
			break
		}
		if dir.RecentUpdate.At.After(node.getRecentUpdateDirectory().At) {
			node.setDir(dir)
		}

	case CodeCreateFile, CodeUpdateFile, CodeDeleteFile:
		var fileExternal File
		err := json.Unmarshal([]byte(updateContent.Content), &fileExternal)
		if err != nil {
			log.Printf("(HandleCodeUpdate) unmarshalling Directory failed%q\n", err)
			break
		}

		fileInternal, ok := node.getFile(fileExternal.Name)
		// if !ok {
		// 	return responseFormat(node, mssg, StatusFileNotFound, true, fileExternal.Name)
		// }

		// log.Printf("")
		if updateContent.Code == CodeDeleteFile {
			if fileInternal.CreatedAt.After(fileExternal.CreatedAt) {
				return responseFormat(node, mssg, StatusFileUpdateOld, true, fileInternal.Name)
			}
			node.deleteFile(fileInternal.Name, fileInternal.RecentUpdate)

		} else if updateContent.Code == CodeCreateFile {
			if ok && fileInternal.RecentUpdate.At.Before(fileExternal.RecentUpdate.At) {
				return responseFormat(node, mssg, StatusFileExist, true, fileInternal.Name)
			}
			node.createFile(fileExternal)
		} else {
			if fileInternal.RecentUpdate.At.After(fileExternal.RecentUpdate.At) {
				return responseFormat(node, mssg, StatusFileUpdateOld, true, fileExternal.Name)
			}
			node.createFile(fileExternal)
		}

	default:
		return responseFormat(node, mssg, StatusBadFormat, true, "")
	}

	return responseFormat(node, mssg, StatusOk, true, "")
}

func (node *NodeConfig) HandleCodePing(mssg *Message) *Message {
	return responseFormat(node, mssg, StatusOk, true, "")
}
