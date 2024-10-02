package node

import (
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"
)

type Record struct {
	OnlineNodes OnlineNodes `json:"online_nodes"`
	Directory   Directory   `json:"directory"`
}

type OnlineNodes struct {
	NodesList    map[string]Node `json:"nodes_list"`
	RecentUpdate UpdateTime      `json:"recent_update"`
}

type Directory struct {
	FilesList    map[string]File `json:"files_list"`
	RecentUpdate UpdateTime      `json:"recent_update"`
}

type Message struct {
	Header MessageHeader `json:"header"`
	Body   MessageBody   `json:"body"`
}

type Node struct {
	Address string `json:"address"`
	Oauth   Oauth  `json:"oauth"`
}

type Oauth struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type UpdateTime struct {
	At      time.Time `json:"at"`
	By      string    `json:"by"`
	Content string    `json:"content"`
	Code    Code      `json:"code"`
}

type File struct {
	Owner        string     `json:"owner"`
	Name         string     `json:"name"`
	CreatedAt    time.Time  `json:"created_at"`
	RecentUpdate UpdateTime `json:"recent_update"`
}

type MessageHeader struct {
	Node        Node   `json:"oauth"`
	Destination string `json:"destination"`
}

type MessageBody struct {
	Code    Code           `json:"action"`
	Status  ResponseStatus `json:"status"`
	Content string         `json:"content"`
}

// used internally
type UpdateFileContent struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// used internally
type CodeInfoContent struct {
	Code    Code   `json:"code"`
	Content string `json:"content"`
}

type NetClient func(remoteAddr string, message *Message) (*Message, error)

type Code uint32

const (
	CodeNone Code = iota
	CodeResponse
	CodeGetInfo
	CodeUpdate
	CodePing
	CodeNodes
	CodeDirectory
	CodeCreateFile
	CodeReadFile
	CodeUpdateFile
	CodeDeleteFile
	CodeRegister
)

func (c Code) String() string {
	var cName = [...]string{
		"CodeNone",
		"CodeResponse",
		"CodeGetInfo",
		"CodeUpdate",
		"CodePing",
		"CodeNodes",
		"CodeDirectory",
		"CodeCreateFile",
		"CodeReadFile",
		"CodeUpdateFile",
		"CodeDeleteFile",
		"CodeRegister",
	}
	if int(c) < len(cName) {
		return cName[c]
	}
	return "Invalid Code"
}

type ResponseStatus string

const (
	StatusOk            ResponseStatus = "OK"
	StatusNotOauth      ResponseStatus = "Node Not Authorized"
	StatusBadFormat     ResponseStatus = "Message Bad Format"
	StatusInternalError ResponseStatus = "Internal Error"
	StatusNodeNotOnline ResponseStatus = "Node Not Online"
	StatusNodeExist     ResponseStatus = "Node Exist"
	StatusFileExist     ResponseStatus = "File Exist"
	StatusFileNotFound  ResponseStatus = "File Not Found"
	StatusFileUpdateOld ResponseStatus = "File Update Old"
)

// const TimeFormat = time.RFC3339Nano

// NodeConfig is internally used by an online node
type NodeConfig struct {
	// The local directory for owned files
	BaseFilePath string
	// in-memory copy of record
	Record Record
	// Internet address of the node
	PublicAddr net.Addr
	// a copy of node's info
	Node Node
	// Network client
	NetClient NetClient
	// initiator shows that this nodes is mesh initiator
	initiator     Node
	updatesChan   chan *UpdateTime
	stopNode      chan bool
	initiatorPing *time.Ticker
	nodesRwMx     *sync.RWMutex
	dirsRwMx      *sync.RWMutex
	initiatorRwMx *sync.RWMutex
}

func (node *NodeConfig) meshInitiator() Node {
	node.initiatorRwMx.RLock()
	defer node.initiatorRwMx.RUnlock()
	return node.initiator
}

func (node *NodeConfig) SetMeshInitiator(nd Node) {
	node.initiatorRwMx.Lock()
	defer node.initiatorRwMx.Unlock()
	node.initiator = nd
}

