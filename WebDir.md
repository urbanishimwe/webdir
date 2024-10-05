# WebDir

A protocol for a decentralized online directory.

## SUMMARY

WebDir is a mesh network where each node contributes a portion of its storage resources to form a shared online directory.  
Each node on the network can perform CRUD operation on any resources shared on the network and updates will be live-communicated.  
When a node creates a file, it automatically becomes the owner of that file.  
The owner of the file maintains all updates to that file and once disconnected that file is no longer available to the network. Nodes keep a copy of the virtual directory (A JSON representation of the overall directory).

## Managing the Network

**The Network Initiator**(also a node) helps new-joining nodes to fetch the IPs of other nodes, so its IP should be known in advance. It also helps to identify nodes that left the network by sending a **PING** at certain intervals. A new node has to **sign up** to join the network by providing a unique **username** and a **password** at **CodeRegister** to the **mesh initiator**.  
Every node keeps a copy of the following records on the network locally:

* Record of **online nodes**(*used to authenticate a nodes for every communication*):  
  ```json  
  {  
     "online_nodes":{  
        "nodes_list":{  
           "node_user_name":{  
              "oauth":{  
                 "user_name":"unique_node_identifier",  
                 "password":"password"  
              },  
              "address":"node_public_address"  
           }  
        },  
        "recent_update":{  
           "by":"node_username",  
           "at":"RFC3339Nano_time_format",  
           "update_type":"””"  
        }  
     }  
  }  
  ```  
* Record of **virtual directory** representation:  
  ```json  
  {  
     "directory":{  
        "files_list":{  
           "file_name":{  
              "name":"file_name",  
              "owner":"node_username",  
              "created_at":"RFC3339Nano_time_format",  
              "recent_update":{  
                 "by":"node_username",  
                 "at":"RFC3339Nano_time_format",  
                 "content":""  
              }  
           }  
        },  
        "recent_update":{  
           "by":"node_username",  
           "at":"RFC3339Nano_time_format",  
           "content":""  
        }  
     }  
  }  
  ```

As you can see the entire record can be represented as:  
```json  
{  
   "online_nodes":{},  
   "directory":{}  
}  
```  
**The entire JSON is shared with a newly-joining node**.

## Communication on the Network

Each node acts both as **a server** and **a client** on the network. This allows the aliveness of every part(admin, nodes) without leaking local file descriptors.  
Implementations of this protocol should facilitate connection through any medium **TCP/IP, HTTP,** or **UDP/IP** connection.  
This is a full mesh network protocol but users may choose to implement it however they want. A node keeps a running thread to update the list of linked nodes depending on the ***connection score**.* Nodes communicate by using Message code.

## Message Format

All communications is sent with the following format:  
```json  
{  
   "header":{  
      "node":{  
         "oauth":{  
            "user_name":"",  
            "password":""  
         },  
         "address":""  
      },  
      "destination":"destination_node_username"  
   },  
   "body":{  
      "code":0,  
      "status":"",  
      "content":""  
   }  
}  
```

## Message Codes

Below is a list of possible Message codes:

| Code | Action |
| :---- | :---- |
| CodeResponse | Response |
| CodeGetInfo | Request info |
| CodeNodes | Send or Request List of online nodes |
| CodeDirectory | Send or Request directory representation |
| CodeUpdate | Send made updates to other nodes on the network |
| CodeCreateFile | Informing a created file |
| CodeReadFile | Request data of a file |
| CodeUpdateFile | Update a file|
| CodeDeleteFile | Delete a file|
| CodeRegister | Registering on the network |
| CodeDrop | Node has dropped off the network |

## Response status

| Status | Explanation |
| :---- | :---- |
| StatusOk | OK |
| StatusNotOauth | Node Not Authorized |
| StatusBadFormat | Message Bad Format |
| StatusInternalError | Internal Error |
| StatusNodeNotOnline | Node Not Online |
| StatusNodeExist | Node Exist |
| StatusFileExist | File Exist |
| StatusFileNotFound | File Not Found |
| StatusFileUpdateOld | File Update Old |

## CodeUpdate

Code update is used to publish any updates made. The contents of the update goes inside `body.cotent` of the message with “code: CodeUpdate”. This is the format of the content.  
```json  
{  
   "content":{  
      "at":"time_formatted",  
      "by":"node_user_name",  
      "content":"content of the update",  
      "code":"type of update made"  
   }  
}  
```

## CUD(Create, Update, Delete) Operation with CodeUpdate

Update are published using the `body.content.content` on `CodeUpdate` with the following format which is of type file.  
```json  
{  
   "name":"file_name",  
   "owner":"node_username",  
   "created_at":"RFC3339Nano_time_format",  
   "recent_update":{  
      "by":"node_username",  
      "at":"RFC3339Nano_time_format",  
      "content":""  
   }  
}  
```