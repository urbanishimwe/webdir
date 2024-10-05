# Webdir

A protocol for a decentralized online directory.

To understand the protocol and code in `\node` please visit [Specification](https://github.com/urbanishimwe/webdir/blob/main/WebDir.md)

## Node internal design

After you have read the [Specification](https://github.com/urbanishimwe/webdir/blob/main/WebDir.md), You may navigate in `node` directory.

The `node/objects.go` contains public object used in communication as specified in spec.

The `node/handlers.go` handles request between nodes.

The `node/client.go` handles request of the client(HTTP)

The mesh initiator, `pings` all nodes to see if any has dropped from the network. This is only done by the mesh initiator.

Once a node make an internal change to `Record`, a signal is made to send the updates channel which the updates all other nodes.

## Example HTTP server

It is implemented in `./cnd/main/`

Compile: `go build -o $exec-name ./cmd/main/`

Check available flags: `./$exec-name -h`

Nodes that are not mesh initiator must have the `-mesh` flag set
```
./$exec-name -mesh="mesh_address"
```

To add custom node server addr instead of using randomly generated port
```
./$exec-name  -addr=":8080"
```

To configure public address(for communication between nodes). The default is HTTP server address
```
./$exec-name -public-addr="node_address"
```
Note: configuring `public-addr` does not also configure `addr`. The latter needs to be configured separately.

Available path:

- POST: /wedir  **A special route used only between nodes communication**

- POST: /login **Client login if `-http-password` was set. It expect password as a plain text inside the request body**

- GET: /login **returns the login page but also delete oauth cookie(in case of logout)**

- GET: / **Home page**

- GET: /record  **Get all record**

- GET: /dir  **Get directory**

- GET: /nodes   **Get online nodes**

- GET: /file?name=filename  **Read a file**

- POST: /file?name=filename **Create a file**

- PUT: /file?name=filename  **Update a file content**

- PATCH: /file?name=filename  **Update a file content**

- DELETE: /file?name=filename **Delete a file**
