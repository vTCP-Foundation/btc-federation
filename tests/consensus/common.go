package consensus

import (
	"btc-federation/pkg/consensus/events"
	"btc-federation/pkg/consensus/integration"
	"btc-federation/pkg/consensus/mocks"
	"btc-federation/pkg/consensus/types"
	"fmt"
)

func CreateTestNetwork(nodeCount int, tracer events.EventTracer, enableMessageLogging bool) (map[types.NodeID]*integration.Node, *integration.NetworkManager, error) {
	// Create consensus config
	publicKeys := make([]types.PublicKey, nodeCount)
	for i := 0; i < nodeCount; i++ {
		publicKeys[i] = []byte(fmt.Sprintf("public-key-node-%d", i))
	}

	consensusConfig, err := types.NewConsensusConfig(publicKeys)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create consensus config: %w", err)
	}

	// Create validators list
	validators := make([]types.NodeID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		validators[i] = types.NodeID(i)
	}

	// Create shared genesis block
	genesisBlock := types.NewBlock(
		types.BlockHash{},              // No parent
		0,                              // Height 0
		0,                              // View 0
		0,                              // Genesis always proposed by Node 0
		[]byte("shared_genesis_block"), // Deterministic genesis payload
	)

	// Create mock infrastructure for all nodes
	mockNetworks := make(map[types.NodeID]*mocks.MockNetwork)
	mockStorages := make(map[types.NodeID]*mocks.MockStorage)
	mockCryptos := make(map[types.NodeID]*mocks.MockCrypto)

	// Create infrastructure components
	for _, nodeID := range validators {
		// Create mock network
		networkConfig := mocks.DefaultNetworkConfig()
		networkFailures := mocks.DefaultNetworkFailureConfig()
		mockNetwork := mocks.NewMockNetwork(nodeID, networkConfig, networkFailures)
		mockNetwork.SetEventTracer(tracer)
		mockNetworks[nodeID] = mockNetwork

		// Create mock storage
		storageConfig := mocks.DefaultStorageConfig()
		storageFailures := mocks.DefaultStorageFailureConfig()
		mockStorages[nodeID] = mocks.NewMockStorage(storageConfig, storageFailures)

		// Create mock crypto
		cryptoConfig := mocks.DefaultCryptoConfig()
		cryptoFailures := mocks.DefaultCryptoFailureConfig()
		mockCryptos[nodeID] = mocks.NewMockCrypto(nodeID, cryptoConfig, cryptoFailures)
	}

	// Set up full mesh connectivity for networks
	for _, nodeID := range validators {
		mockNetworks[nodeID].SetPeers(mockNetworks)
	}

	// Set up cryptographic trust relationships
	for _, nodeID := range validators {
		for _, otherNodeID := range validators {
			mockCryptos[nodeID].AddNode(otherNodeID)
		}
	}

	// Create nodes with injected dependencies
	nodes := make(map[types.NodeID]*integration.Node)
	for _, nodeID := range validators {
		config := &integration.NodeConfig{
			NodeID:       nodeID,
			Validators:   validators,
			Config:       consensusConfig,
			EventTracer:  tracer,
			GenesisBlock: genesisBlock,
		}

		node, err := integration.NewNode(config, mockNetworks[nodeID], mockStorages[nodeID], mockCryptos[nodeID])
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create node %d: %w", nodeID, err)
		}

		nodes[nodeID] = node
	}

	// Start all nodes
	for nodeID, node := range nodes {
		if err := node.Start(); err != nil {
			return nil, nil, fmt.Errorf("failed to start node %d: %w", nodeID, err)
		}
	}

	// Create network manager for message processing
	networkManager := integration.NewNetworkManager()
	networkManager.EnableLogging(enableMessageLogging)

	for _, node := range nodes {
		networkManager.AddNode(node)
	}

	// Start message processing
	networkManager.StartAll()

	return nodes, networkManager, nil
}
