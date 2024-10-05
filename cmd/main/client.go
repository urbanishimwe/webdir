package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/urbanishimwe/webdir/node"
)

//go:embed index.html
var indexHtml string

//go:embed login.html
var loginHtml string

const oauthCookieName = "access-token"

type httpServer struct {
	node           *node.NodeConfig
	httpServer     net.Listener
	clientPassword string
}

func mustNewHttpServer(addr string) *httpServer {
	srv := &httpServer{}
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
	mux.HandleFunc("/webdir", srv.webDirHandler)

	mux.HandleFunc("/login", srv.loginHandler)
	mux.HandleFunc("/", srv.homeHandler)

	// ROUTES THAT NEEDS OAUTH
	mux.HandleFunc("/record", srv.oauthFirst(srv.recordHandler, http.MethodGet))
	mux.HandleFunc("/dir", srv.oauthFirst(srv.dirHandler, http.MethodGet))
	mux.HandleFunc("/nodes", srv.oauthFirst(srv.nodesHandler, http.MethodGet))
	mux.HandleFunc("/ping", srv.oauthFirst(srv.recordHandler, http.MethodGet))
	mux.HandleFunc("/file", srv.oauthFirst(srv.fileHandler, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete))
	mux.HandleFunc("/stop", srv.oauthFirst(srv.stopHandler, http.MethodGet))
	// END OF ROUTES ThAT NEEDS OAUTH

	log.Printf("Node(%s) HTTP listening on: %s", srv.node.Node.Oauth.UserName, srv.httpServer.Addr())
	http.Serve(srv.httpServer, mux)
}

func (srv *httpServer) oauthFirst(h http.HandlerFunc, allowedMethod ...string) http.HandlerFunc {
	return func(wr http.ResponseWriter, r *http.Request) {
		if !checkAllowedMethod(r.Method, allowedMethod) {
			wr.WriteHeader(http.StatusMethodNotAllowed)
			wr.Header().Set("Allow", strings.Join(allowedMethod, ", "))
			return
		}

		if srv.clientPassword == "" || srv.verifyCookie(r) {
			h(wr, r)
			return
		}

		wr.WriteHeader(http.StatusUnauthorized)
	}
}

func (srv *httpServer) loginHandler(wr http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cookie := http.Cookie{
			Name:   oauthCookieName,
			MaxAge: -1,
		}
		// logout the client as well
		http.SetCookie(wr, &cookie)
		wr.Write([]byte(loginHtml))
		return
	}

	if r.Method != http.MethodPost {
		wr.WriteHeader(http.StatusMethodNotAllowed)
		wr.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
		return
	}

	// Go doesn't populate json body into r.FormValue after calling r.ParseForm...
	// make it easy and add password directly inside the body
	password, err := io.ReadAll(http.MaxBytesReader(wr, r.Body, 40))
	if err != nil {
		wr.WriteHeader(http.StatusBadRequest)
		wr.Write([]byte(err.Error()))
		return
	}

	if !bytes.Equal([]byte(srv.clientPassword), password) {
		wr.WriteHeader(http.StatusBadRequest)
		wr.Write([]byte("Wrong password!"))
		return
	}

	maxAge := 24 * 60 * 60 // 1 day seconds
	expiresAt := time.Now().Add(time.Second * time.Duration(maxAge))
	cookie := http.Cookie{
		Name:     oauthCookieName,
		Value:    srv.signCookie(expiresAt),
		MaxAge:   maxAge, // expires after 1 day
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
	}

	http.SetCookie(wr, &cookie)
}

func (srv *httpServer) homeHandler(wr http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "", "/":
	default:
		http.NotFound(wr, r)
		return
	}

	if r.Method != http.MethodGet {
		wr.WriteHeader(http.StatusMethodNotAllowed)
		wr.Header().Set("Allow", http.MethodGet)
		return
	}

	if srv.clientPassword == "" || srv.verifyCookie(r) {
		wr.Write([]byte(indexHtml))
		return
	}

	srv.loginHandler(wr, r)
}

func (srv *httpServer) recordHandler(wr http.ResponseWriter, r *http.Request) {
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
		resBody, _ = json.Marshal(srv.node.ClientReadFile(r.URL.Query().Get("name")))

	case http.MethodPost:
		resBody, _ = json.Marshal(srv.node.ClientCreateFile(r.URL.Query().Get("name")))

	case http.MethodPut, http.MethodPatch:
		reqBody, _ := io.ReadAll(http.MaxBytesReader(wr, r.Body, 1<<20))
		updateCont := node.UpdateFileContent{
			Name:    r.URL.Query().Get("name"),
			Content: string(reqBody),
		}
		resBody, _ = json.Marshal(srv.node.ClientUpdateFile(updateCont))

	case http.MethodDelete:
		resBody, _ = json.Marshal(srv.node.ClientDeleteFile(r.URL.Query().Get("name")))
	}

	wr.Write(resBody)
}

func (srv *httpServer) stopHandler(wr http.ResponseWriter, r *http.Request) {
	defer srv.httpServer.Close()
	srv.node.Stop()
	wr.Write([]byte("OK"))
	if fl, ok := wr.(http.Flusher); ok {
		fl.Flush()
	}
}

func (srv *httpServer) webDirHandler(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		wr.WriteHeader(http.StatusMethodNotAllowed)
		wr.Write(webDirFormatBadRequest("Method Not Allowed"))
		return
	}

	reqBody, _ := io.ReadAll(http.MaxBytesReader(wr, r.Body, 1<<20))
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

func checkAllowedMethod(method string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, v := range allowed {
		if v == method {
			return true
		}
	}
	return false
}

/*
THIS IS THE EASIEST YET SECURELY STRONG OAUTHENTICATION MECHANISM

SIGNING: After verifying password

* expiresAt = time.Now().Add(1 day). The expiry time of the token

* shaed = sha256(expiresAt, httpPassword). httpPassword is the checksum

* return hex(shaed) + " " + str(expiresAt)

VERIFYING:

* separate hex(shaed) and str(expiresAt) from "clientCookie"

* check if expiresAt has not expired

* ourCookie = signCookie(expiresAt)

* return ourCookie == clientCookie
*/
func (h *httpServer) signCookie(expiresAt time.Time) string {
	// - Hash the expires time with our password
	// You will know why in verifyCookie
	expiresAtStr := expiresAt.Format(time.RFC3339Nano)
	signer := sha256.New()
	signer.Write([]byte(expiresAtStr))
	shaed := hex.EncodeToString(signer.Sum([]byte(h.clientPassword)))
	return string(shaed) + " " + expiresAtStr
}

func (h *httpServer) verifyCookie(r *http.Request) bool {
	c, _ := r.Cookie(oauthCookieName)
	if c == nil {
		return false
	}

	cookie := c.Value
	var expiresAt time.Time
	var err error

	i := (strings.IndexByte(cookie, ' ') + 1) % len(cookie) // avoid overflow attacks
	expiresAt, err = time.Parse(time.RFC3339Nano, cookie[i:])
	if err != nil || expiresAt.Before(time.Now()) {
		return false
	}

	return bytes.Equal([]byte(h.signCookie(expiresAt)), []byte(cookie))
}
