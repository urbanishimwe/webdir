# Webdir

A protocol for a decentralized online directory.

To understand the protocol and code in `\node` please visit [Specification](https://github.com/urbanishimwe/webdir/blob/main/WebDir.md)

Compile by running `go build ./cmd/main/`

You can see supported flags by `./main -h`

Nodes that are not mesh initiator are started with
```
./main -mesh="mesh_address"
```

To add custom mesh server addr instead of using randomly generated port
```
./main -addr=":8080"
```

To configure public address instead of using automatic generated address
```
./main -public-addr="node_address"
```

Note: configuring `public-addr` does not also configure `addr`. The latter needs to be configured separately.

## Node internal design

This is a `Full Mesh Network`.
Object `node.NodeConfig` holds all data required by the node.

The mesh initiator, `ping` all nodes to see if any has dropped from the network. This is only done by the mesh initiator.

Once every node make an internal change to `Record`, a signal is made to send the updates info to all other nodes.

Available path:
- POST:/wedir  **A special used by nodes communication**
- GET:/record  **Get all record**
- GET:/dir  **Get directory**
- GET:/nodes   **Get online nodes**
- GET:/file?file=filename  **Read a file**
- POST:/file?file=filename **Create a file**
- PUT:/file?file=filename  **Update a file content**
- DELETE:/file?file=filename **Delete a file**
