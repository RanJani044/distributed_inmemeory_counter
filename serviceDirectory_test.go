package main

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestConcurrentIncrementsOnSingleNode(t *testing.T) {
	node := NewNode("localhost:8080")
	counter := NewCounter(node)
	go counter.StartAPI("8080")

	// Test concurrent increments
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Increment()
		}()
	}
	wg.Wait()

	// Validate the final count
	if counter.GetCount() != 10 {
		t.Errorf("Expected counter to be 10, but got %d", counter.GetCount())
	}
}

func TestIncrementPropagationBetweenNodes(t *testing.T) {
	// Simulate two nodes with different ports
	port1 := "8080"
	port2 := "8081"

	// Create Node instances
	node1 := NewNode("localhost:" + port1)
	node2 := NewNode("localhost:" + port2)

	// Create Counter instances
	counter1 := NewCounter(node1)
	counter2 := NewCounter(node2)

	// Register node2 as a peer of node1 and vice versa
	node1.RegisterPeers([]string{"localhost:" + port2})
	node2.RegisterPeers([]string{"localhost:" + port1})

	// Start the API servers for both nodes on different ports
	go counter1.StartAPI(port1)
	go counter2.StartAPI(port2)

	// Give the servers some time to start up (this may need to be adjusted)
	time.Sleep(2 * time.Second)

	// Perform an increment on node1 and propagate to node2
	counter1.Increment()

	// Wait for propagation to complete
	time.Sleep(1 * time.Second)

	// Validate the counter values across nodes
	if counter1.GetCount() != 1 {
		t.Errorf("Expected node1 counter to be 1, but got %d", counter1.GetCount())
	}
	if counter2.GetCount() != 1 {
		t.Errorf("Expected node2 counter to be 1, but got %d", counter2.GetCount())
	}
}

// Modified TestIncrementPropagationBetweenNodes to fix server conflict
// func TestIncrementPropagationBetweenNodes(t *testing.T) {
// 	// Simulate two nodes with different ports
// 	node1 := NewNode("localhost:8081")
// 	node2 := NewNode("localhost:8082")
// 	counter1 := NewCounter(node1)
// 	counter2 := NewCounter(node2)

// 	// Register node2 as a peer of node1
// 	node1.RegisterPeers([]string{"localhost:8082"})
// 	node2.RegisterPeers([]string{"localhost:8081"})

// 	// Use a WaitGroup to ensure the servers are started before continuing
// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	// Start the API servers for both nodes in separate goroutines
// 	go func() {
// 		defer wg.Done()
// 		counter1.StartAPI("8081")

// 	}()

// 	go func() {
// 		defer wg.Done()
// 		counter2.StartAPI("8082")
// 	}()

// 	// Wait for both servers to start
// 	wg.Wait()

// 	// Increment on node1 and propagate to node2
// 	counter1.Increment()

// 	// Wait for propagation to complete
// 	time.Sleep(1 * time.Second)

// 	// Validate the counter values across nodes
// 	if counter1.GetCount() != 1 {
// 		t.Errorf("Expected node1 counter to be 1, but got %d", counter1.GetCount())
// 	}
// 	if counter2.GetCount() != 1 {
// 		t.Errorf("Expected node2 counter to be 1, but got %d", counter2.GetCount())
// 	}
// }

// func TestDeduplicationAndRetryHandling(t *testing.T) {
// 	// Simulate a node and its peer with failure during propagation
// 	node1 := NewNode("localhost:8080")
// 	node2 := NewNode("localhost:8081")
// 	counter1 := NewCounter(node1)
// 	counter2 := NewCounter(node2)

// 	// Register node2 as a peer of node1
// 	node1.RegisterPeers([]string{"localhost:8081"})
// 	node2.RegisterPeers([]string{"localhost:8080"})

// 	// Start the API servers for both nodes
// 	go counter1.StartAPI("8080")
// 	go counter2.StartAPI("8081")

// 	// Mock the peer server to simulate failure and retry
// 	originalHealthCheck := counter2.HealthCheck
// 	counter2.HealthCheck = func(w http.ResponseWriter, r *http.Request) {
// 		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
// 	}

// 	// Increment and try to propagate to peer
// 	counter1.Increment()

// 	// Simulate retry after failure
// 	time.Sleep(1 * time.Second)

// 	// Recover from failure
// 	counter2.HealthCheck = originalHealthCheck

// 	// Validate retry and propagation
// 	if counter1.GetCount() != 1 {
// 		t.Errorf("Expected node1 counter to be 1, but got %d", counter1.GetCount())
// 	}
// 	if counter2.GetCount() != 1 {
// 		t.Errorf("Expected node2 counter to be 1, but got %d", counter2.GetCount())
// 	}
// }

func TestClusterRebalancing(t *testing.T) {
	// Simulate a cluster with 3 nodes
	node1 := NewNode("localhost:8080")
	node2 := NewNode("localhost:8081")
	node3 := NewNode("localhost:8082")

	counter1 := NewCounter(node1)
	counter2 := NewCounter(node2)
	counter3 := NewCounter(node3)

	// Register peers among nodes
	node1.RegisterPeers([]string{"localhost:8081", "localhost:8082"})
	node2.RegisterPeers([]string{"localhost:8080", "localhost:8082"})
	node3.RegisterPeers([]string{"localhost:8080", "localhost:8081"})

	// Start API servers for all nodes
	go counter1.StartAPI("8080")
	go counter2.StartAPI("8081")
	go counter3.StartAPI("8082")

	// Increment on node1
	counter1.Increment()

	// Wait for propagation
	time.Sleep(1 * time.Second)

	// Validate that all nodes have the same counter value
	if counter1.GetCount() != 1 {
		t.Errorf("Expected node1 counter to be 1, but got %d", counter1.GetCount())
	}
	if counter2.GetCount() != 1 {
		t.Errorf("Expected node2 counter to be 1, but got %d", counter2.GetCount())
	}
	if counter3.GetCount() != 1 {
		t.Errorf("Expected node3 counter to be 1, but got %d", counter3.GetCount())
	}

	// Simulate node3 leaving
	node3.Mutex.Lock()
	delete(node3.Peers, "localhost:8080")
	delete(node3.Peers, "localhost:8081")
	node3.Mutex.Unlock()

	// Increment on node1
	counter1.Increment()

	// Wait for propagation (node3 should no longer get the increment)
	time.Sleep(1 * time.Second)

	// Validate that node1 and node2 have the updated count, but node3 does not
	if counter1.GetCount() != 2 {
		t.Errorf("Expected node1 counter to be 2, but got %d", counter1.GetCount())
	}
	if counter2.GetCount() != 2 {
		t.Errorf("Expected node2 counter to be 2, but got %d", counter2.GetCount())
	}
	if counter3.GetCount() != 1 {
		t.Errorf("Expected node3 counter to remain 1, but got %d", counter3.GetCount())
	}
}

func TestHealthCheck(t *testing.T) {
	// Setup mock servers for testing health check
	node := NewNode("localhost:9097")
	counter := NewCounter(node)
	go counter.StartAPI("9097")

	// Test health check endpoint
	resp, err := http.Get("http://localhost:9097/health")
	if err != nil {
		t.Fatalf("Failed to send health check request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected health check to return 200 OK, but got %v", resp.StatusCode)
	}
}