func (node *NodeConfig) Init() {
	if node.Record.OnlineNodes.NodesList == nil {
		node.Record.OnlineNodes.NodesList = map[string]Node{}
	}
	if node.Record.Directory.FilesList == nil {
		node.Record.Directory.FilesList = map[string]File{}
	}

	node.updatesChan = make(chan *UpdateTime, 100)
	node.initiatorPing = time.NewTicker(time.Second)
	node.stopNode = make(chan bool)
	node.nodesRwMx = &sync.RWMutex{}
	node.dirsRwMx = &sync.RWMutex{}
	node.initiatorRwMx = &sync.RWMutex{}
}

// The following avoid reads and writes to be synced

func (node *NodeConfig) marshalJSONRecord() ([]byte, error) {
	node.holdAllRLocks()
	defer node.releaseAllRLocks()
	return json.Marshal(node.Record)
}

func (node *NodeConfig) holdAllRLocks() {
	node.nodesRwMx.RLock()
	node.dirsRwMx.RLock()
}

func (node *NodeConfig) releaseAllRLocks() {
	node.nodesRwMx.RUnlock()
	node.dirsRwMx.RUnlock()
}

func (node *NodeConfig) marshalJSONNodes() ([]byte, error) {
	node.nodesRwMx.RLock()
	defer node.nodesRwMx.RUnlock()
	return json.Marshal(node.Record.OnlineNodes)
}

func (node *NodeConfig) setOnlineNodes(nodes OnlineNodes) {
	node.nodesRwMx.Lock()
	defer node.nodesRwMx.Unlock()
	node.Record.OnlineNodes = nodes
}

func (node *NodeConfig) getNode(nodeName string) (Node, bool) {
	node.nodesRwMx.RLock()
	defer node.nodesRwMx.RUnlock()
	cl, ok := node.Record.OnlineNodes.NodesList[nodeName]
	return cl, ok
}

func (node *NodeConfig) createNode(cl Node, updateTime UpdateTime) {
	node.nodesRwMx.Lock()
	defer node.nodesRwMx.Unlock()
	node.Record.OnlineNodes.NodesList[cl.Oauth.UserName] = cl
	node.Record.OnlineNodes.RecentUpdate = updateTime
}

func (node *NodeConfig) deleteNode(nodeName string, updateTime UpdateTime) {
	node.nodesRwMx.Lock()
	defer node.nodesRwMx.Unlock()
	delete(node.Record.OnlineNodes.NodesList, nodeName)
	node.Record.OnlineNodes.RecentUpdate = updateTime
	log.Printf("Deleted node(%q)\n", nodeName)
}

func (node *NodeConfig) getRecentUpdateNodes() UpdateTime {
	node.nodesRwMx.RLock()
	defer node.nodesRwMx.RUnlock()
	return node.Record.OnlineNodes.RecentUpdate
}

func (node *NodeConfig) marshalJSONDirectory() ([]byte, error) {
	node.dirsRwMx.RLock()
	defer node.dirsRwMx.RUnlock()
	return json.Marshal(node.Record.Directory)
}

func (node *NodeConfig) setDir(dir Directory) {
	node.dirsRwMx.Lock()
	defer node.dirsRwMx.Unlock()
	node.Record.Directory = dir
}

func (node *NodeConfig) getFile(fileName string) (File, bool) {
	node.dirsRwMx.RLock()
	defer node.dirsRwMx.RUnlock()
	f, ok := node.Record.Directory.FilesList[fileName]
	return f, ok
}

func (node *NodeConfig) createFile(f File) {
	node.dirsRwMx.Lock()
	defer node.dirsRwMx.Unlock()
	node.Record.Directory.FilesList[f.Name] = f
	node.Record.Directory.RecentUpdate = f.RecentUpdate
}

func (node *NodeConfig) deleteFile(fileName string, updateTime UpdateTime) {
	node.dirsRwMx.Lock()
	defer node.dirsRwMx.Unlock()
	delete(node.Record.Directory.FilesList, fileName)
	node.Record.Directory.RecentUpdate = updateTime
}

func (node *NodeConfig) getRecentUpdateDirectory() UpdateTime {
	node.dirsRwMx.RLock()
	defer node.dirsRwMx.RUnlock()
	return node.Record.Directory.RecentUpdate
}
