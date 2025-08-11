package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	node1Cmd *exec.Cmd
	node2Cmd *exec.Cmd
)

func main() {
	// Change to the project root directory
	if err := os.Chdir("/home/hsc/impact/vtcp/btc-federation"); err != nil {
		fmt.Printf("Failed to change directory: %v\n", err)
		return
	}

	fmt.Println("=== Connection Deduplication Verification ===")
	fmt.Println("This will start nodes sequentially to test connection deduplication")
	fmt.Println()

	// Start Node 2 first
	fmt.Println("Starting Node 2...")
	go startNode2()
	time.Sleep(10 * time.Second) // Give Node2 plenty of time to retry and fail again

	fmt.Println("Starting Node 1...")
	go startNode1()
	fmt.Println("Both nodes started. Waiting 15 seconds for connection establishment and deduplication...")
	time.Sleep(15 * time.Second) // Wait for connections to establish

	// Function to check connections
	checkConnections := func() {
		fmt.Println("\n=== Connection State Check ===")

		// Check Node 1 connections (port 9001)
		fmt.Println("Node 1 (port 9001) connections:")
		cmd1 := exec.Command("bash", "-c", "ss -tn | grep :9001 | grep ESTAB")
		output1, _ := cmd1.Output()
		if len(output1) > 0 {
			fmt.Printf("✅ %s", string(output1))
		} else {
			fmt.Println("❌ No established connections found for Node 1")
		}

		// Check Node 2 connections (port 9002)
		fmt.Println("Node 2 (port 9002) connections:")
		cmd2 := exec.Command("bash", "-c", "ss -tn | grep :9002 | grep ESTAB")
		output2, _ := cmd2.Output()
		if len(output2) > 0 {
			fmt.Printf("✅ %s", string(output2))
		} else {
			fmt.Println("❌ No established connections found for Node 2")
		}

		// Count total connections between the nodes
		fmt.Println("\nTotal connections between 127.0.0.1:9001 and 127.0.0.1:9002:")
		cmd3 := exec.Command("bash", "-c", "ss -tn | grep ESTAB | grep -E '(127.0.0.1:9001.*127.0.0.1:9002|127.0.0.1:9002.*127.0.0.1:9001)'")
		output3, _ := cmd3.Output()
		if len(output3) > 0 {
			outputStr := strings.TrimSpace(string(output3))
			connectionCount := len(strings.Split(outputStr, "\n"))

			fmt.Printf("Connection count: %d\n", connectionCount)
			if connectionCount == 1 {
				fmt.Println("✅ SUCCESS: Exactly 1 connection (deduplication working!)")
			} else {
				fmt.Printf("⚠️  WARNING: Found %d connections (expected 1)\n", connectionCount)
			}
			fmt.Printf("Details:\n%s\n", outputStr)
		} else {
			fmt.Println("❌ No connections found between the nodes")
		}

		// Also show all TCP connections for debugging
		fmt.Println("\nAll TCP connections on ports 9001-9002 (for debugging):")
		cmd4 := exec.Command("bash", "-c", "ss -tn | grep -E ':(900[12])' || echo 'No connections on ports 9001-9002'")
		output4, _ := cmd4.Output()
		fmt.Printf("%s", string(output4))
	}

	// Check connections multiple times
	checkConnections()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Waiting 10 more seconds for Node2 retry and deduplication...")
	time.Sleep(10 * time.Second)
	checkConnections()

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Final check after 25 seconds total:")
	checkConnections()

	fmt.Println("\n=== Analysis ===")
	fmt.Println("Expected sequence:")
	fmt.Println("1. ✅ Node2 starts, tries to connect to Node1 (fails - Node1 not ready)")
	fmt.Println("2. ✅ Node1 starts, connects to Node2 (succeeds)")
	fmt.Println("3. ✅ Node2 retries connection, should detect existing connection")
	fmt.Println("4. ✅ Deduplication should prevent second connection")
	fmt.Println("5. ✅ Final result: exactly 1 TCP connection")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Nodes are still running. Press ENTER to stop them and exit...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadLine()

	// Cleanup
	if node1Cmd != nil && node1Cmd.Process != nil {
		node1Cmd.Process.Kill()
	}
	if node2Cmd != nil && node2Cmd.Process != nil {
		node2Cmd.Process.Kill()
	}
}

func startNode2() {
	node2Cmd = exec.Command("./btc-federation", "-config", "conf.yaml")
	node2Cmd.Dir = "bin/node2"

	// Capture output from Node 2
	node2Stdout, _ := node2Cmd.StdoutPipe()
	node2Stderr, _ := node2Cmd.StderrPipe()

	if err := node2Cmd.Start(); err != nil {
		fmt.Printf("Failed to start Node 2: %v\n", err)
		return
	}

	// Start reading Node 2 output in background
	go func() {
		scanner := bufio.NewScanner(node2Stdout)
		for scanner.Scan() {
			fmt.Printf("Node2: %s\n", scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(node2Stderr)
		for scanner.Scan() {
			fmt.Printf("Node2: %s\n", scanner.Text())
		}
	}()

	// Wait for the process to complete
	node2Cmd.Wait()
}

func startNode1() {
	node1Cmd = exec.Command("./btc-federation", "-config", "conf.yaml")
	node1Cmd.Dir = "bin/node1"

	// Capture output from Node 1
	node1Stdout, _ := node1Cmd.StdoutPipe()
	node1Stderr, _ := node1Cmd.StderrPipe()

	if err := node1Cmd.Start(); err != nil {
		fmt.Printf("Failed to start Node 1: %v\n", err)
		return
	}

	// Start reading Node 1 output in background
	go func() {
		scanner := bufio.NewScanner(node1Stdout)
		for scanner.Scan() {
			fmt.Printf("Node1: %s\n", scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(node1Stderr)
		for scanner.Scan() {
			fmt.Printf("Node1: %s\n", scanner.Text())
		}
	}()

	// Wait for the process to complete
	node1Cmd.Wait()
}
