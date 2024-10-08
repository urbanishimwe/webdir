package main

import (
	"flag"
	"log"
	"net"

	"github.com/urbanishimwe/webdir/node"
)

var (
	addr, mesh, publicAddr, username, password, httpPassword string
)

func init() {
	flag.StringVar(&addr, "addr", "", "Address and port for the node server. If empty, random port is used and server listen on all available address")
	flag.StringVar(&mesh, "mesh", "", "Address of the mesh initiator for registering to the network. If empty this node is the mesh initiator")
	flag.StringVar(&publicAddr, "public-addr", "", "Internet address for this network if not specified node address is used instead")
	flag.StringVar(&username, "name", "", "username of the node, if empty random text are used")
	flag.StringVar(&password, "password", "", "password of the node, if empty random text are used")
	flag.StringVar(&httpPassword, "http-password", "", "password for the client")
	flag.Parse()
}

func main() {
	httpSrv := mustNewHttpServer(addr)
	temp := buildTempNodeConfig(httpSrv)
	httpSrv.node = node.MustInitServer(temp, mesh, webDirMakeHTTPRequest)
	httpSrv.clientPassword = httpPassword
	httpSrv.listenAndServe()
}

func buildTempNodeConfig(srv *httpServer) node.NodeConfig {
	tempConfig := node.NodeConfig{}
	if username != "" {
		tempConfig.Node.Oauth.UserName = username
	}

	if password != "" {
		tempConfig.Node.Oauth.Password = password
	}

	if publicAddr == "" {
		tempConfig.PublicAddr = srv.httpServer.Addr()
	} else if netAddr, err := net.ResolveTCPAddr("", publicAddr); err != nil {
		log.Fatalf("Resovling public address(%s) failed: %q", publicAddr, err)
	} else {
		tempConfig.PublicAddr = netAddr
	}

	return tempConfig
}
