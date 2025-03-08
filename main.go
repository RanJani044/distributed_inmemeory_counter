package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Node struct {
	ID       string
	Peers    map[string]*Node
	Mutex    sync.Mutex
	LastPing time.Time
}

func NewNode(id string) *Node {
	return &Node{
		ID:    id,
		Peers: make(map[string]*Node),
	}
}

func (node *Node) RegisterPeers(peers []string) {
	// Register all new peers that are not already in the list
	for _, peer := range peers {
		if peer != node.ID {
			node.Peers[peer] = &Node{ID: peer}
		}
	}
}

func (node *Node) Heartbeat() {
	// Periodically check the status of all peers
	time.Sleep(30 * time.Second)
	for {
		time.Sleep(10 * time.Second) // Heartbeat interval
		node.Mutex.Lock()
		fmt.Println("checking the heart beat")
		for _, peer := range node.Peers {
			go node.checkPeerStatus(peer)
		}
		node.Mutex.Unlock()
	}
}

func (node *Node) checkPeerStatus(peer *Node) {
	if peer == nil {
		fmt.Println("Warning: Peer is nil, skipping.")
		return
	}

	// Construct correct URL
	peerAddress := peer.ID
	if !strings.Contains(peerAddress, "localhost") {
		peerAddress = "localhost:" + peerAddress
	}

	url := "http://" + peerAddress + "/health"
	fmt.Println("Checking peer health at:", url)

	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		// If the peer is unresponsive, remove it from the peer list
		fmt.Printf("Peer %s is unresponsive (StatusCode: %d). Removing from peers.\n", peer.ID, resp.StatusCode)
		node.Mutex.Lock()
		delete(node.Peers, peer.ID)
		node.Mutex.Unlock()
	} else {
		fmt.Printf("Peer %s is healthy!\n", peer.ID)
	}
}

func (node *Node) GetPeers() []string {
	// Return a list of peer IDs
	node.Mutex.Lock()
	defer node.Mutex.Unlock()

	peers := []string{}
	for peer := range node.Peers {
		peers = append(peers, peer)
	}
	return peers
}

type Counter struct {
	Value int
	Mutex sync.Mutex
	Node  *Node
}

func NewCounter(node *Node) *Counter {
	return &Counter{
		Value: 0,
		Node:  node,
	}
}

func (counter *Counter) Increment() {
	// Increment the counter for the current node
	counter.Mutex.Lock()
	defer counter.Mutex.Unlock()
	counter.Value++
	fmt.Println("Counter incremented:", counter.Value)
	var wg sync.WaitGroup
	// Propagate the increment to peers
	for _, peer := range counter.Node.Peers {
		wg.Add(1)
		go counter.propagateIncrement(peer)
	}
	wg.Wait()
}

func (counter *Counter) propagateIncrement(peer *Node) {
	// Check if peer.ID contains "localhost:" or just the port
	peerAddress := peer.ID // This should already be "localhost:port"
	if !strings.Contains(peerAddress, "localhost") {
		// If it's just a port number, prepend localhost:
		peerAddress = "localhost:" + peerAddress
	}

	// Now the correct URL is formed
	url := "http://" + peerAddress + "/increment"
	fmt.Println("Checking URL:", url)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to propagate increment to peer %s: %v\n", peer.ID, err)
	} else {
		fmt.Printf("Successfully propagated increment to peer %s\n", peer.ID)
	}
}

func (counter *Counter) GetCount() int {
	return counter.Value
}

func (counter *Counter) StartAPI(port string) {
	// Start the HTTP server to handle requests
	http.HandleFunc("/increment", counter.IncrementHandler)
	http.HandleFunc("/count", counter.GetCountHandler)
	http.HandleFunc("/health", counter.HealthCheck)

	// Start the HTTP server
	fmt.Println("Starting server on port", port)
	http.ListenAndServe(":"+port, nil)
}

func (counter *Counter) IncrementHandler(w http.ResponseWriter, r *http.Request) {
	// Handle the increment request
	counter.Increment()
	w.Write([]byte("Incremented successfully"))
}

func (counter *Counter) GetCountHandler(w http.ResponseWriter, r *http.Request) {
	// Return the current counter value
	w.Write([]byte(fmt.Sprintf("Current Counter: %d", counter.GetCount())))
}

func (counter *Counter) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Health check to ensure the node is alive
	w.Write([]byte("OK"))
}

func registerNewNode(existingNode *Node, newNodePort string) {
	// Register a new node with the existing node
	fmt.Printf("Registering new node %s to existing node %s\n", newNodePort, existingNode.ID)
	existingNode.RegisterPeers([]string{newNodePort})
}

func main() {
	// Parse command-line arguments
	port := flag.String("port", "", "Port for the node to run on")
	peers := flag.String("peers", "", "Comma-separated list of initial peer nodes")
	newNodePort := flag.String("newnode", "", "Port of the new node to add dynamically")
	flag.Parse()

	// Initialize node
	nodeID := *port
	node := NewNode(nodeID)

	// Register initial peers if provided
	if *peers != "" {
		initialPeers := strings.Split(*peers, ",")
		node.RegisterPeers(initialPeers)
	}

	// Create counter for the node
	counter := NewCounter(node)

	// Start API server for counter operations
	go counter.StartAPI(*port)

	// Start heartbeat mechanism
	go node.Heartbeat()

	// If a new node is provided, register it dynamically
	if *newNodePort != "" {
		// Create a new node and register it to the existing cluster
		newNode := NewNode(*newNodePort)
		//counterNewNode := NewCounter(newNode)
		go newNode.Heartbeat()

		// Register new node with the existing cluster
		registerNewNode(node, *newNodePort)

		// Handle the new peer addition and propagate the counter increment
		handleNewPeer(counter, *newNodePort)
	}

	// Keep the main goroutine running
	select {}
}

// Function to handle a new peer being added dynamically
func handleNewPeer(counter *Counter, newPeerID string) {
	// Add the new peer dynamically
	counter.Node.Mutex.Lock()
	counter.Node.Peers[newPeerID] = &Node{ID: newPeerID}
	counter.Node.Mutex.Unlock()

	// Increment the counter when a new peer joins
	fmt.Printf("New peer added: %s\n", newPeerID)
	counter.Increment() // Increment the counter and propagate the change
}
