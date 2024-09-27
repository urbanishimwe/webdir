package main

import (
	"flag"
	"log"
	"net"

	"github.com/urbanishimwe/webdir/node"
)

var (
	addr, mesh, publicAddr, username, password string
)

func init() {
	flag.StringVar(&addr, "addr", "", "Address and port for the node server. If empty, random port is used and server listen on all available address")
	flag.StringVar(&mesh, "mesh", "", "Address of the mesh initiator for registering to the network. If empty this node is the mesh initiator")
	flag.StringVar(&publicAddr, "public-addr", "", "Internet address for this network if not specified node address is used instead")
	flag.StringVar(&username, "name", "", "username of the node, if empty random text are used")
	flag.StringVar(&password, "password", "", "password of the node, if empty random text are used")
	flag.Parse()
}

func main() {
	temp := buildTempNodeConfig()
	httpSrv := mustNewHttpServer(nil, addr)
	if publicAddr == "" {
		temp.PublicAddr = httpSrv.httpServer.Addr()
	} else if netAddr, err := net.ResolveIPAddr("", publicAddr); err != nil {
		log.Fatalf("Resovling public address(%s) failed: %q", publicAddr, err)
	} else {
		temp.PublicAddr = netAddr
	}
	httpSrv.node = node.MustInitServer(temp, mesh, webDirMakeHTTPRequest)
	httpSrv.listenAndServe()
}

func buildTempNodeConfig() node.NodeConfig {
	tempConfig := node.NodeConfig{}
	if username != "" {
		tempConfig.Node.Oauth.UserName = username
	}

	if password != "" {
		tempConfig.Node.Oauth.Password = password
	}

	return tempConfig
}
