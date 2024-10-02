package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"

	"github.com/urbanishimwe/webdir/node"
)

//go:embed index.html
var indexHtml string

type httpServer struct {
	node       *node.NodeConfig
	httpServer net.Listener
}

func mustNewHttpServer(node *node.NodeConfig, addr string) *httpServer {
	srv := &httpServer{node: node}
	httpServer, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start listen on ")
	}
	srv.httpServer = httpServer
	return srv
}

func (srv *httpServer) listenAndServe() {
	mux := http.NewServeMux()

	// THIS IS A SPECIAL ROUTE THAT IS ONLY USED BY NODES TO COMMUNICATE WITH EACH OTHER
	mux.HandleFunc("/webdir", srv.webDirhandler)
	// ENDS OF NODES SPECIAL ROUTES

	mux.HandleFunc("/record", srv.recordHandler)
	mux.HandleFunc("/dir", srv.dirHandler)
	mux.HandleFunc("/nodes", srv.nodesHandler)
	mux.HandleFunc("/ping", srv.recordHandler)
	mux.HandleFunc("/file", srv.fileHandler)
	mux.HandleFunc("/stop", srv.stopHandler)

	mux.HandleFunc("/", srv.homeHandler)
	log.Printf("Node(%s) HTTP listening on: %s", srv.node.Node.Oauth.UserName, srv.httpServer.Addr())
	http.Serve(srv.httpServer, mux)
}

func (srv *httpServer) homeHandler(wr http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && (r.URL.Path == "" || r.URL.Path == "/") {
		wr.Write([]byte(indexHtml))
		return
	}
	http.NotFound(wr, r)
}

func (srv *httpServer) recordHandler(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(wr, r)
		return
	}

	var resBody []byte
	mssg := srv.node.ClientRecord()
	if mssg.Status != node.StatusOk {
		resBody, _ = json.Marshal(&mssg)
	} else {
		resBody = []byte(mssg.Content)
	}
	wr.Write(resBody)
}

func (srv *httpServer) dirHandler(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(wr, r)
		return
	}

	var resBody []byte
	mssg := srv.node.ClientDir()
	if mssg.Status != node.StatusOk {
		resBody, _ = json.Marshal(&mssg)
	} else {
		resBody = []byte(mssg.Content)
	}
	wr.Write(resBody)
}

func (srv *httpServer) nodesHandler(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(wr, r)
		return
	}

	var resBody []byte
	mssg := srv.node.ClientNodes()
	if mssg.Status != node.StatusOk {
		resBody, _ = json.Marshal(&mssg)
	} else {
		resBody = []byte(mssg.Content)
	}
	wr.Write(resBody)
}

func (srv *httpServer) fileHandler(wr http.ResponseWriter, r *http.Request) {
	var resBody []byte

	switch r.Method {
	case http.MethodGet:
		resBody, _ = json.Marshal(srv.node.ClientReadFile(r.URL.Query().Get("file")))

	case http.MethodPost:
		resBody, _ = json.Marshal(srv.node.ClientCreateFile(r.URL.Query().Get("file")))

	case http.MethodPut, http.MethodPatch:
		reqBody, _ := io.ReadAll(r.Body)
		updateCont := node.UpdateFileContent{
			Name:    r.URL.Query().Get("file"),
			Content: string(reqBody),
		}
		resBody, _ = json.Marshal(srv.node.ClientUpdateFile(updateCont))

	case http.MethodDelete:
		resBody, _ = json.Marshal(srv.node.ClientDeleteFile(r.URL.Query().Get("file")))

	default:
		http.NotFound(wr, r)
	}

	wr.Write(resBody)
}

func (srv *httpServer) stopHandler(wr http.ResponseWriter, r *http.Request) {
	defer srv.httpServer.Close()
	if r.Method != http.MethodGet {
		http.NotFound(wr, r)
		return
	}
	srv.node.Stop()
	wr.Write(nil)
}

func (srv *httpServer) webDirhandler(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		wr.WriteHeader(http.StatusMethodNotAllowed)
		wr.Write(webDirFormatBadRequest("Method Not Allowed"))
		return
	}

	reqBody, _ := io.ReadAll(r.Body)
	var mssg node.Message
	err := json.Unmarshal(reqBody, &mssg)
	if err != nil {
		wr.WriteHeader(http.StatusBadRequest)
		wr.Write(webDirFormatBadRequest("Bad Request"))
		return
	}

	resMssg := srv.node.ClientWebDir(&mssg)
	resBody, _ := json.Marshal(&resMssg)
	wr.Write(resBody)
}

func webDirFormatBadRequest(content string) []byte {
	mssg := node.Message{
		Body: node.MessageBody{
			Code:    node.CodeResponse,
			Status:  node.StatusBadFormat,
			Content: content,
		},
	}

	resBody, _ := json.Marshal(&mssg)
	return resBody
}

func webDirMakeHTTPRequest(address string, mssg *node.Message) (*node.Message, error) {
	reqBody, _ := json.Marshal(mssg)
	newURL := url.URL{
		Host:   address,
		Path:   "webdir",
		Scheme: "http",
	}

	resp, err := http.Post(newURL.String(), mime.TypeByExtension(".json"), bytes.NewReader(reqBody))
	if err != nil {
		log.Println("webDirMakeHTTPRequest http post failed")
		return &node.Message{}, err
	}

	defer resp.Body.Close()
	resRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("webDirMakeHTTPRequest resp body read failed")
		return &node.Message{}, err
	}

	var resMssg node.Message
	err = json.Unmarshal(resRaw, &resMssg)
	return &resMssg, err
}
