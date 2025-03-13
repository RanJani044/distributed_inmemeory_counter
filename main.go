package main

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Node struct {
	ID          string
	Peers       map[string]*Node
	Mutex       sync.Mutex
	LastPing    time.Time
	FailedPeers map[string]int
	Counter     *Counter
}

func NewNode(id string) *Node {
	counter := NewCounter(nil)
	return &Node{
		ID:          id,
		Peers:       make(map[string]*Node),
		FailedPeers: make(map[string]int),
		Counter:     counter,
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
		time.Sleep(10 * time.Second)
		node.Mutex.Lock()

		if len(node.Peers) == 0 {
			fmt.Println("No peers to check.")
			node.Mutex.Unlock()
			continue
		}
		// Make a snapshot of peers list to avoid modifying the map while iterating
		peersCopy := make([]*Node, 0, len(node.Peers))
		for _, peer := range node.Peers {
			if peer != nil {
				peersCopy = append(peersCopy, peer)

			}
		}
		node.Mutex.Unlock()

		// Check the status of each peer in the snapshot
		for _, peer := range peersCopy {
			if peer == nil {
				fmt.Println("Warning: Peer is nil, skipping.")
				continue
			}
			go func(p *Node) {
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("Recovered from panic in heartbeat for peer %s: %v\n", p.ID, r)
					}
				}()
				node.checkPeerStatus(p)
			}(peer)
		}
	}
}

func (node *Node) checkPeerStatus(peer *Node) {
	if peer == nil {
		fmt.Println("Warning: Peer is nil, skipping.")
		return
	}

	peerAddress := peer.ID
	if !strings.Contains(peerAddress, "localhost") {
		peerAddress = "localhost:" + peerAddress
	}

	url := "http://" + peerAddress + "/health"
	fmt.Println("Checking peer health at:", url)

	resp, err := http.Get(url)

	if err != nil {
		fmt.Printf("Error checking peer health: %v\n", err)
	}

	var maxretry = 3
	if resp == nil {
		fmt.Printf("Peer  is unresponsive. Removing from peers.\n")
		node.Mutex.Lock()
		if _, exists := node.Peers[peer.ID]; exists {
			delete(node.Peers, peer.ID)
		}
		node.Mutex.Unlock()
		node.retryFailedPeer(peer, maxretry)

		//return
	} else {
		fmt.Printf("Peer %s is healthy!\n", peer.ID)
	}

}

func (node *Node) retryFailedPeer(peer *Node, maxRetries int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic after retries: %v\n", r)
		}
	}()
	// Retry logic with a maximum of 2 attempts
	attempts := 1
	for attempts <= maxRetries {
		time.Sleep(time.Duration(math.Pow(2, float64(attempts))) * time.Second)
		fmt.Printf("Retrying peer %s (attempt %d)...\n", peer.ID, attempts)

		peerAddress := peer.ID
		if !strings.Contains(peerAddress, "localhost") {
			peerAddress = "localhost:" + peerAddress
		}
		url := "http://" + peerAddress + "/health"
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Printf("Peer %s successfully reconnected!\n", peer.ID)
			return
		}
		attempts++
	}

	// After maxRetries, peer is still unresponsive, so remove it and trigger count
	fmt.Printf("Peer %s failed to reconnect after %d attempts. Removing from peers.\n", peer.ID, maxRetries)
	node.Mutex.Lock()
	if _, exists := node.Peers[peer.ID]; exists {
		delete(node.Peers, peer.ID)
	}
	node.Mutex.Unlock()

	// Trigger count update after deleting the peer
	currentCount := node.Counter.GetCount()
	fmt.Printf("Peer  removed. Current counter value: %d\n", currentCount)
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
	if peer == nil {
		fmt.Println("Warning: Peer is nil, skipping.")
		return
	}
	peerAddress := peer.ID
	if !strings.Contains(peerAddress, "localhost") {
		peerAddress = "localhost:" + peerAddress
	}

	url := "http://" + peerAddress + "/increment"
	resp, err := http.Post(url, "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to propagate increment to peer %s: %v\n", peer.ID, err)
		time.Sleep(2 * time.Second)
		counter.propagateIncrement(peer)
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

	fmt.Println("Starting server on port", port)
	http.ListenAndServe(":"+port, nil)
}

func (counter *Counter) IncrementHandler(w http.ResponseWriter, r *http.Request) {
	counter.Increment()
	w.Write([]byte("Incremented successfully"))
}

func (counter *Counter) GetCountHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf("Current Counter: %d", counter.GetCount())))
}

func (counter *Counter) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func registerNewNode(existingNode *Node, newNodePort string) {
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

	counter := NewCounter(node)

	go counter.StartAPI(*port)

	go node.Heartbeat()

	// If a new node is provided, register it dynamically
	if *newNodePort != "" {
		// Create a new node and register it to the existing cluster
		newNode := NewNode(*newNodePort)
		go newNode.Heartbeat()

		// Register new node with the existing cluster
		registerNewNode(node, *newNodePort)

		handleNewPeer(counter, *newNodePort)
	}

	// Keep the main goroutine running
	select {}
}

func handleNewPeer(counter *Counter, newPeerID string) {
	// Add the new peer dynamically
	counter.Node.Mutex.Lock()
	counter.Node.Peers[newPeerID] = &Node{ID: newPeerID}
	counter.Node.Mutex.Unlock()

	// Increment the counter when a new peer joins
	fmt.Printf("New peer added: %s\n", newPeerID)
	counter.Increment()
}
