 Service Directory

- This project implements a distributed counter system where multiple nodes increment a shared counter and propagate the increment operation to other nodes in the system.
- It provides eventual consistency by ensuring that all nodes eventually converge to the same counter value.


 Design Decisions

 Service Discovery:
- Service discovery is achieved through peer registration, where each node maintains a list of peers (other nodes in the system).
- When a node is initialized, it can register other nodes as its peers. This allows for communication between nodes via HTTP to propagate counter increments.
- Each node maintains a Peers map, which stores the nodes' addresses.
- Periodically, each node performs a "heartbeat" operation to check the health status of its peers.
- This is done by sending HTTP requests to the /health endpoint of each peer. If a peer fails to respond or returns a non-OK status, the node will remove that peer from its list.
- Nodes are registered dynamically during initialization and can be added or removed through RegisterPeers.

Eventual Consistency:
- Eventual consistency is achieved by propagating increments to all registered peers whenever a counter increment is performed. This ensures that all nodes eventually have the same counter value.
- When a node's counter is incremented, it sends an HTTP POST request to the /increment endpoint of all its peers to propagate the increment.
- The system does not guarantee strict consistency at any given time but ensures that all nodes will eventually converge to the same counter value, given enough time.

Handling Network Partitions:
- In case of a network partition (i.e., a node cannot communicate with its peers), the system will eventually converge once the partition is resolved.
- The "heartbeat" mechanism ensures that peers are periodically checked for their health. If the partition is resolved, the system will propagate the missing increments once connectivity is restored.
- The design doesn't provide strong consistency guarantees, so nodes may temporarily diverge in their counter values during a network partition.
- However, once the partition is resolved, the counter values will eventually become consistent.

Instructions to Run Nodes and Tests:
- To start a node, you can specify the port and initial peers by running the following command:
    go run main.go -port=<port_number> -peers=<comma_separated_list_of_peer_ports>
    Example:
    go run main.go -port=8080 -peers=8081,8082
- This starts a node on port 8080 and registers nodes running on ports 8081 and 8082 as peers.

- If you want to add a new node dynamically to the system, you can specify the -newnode flag along with the port of the new node:
    go run main.go -port=8080 -peers=8081,8082 -newnode=8083
- This will start a node on port 8080, register peers at ports 8081 and 8082, and dynamically add a new node at port 8083.
