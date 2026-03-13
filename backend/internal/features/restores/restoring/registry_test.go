package restoring

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	cache_utils "databasus-backend/internal/util/cache"
	"databasus-backend/internal/util/logger"
)

func Test_HearthbeatNodeInRegistry_RegistersNodeWithTTL(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	defer cleanupTestNode(registry, node)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node)
	assert.NoError(t, err)

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, node.ID, nodes[0].ID)
	assert.Equal(t, node.ThroughputMBs, nodes[0].ThroughputMBs)
}

func Test_UnregisterNodeFromRegistry_RemovesNodeAndCounter(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node.ID)
	assert.NoError(t, err)

	err = registry.UnregisterNodeFromRegistry(node)
	assert.NoError(t, err)

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Empty(t, nodes)

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Empty(t, stats)
}

func Test_GetAvailableNodes_ReturnsAllRegisteredNodes(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	node3 := createTestRestoreNode()
	defer cleanupTestNode(registry, node1)
	defer cleanupTestNode(registry, node2)
	defer cleanupTestNode(registry, node3)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node1)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node2)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node3)
	assert.NoError(t, err)

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 3)

	nodeIDs := make(map[uuid.UUID]bool)
	for _, node := range nodes {
		nodeIDs[node.ID] = true
	}
	assert.True(t, nodeIDs[node1.ID])
	assert.True(t, nodeIDs[node2.ID])
	assert.True(t, nodeIDs[node3.ID])
}

func Test_GetAvailableNodes_WhenNoNodesExist_ReturnsEmptySlice(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.NotNil(t, nodes)
	assert.Empty(t, nodes)
}

func Test_IncrementRestoresInProgress_IncrementsCounter(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	defer cleanupTestNode(registry, node)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node.ID)
	assert.NoError(t, err)

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, node.ID, stats[0].ID)
	assert.Equal(t, 1, stats[0].ActiveRestores)

	err = registry.IncrementRestoresInProgress(node.ID)
	assert.NoError(t, err)

	stats, err = registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, 2, stats[0].ActiveRestores)
}

func Test_DecrementRestoresInProgress_DecrementsCounter(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	defer cleanupTestNode(registry, node)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node.ID)
	assert.NoError(t, err)

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Equal(t, 3, stats[0].ActiveRestores)

	err = registry.DecrementRestoresInProgress(node.ID)
	assert.NoError(t, err)

	stats, err = registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Equal(t, 2, stats[0].ActiveRestores)

	err = registry.DecrementRestoresInProgress(node.ID)
	assert.NoError(t, err)

	stats, err = registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Equal(t, 1, stats[0].ActiveRestores)
}

func Test_DecrementRestoresInProgress_WhenNegative_ResetsToZero(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	defer cleanupTestNode(registry, node)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node)
	assert.NoError(t, err)

	err = registry.DecrementRestoresInProgress(node.ID)
	assert.NoError(t, err)

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, 0, stats[0].ActiveRestores)
}

func Test_GetRestoreNodesStats_ReturnsStatsForAllNodes(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	node3 := createTestRestoreNode()
	defer cleanupTestNode(registry, node1)
	defer cleanupTestNode(registry, node2)
	defer cleanupTestNode(registry, node3)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node1)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node2)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node3)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node1.ID)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node2.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node2.ID)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node3.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node3.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node3.ID)
	assert.NoError(t, err)

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Len(t, stats, 3)

	statsMap := make(map[uuid.UUID]int)
	for _, stat := range stats {
		statsMap[stat.ID] = stat.ActiveRestores
	}

	assert.Equal(t, 1, statsMap[node1.ID])
	assert.Equal(t, 2, statsMap[node2.ID])
	assert.Equal(t, 3, statsMap[node3.ID])
}

func Test_GetRestoreNodesStats_WhenNoStats_ReturnsEmptySlice(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Empty(t, stats)
}

func Test_MultipleNodes_RegisteredAndQueriedCorrectly(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node1.ThroughputMBs = 50
	node2 := createTestRestoreNode()
	node2.ThroughputMBs = 100
	node3 := createTestRestoreNode()
	node3.ThroughputMBs = 150
	defer cleanupTestNode(registry, node1)
	defer cleanupTestNode(registry, node2)
	defer cleanupTestNode(registry, node3)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node1)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node2)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node3)
	assert.NoError(t, err)

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 3)

	nodeMap := make(map[uuid.UUID]RestoreNode)
	for _, node := range nodes {
		nodeMap[node.ID] = node
	}

	assert.Equal(t, 50, nodeMap[node1.ID].ThroughputMBs)
	assert.Equal(t, 100, nodeMap[node2.ID].ThroughputMBs)
	assert.Equal(t, 150, nodeMap[node3.ID].ThroughputMBs)
}

func Test_RestoreCounters_TrackedSeparatelyPerNode(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	defer cleanupTestNode(registry, node1)
	defer cleanupTestNode(registry, node2)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node1)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node2)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node1.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node1.ID)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node2.ID)
	assert.NoError(t, err)

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Len(t, stats, 2)

	statsMap := make(map[uuid.UUID]int)
	for _, stat := range stats {
		statsMap[stat.ID] = stat.ActiveRestores
	}

	assert.Equal(t, 2, statsMap[node1.ID])
	assert.Equal(t, 1, statsMap[node2.ID])

	err = registry.DecrementRestoresInProgress(node1.ID)
	assert.NoError(t, err)

	stats, err = registry.GetRestoreNodesStats()
	assert.NoError(t, err)

	statsMap = make(map[uuid.UUID]int)
	for _, stat := range stats {
		statsMap[stat.ID] = stat.ActiveRestores
	}

	assert.Equal(t, 1, statsMap[node1.ID])
	assert.Equal(t, 1, statsMap[node2.ID])
}

func Test_GetAvailableNodes_SkipsInvalidJsonData(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	defer cleanupTestNode(registry, node)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), registry.timeout)
	defer cancel()

	invalidKey := nodeInfoKeyPrefix + uuid.New().String() + nodeInfoKeySuffix
	registry.client.Do(
		ctx,
		registry.client.B().Set().Key(invalidKey).Value("invalid json data").Build(),
	)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), registry.timeout)
		defer cleanupCancel()
		registry.client.Do(cleanupCtx, registry.client.B().Del().Key(invalidKey).Build())
	}()

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, node.ID, nodes[0].ID)
}

func Test_PipelineGetKeys_HandlesEmptyKeysList(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()

	keyDataMap, err := registry.pipelineGetKeys([]string{})
	assert.NoError(t, err)
	assert.NotNil(t, keyDataMap)
	assert.Empty(t, keyDataMap)
}

func Test_HearthbeatNodeInRegistry_UpdatesLastHeartbeat(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	originalHeartbeat := node.LastHeartbeat
	defer cleanupTestNode(registry, node)

	time.Sleep(10 * time.Millisecond)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node)
	assert.NoError(t, err)

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.True(t, nodes[0].LastHeartbeat.After(originalHeartbeat))
}

func Test_HearthbeatNodeInRegistry_RejectsZeroTimestamp(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()

	err := registry.HearthbeatNodeInRegistry(time.Time{}, node)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zero heartbeat timestamp")

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 0)
}

func Test_GetAvailableNodes_ExcludesStaleNodesFromCache(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	node3 := createTestRestoreNode()
	defer cleanupTestNode(registry, node1)
	defer cleanupTestNode(registry, node2)
	defer cleanupTestNode(registry, node3)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node1)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node2)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node3)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), registry.timeout)
	defer cancel()

	key := fmt.Sprintf("%s%s%s", nodeInfoKeyPrefix, node2.ID.String(), nodeInfoKeySuffix)
	result := registry.client.Do(ctx, registry.client.B().Get().Key(key).Build())
	assert.NoError(t, result.Error())

	data, err := result.AsBytes()
	assert.NoError(t, err)

	var node RestoreNode
	err = json.Unmarshal(data, &node)
	assert.NoError(t, err)

	node.LastHeartbeat = time.Now().UTC().Add(-3 * time.Minute)
	modifiedData, err := json.Marshal(node)
	assert.NoError(t, err)

	setCtx, setCancel := context.WithTimeout(context.Background(), registry.timeout)
	defer setCancel()
	setResult := registry.client.Do(
		setCtx,
		registry.client.B().Set().Key(key).Value(string(modifiedData)).Build(),
	)
	assert.NoError(t, setResult.Error())

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 2)

	nodeIDs := make(map[uuid.UUID]bool)
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	assert.True(t, nodeIDs[node1.ID])
	assert.False(t, nodeIDs[node2.ID])
	assert.True(t, nodeIDs[node3.ID])
}

func Test_GetRestoreNodesStats_ExcludesStaleNodesFromCache(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	node3 := createTestRestoreNode()
	defer cleanupTestNode(registry, node1)
	defer cleanupTestNode(registry, node2)
	defer cleanupTestNode(registry, node3)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node1)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node2)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node3)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node1.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node2.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node3.ID)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), registry.timeout)
	defer cancel()

	key := fmt.Sprintf("%s%s%s", nodeInfoKeyPrefix, node2.ID.String(), nodeInfoKeySuffix)
	result := registry.client.Do(ctx, registry.client.B().Get().Key(key).Build())
	assert.NoError(t, result.Error())

	data, err := result.AsBytes()
	assert.NoError(t, err)

	var node RestoreNode
	err = json.Unmarshal(data, &node)
	assert.NoError(t, err)

	node.LastHeartbeat = time.Now().UTC().Add(-3 * time.Minute)
	modifiedData, err := json.Marshal(node)
	assert.NoError(t, err)

	setCtx, setCancel := context.WithTimeout(context.Background(), registry.timeout)
	defer setCancel()
	setResult := registry.client.Do(
		setCtx,
		registry.client.B().Set().Key(key).Value(string(modifiedData)).Build(),
	)
	assert.NoError(t, setResult.Error())

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Len(t, stats, 2)

	statsMap := make(map[uuid.UUID]int)
	for _, stat := range stats {
		statsMap[stat.ID] = stat.ActiveRestores
	}

	assert.Equal(t, 1, statsMap[node1.ID])
	_, hasNode2 := statsMap[node2.ID]
	assert.False(t, hasNode2)
	assert.Equal(t, 1, statsMap[node3.ID])
}

func Test_CleanupDeadNodes_RemovesNodeInfoAndCounter(t *testing.T) {
	cache_utils.ClearAllCache()

	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	defer cleanupTestNode(registry, node1)
	defer cleanupTestNode(registry, node2)

	err := registry.HearthbeatNodeInRegistry(time.Now().UTC(), node1)
	assert.NoError(t, err)
	err = registry.HearthbeatNodeInRegistry(time.Now().UTC(), node2)
	assert.NoError(t, err)

	err = registry.IncrementRestoresInProgress(node1.ID)
	assert.NoError(t, err)
	err = registry.IncrementRestoresInProgress(node2.ID)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), registry.timeout)
	defer cancel()

	key := fmt.Sprintf("%s%s%s", nodeInfoKeyPrefix, node2.ID.String(), nodeInfoKeySuffix)
	result := registry.client.Do(ctx, registry.client.B().Get().Key(key).Build())
	assert.NoError(t, result.Error())

	data, err := result.AsBytes()
	assert.NoError(t, err)

	var node RestoreNode
	err = json.Unmarshal(data, &node)
	assert.NoError(t, err)

	node.LastHeartbeat = time.Now().UTC().Add(-3 * time.Minute)
	modifiedData, err := json.Marshal(node)
	assert.NoError(t, err)

	setCtx, setCancel := context.WithTimeout(context.Background(), registry.timeout)
	defer setCancel()
	setResult := registry.client.Do(
		setCtx,
		registry.client.B().Set().Key(key).Value(string(modifiedData)).Build(),
	)
	assert.NoError(t, setResult.Error())

	err = registry.cleanupDeadNodes()
	assert.NoError(t, err)

	checkCtx, checkCancel := context.WithTimeout(context.Background(), registry.timeout)
	defer checkCancel()

	infoKey := fmt.Sprintf("%s%s%s", nodeInfoKeyPrefix, node2.ID.String(), nodeInfoKeySuffix)
	infoResult := registry.client.Do(checkCtx, registry.client.B().Get().Key(infoKey).Build())
	assert.Error(t, infoResult.Error())

	counterKey := fmt.Sprintf(
		"%s%s%s",
		nodeActiveRestoresPrefix,
		node2.ID.String(),
		nodeActiveRestoresSuffix,
	)
	counterCtx, counterCancel := context.WithTimeout(context.Background(), registry.timeout)
	defer counterCancel()
	counterResult := registry.client.Do(
		counterCtx,
		registry.client.B().Get().Key(counterKey).Build(),
	)
	assert.Error(t, counterResult.Error())

	activeInfoKey := fmt.Sprintf("%s%s%s", nodeInfoKeyPrefix, node1.ID.String(), nodeInfoKeySuffix)
	activeCtx, activeCancel := context.WithTimeout(context.Background(), registry.timeout)
	defer activeCancel()
	activeResult := registry.client.Do(
		activeCtx,
		registry.client.B().Get().Key(activeInfoKey).Build(),
	)
	assert.NoError(t, activeResult.Error())

	nodes, err := registry.GetAvailableNodes()
	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Equal(t, node1.ID, nodes[0].ID)

	stats, err := registry.GetRestoreNodesStats()
	assert.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, node1.ID, stats[0].ID)
}

func createTestRegistry() *RestoreNodesRegistry {
	return &RestoreNodesRegistry{
		client:            cache_utils.GetValkeyClient(),
		logger:            logger.GetLogger(),
		timeout:           cache_utils.DefaultCacheTimeout,
		pubsubRestores:    cache_utils.NewPubSubManager(),
		pubsubCompletions: cache_utils.NewPubSubManager(),
		runOnce:           sync.Once{},
		hasRun:            atomic.Bool{},
	}
}

func createTestRestoreNode() RestoreNode {
	return RestoreNode{
		ID:            uuid.New(),
		ThroughputMBs: 100,
		LastHeartbeat: time.Now().UTC(),
	}
}

func cleanupTestNode(registry *RestoreNodesRegistry, node RestoreNode) {
	registry.UnregisterNodeFromRegistry(node)
}

func Test_AssignRestoreToNode_PublishesJsonMessageToChannel(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID := uuid.New()

	err := registry.AssignRestoreToNode(node.ID, restoreID, true)
	assert.NoError(t, err)
}

func Test_SubscribeNodeForRestoresAssignment_ReceivesSubmittedRestoresForMatchingNode(
	t *testing.T,
) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID := uuid.New()
	defer registry.UnsubscribeNodeForRestoresAssignments()

	receivedRestoreID := make(chan uuid.UUID, 1)
	handler := func(id uuid.UUID, isCallNotifier bool) {
		receivedRestoreID <- id
	}

	err := registry.SubscribeNodeForRestoresAssignment(node.ID, handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.AssignRestoreToNode(node.ID, restoreID, true)
	assert.NoError(t, err)

	select {
	case received := <-receivedRestoreID:
		assert.Equal(t, restoreID, received)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for restore message")
	}
}

func Test_SubscribeNodeForRestoresAssignment_FiltersOutRestoresForDifferentNode(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	restoreID := uuid.New()
	defer registry.UnsubscribeNodeForRestoresAssignments()

	receivedRestoreID := make(chan uuid.UUID, 1)
	handler := func(id uuid.UUID, isCallNotifier bool) {
		receivedRestoreID <- id
	}

	err := registry.SubscribeNodeForRestoresAssignment(node1.ID, handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.AssignRestoreToNode(node2.ID, restoreID, false)
	assert.NoError(t, err)

	select {
	case <-receivedRestoreID:
		t.Fatal("Should not receive restore for different node")
	case <-time.After(500 * time.Millisecond):
	}
}

func Test_SubscribeNodeForRestoresAssignment_ParsesJsonAndRestoreIdCorrectly(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID1 := uuid.New()
	restoreID2 := uuid.New()
	defer registry.UnsubscribeNodeForRestoresAssignments()

	receivedRestores := make(chan uuid.UUID, 2)
	handler := func(id uuid.UUID, isCallNotifier bool) {
		receivedRestores <- id
	}

	err := registry.SubscribeNodeForRestoresAssignment(node.ID, handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.AssignRestoreToNode(node.ID, restoreID1, true)
	assert.NoError(t, err)

	err = registry.AssignRestoreToNode(node.ID, restoreID2, false)
	assert.NoError(t, err)

	received1 := <-receivedRestores
	received2 := <-receivedRestores

	receivedIDs := []uuid.UUID{received1, received2}
	assert.Contains(t, receivedIDs, restoreID1)
	assert.Contains(t, receivedIDs, restoreID2)
}

func Test_SubscribeNodeForRestoresAssignment_HandlesInvalidJson(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	defer registry.UnsubscribeNodeForRestoresAssignments()

	receivedRestoreID := make(chan uuid.UUID, 1)
	handler := func(id uuid.UUID, isCallNotifier bool) {
		receivedRestoreID <- id
	}

	err := registry.SubscribeNodeForRestoresAssignment(node.ID, handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	err = registry.pubsubRestores.Publish(ctx, "restore:submit", "invalid json")
	assert.NoError(t, err)

	select {
	case <-receivedRestoreID:
		t.Fatal("Should not receive restore for invalid JSON")
	case <-time.After(500 * time.Millisecond):
	}
}

func Test_UnsubscribeNodeForRestoresAssignments_StopsReceivingMessages(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID1 := uuid.New()
	restoreID2 := uuid.New()

	receivedRestoreID := make(chan uuid.UUID, 2)
	handler := func(id uuid.UUID, isCallNotifier bool) {
		receivedRestoreID <- id
	}

	err := registry.SubscribeNodeForRestoresAssignment(node.ID, handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.AssignRestoreToNode(node.ID, restoreID1, true)
	assert.NoError(t, err)

	received := <-receivedRestoreID
	assert.Equal(t, restoreID1, received)

	err = registry.UnsubscribeNodeForRestoresAssignments()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.AssignRestoreToNode(node.ID, restoreID2, false)
	assert.NoError(t, err)

	select {
	case <-receivedRestoreID:
		t.Fatal("Should not receive restore after unsubscribe")
	case <-time.After(500 * time.Millisecond):
	}
}

func Test_SubscribeNodeForRestoresAssignment_WhenAlreadySubscribed_ReturnsError(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	defer registry.UnsubscribeNodeForRestoresAssignments()

	handler := func(id uuid.UUID, isCallNotifier bool) {}

	err := registry.SubscribeNodeForRestoresAssignment(node.ID, handler)
	assert.NoError(t, err)

	err = registry.SubscribeNodeForRestoresAssignment(node.ID, handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already subscribed")
}

func Test_MultipleNodes_EachReceivesOnlyTheirRestores(t *testing.T) {
	cache_utils.ClearAllCache()
	registry1 := createTestRegistry()
	registry2 := createTestRegistry()
	registry3 := createTestRegistry()

	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	node3 := createTestRestoreNode()

	restoreID1 := uuid.New()
	restoreID2 := uuid.New()
	restoreID3 := uuid.New()

	defer registry1.UnsubscribeNodeForRestoresAssignments()
	defer registry2.UnsubscribeNodeForRestoresAssignments()
	defer registry3.UnsubscribeNodeForRestoresAssignments()

	receivedRestores1 := make(chan uuid.UUID, 3)
	receivedRestores2 := make(chan uuid.UUID, 3)
	receivedRestores3 := make(chan uuid.UUID, 3)

	handler1 := func(id uuid.UUID, isCallNotifier bool) { receivedRestores1 <- id }
	handler2 := func(id uuid.UUID, isCallNotifier bool) { receivedRestores2 <- id }
	handler3 := func(id uuid.UUID, isCallNotifier bool) { receivedRestores3 <- id }

	err := registry1.SubscribeNodeForRestoresAssignment(node1.ID, handler1)
	assert.NoError(t, err)

	err = registry2.SubscribeNodeForRestoresAssignment(node2.ID, handler2)
	assert.NoError(t, err)

	err = registry3.SubscribeNodeForRestoresAssignment(node3.ID, handler3)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	submitRegistry := createTestRegistry()
	err = submitRegistry.AssignRestoreToNode(node1.ID, restoreID1, true)
	assert.NoError(t, err)

	err = submitRegistry.AssignRestoreToNode(node2.ID, restoreID2, false)
	assert.NoError(t, err)

	err = submitRegistry.AssignRestoreToNode(node3.ID, restoreID3, true)
	assert.NoError(t, err)

	select {
	case received := <-receivedRestores1:
		assert.Equal(t, restoreID1, received)
	case <-time.After(2 * time.Second):
		t.Fatal("Node 1 timeout waiting for restore message")
	}

	select {
	case received := <-receivedRestores2:
		assert.Equal(t, restoreID2, received)
	case <-time.After(2 * time.Second):
		t.Fatal("Node 2 timeout waiting for restore message")
	}

	select {
	case received := <-receivedRestores3:
		assert.Equal(t, restoreID3, received)
	case <-time.After(2 * time.Second):
		t.Fatal("Node 3 timeout waiting for restore message")
	}

	select {
	case <-receivedRestores1:
		t.Fatal("Node 1 should not receive additional restores")
	case <-time.After(300 * time.Millisecond):
	}

	select {
	case <-receivedRestores2:
		t.Fatal("Node 2 should not receive additional restores")
	case <-time.After(300 * time.Millisecond):
	}

	select {
	case <-receivedRestores3:
		t.Fatal("Node 3 should not receive additional restores")
	case <-time.After(300 * time.Millisecond):
	}
}

func Test_PublishRestoreCompletion_PublishesMessageToChannel(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID := uuid.New()

	err := registry.PublishRestoreCompletion(node.ID, restoreID)
	assert.NoError(t, err)
}

func Test_SubscribeForRestoresCompletions_ReceivesCompletedRestores(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID := uuid.New()
	defer registry.UnsubscribeForRestoresCompletions()

	receivedRestoreID := make(chan uuid.UUID, 1)
	receivedNodeID := make(chan uuid.UUID, 1)
	handler := func(nodeID, restoreID uuid.UUID) {
		receivedNodeID <- nodeID
		receivedRestoreID <- restoreID
	}

	err := registry.SubscribeForRestoresCompletions(handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.PublishRestoreCompletion(node.ID, restoreID)
	assert.NoError(t, err)

	select {
	case receivedNode := <-receivedNodeID:
		assert.Equal(t, node.ID, receivedNode)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for node ID")
	}

	select {
	case received := <-receivedRestoreID:
		assert.Equal(t, restoreID, received)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for restore completion message")
	}
}

func Test_SubscribeForRestoresCompletions_ParsesJsonCorrectly(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID1 := uuid.New()
	restoreID2 := uuid.New()
	defer registry.UnsubscribeForRestoresCompletions()

	receivedRestores := make(chan uuid.UUID, 2)
	handler := func(nodeID, restoreID uuid.UUID) {
		receivedRestores <- restoreID
	}

	err := registry.SubscribeForRestoresCompletions(handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.PublishRestoreCompletion(node.ID, restoreID1)
	assert.NoError(t, err)

	err = registry.PublishRestoreCompletion(node.ID, restoreID2)
	assert.NoError(t, err)

	received1 := <-receivedRestores
	received2 := <-receivedRestores

	receivedIDs := []uuid.UUID{received1, received2}
	assert.Contains(t, receivedIDs, restoreID1)
	assert.Contains(t, receivedIDs, restoreID2)
}

func Test_SubscribeForRestoresCompletions_HandlesInvalidJson(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	defer registry.UnsubscribeForRestoresCompletions()

	receivedRestoreID := make(chan uuid.UUID, 1)
	handler := func(nodeID, restoreID uuid.UUID) {
		receivedRestoreID <- restoreID
	}

	err := registry.SubscribeForRestoresCompletions(handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	err = registry.pubsubCompletions.Publish(ctx, "restore:completion", "invalid json")
	assert.NoError(t, err)

	select {
	case <-receivedRestoreID:
		t.Fatal("Should not receive restore for invalid JSON")
	case <-time.After(500 * time.Millisecond):
	}
}

func Test_UnsubscribeForRestoresCompletions_StopsReceivingMessages(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	node := createTestRestoreNode()
	restoreID1 := uuid.New()
	restoreID2 := uuid.New()

	receivedRestoreID := make(chan uuid.UUID, 2)
	handler := func(nodeID, restoreID uuid.UUID) {
		receivedRestoreID <- restoreID
	}

	err := registry.SubscribeForRestoresCompletions(handler)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.PublishRestoreCompletion(node.ID, restoreID1)
	assert.NoError(t, err)

	received := <-receivedRestoreID
	assert.Equal(t, restoreID1, received)

	err = registry.UnsubscribeForRestoresCompletions()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = registry.PublishRestoreCompletion(node.ID, restoreID2)
	assert.NoError(t, err)

	select {
	case <-receivedRestoreID:
		t.Fatal("Should not receive restore after unsubscribe")
	case <-time.After(500 * time.Millisecond):
	}
}

func Test_SubscribeForRestoresCompletions_WhenAlreadySubscribed_ReturnsError(t *testing.T) {
	cache_utils.ClearAllCache()
	registry := createTestRegistry()
	defer registry.UnsubscribeForRestoresCompletions()

	handler := func(nodeID, restoreID uuid.UUID) {}

	err := registry.SubscribeForRestoresCompletions(handler)
	assert.NoError(t, err)

	err = registry.SubscribeForRestoresCompletions(handler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already subscribed")
}

func Test_MultipleSubscribers_EachReceivesCompletionMessages(t *testing.T) {
	cache_utils.ClearAllCache()
	registry1 := createTestRegistry()
	registry2 := createTestRegistry()
	registry3 := createTestRegistry()

	node1 := createTestRestoreNode()
	node2 := createTestRestoreNode()
	node3 := createTestRestoreNode()

	restoreID1 := uuid.New()
	restoreID2 := uuid.New()
	restoreID3 := uuid.New()

	defer registry1.UnsubscribeForRestoresCompletions()
	defer registry2.UnsubscribeForRestoresCompletions()
	defer registry3.UnsubscribeForRestoresCompletions()

	receivedRestores1 := make(chan uuid.UUID, 3)
	receivedRestores2 := make(chan uuid.UUID, 3)
	receivedRestores3 := make(chan uuid.UUID, 3)

	handler1 := func(nodeID, restoreID uuid.UUID) { receivedRestores1 <- restoreID }
	handler2 := func(nodeID, restoreID uuid.UUID) { receivedRestores2 <- restoreID }
	handler3 := func(nodeID, restoreID uuid.UUID) { receivedRestores3 <- restoreID }

	err := registry1.SubscribeForRestoresCompletions(handler1)
	assert.NoError(t, err)

	err = registry2.SubscribeForRestoresCompletions(handler2)
	assert.NoError(t, err)

	err = registry3.SubscribeForRestoresCompletions(handler3)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	publishRegistry := createTestRegistry()
	err = publishRegistry.PublishRestoreCompletion(node1.ID, restoreID1)
	assert.NoError(t, err)

	err = publishRegistry.PublishRestoreCompletion(node2.ID, restoreID2)
	assert.NoError(t, err)

	err = publishRegistry.PublishRestoreCompletion(node3.ID, restoreID3)
	assert.NoError(t, err)

	receivedAll1 := []uuid.UUID{}
	receivedAll2 := []uuid.UUID{}
	receivedAll3 := []uuid.UUID{}

	for i := 0; i < 3; i++ {
		select {
		case received := <-receivedRestores1:
			receivedAll1 = append(receivedAll1, received)
		case <-time.After(2 * time.Second):
			t.Fatal("Subscriber 1 timeout waiting for completion message")
		}
	}

	for i := 0; i < 3; i++ {
		select {
		case received := <-receivedRestores2:
			receivedAll2 = append(receivedAll2, received)
		case <-time.After(2 * time.Second):
			t.Fatal("Subscriber 2 timeout waiting for completion message")
		}
	}

	for i := 0; i < 3; i++ {
		select {
		case received := <-receivedRestores3:
			receivedAll3 = append(receivedAll3, received)
		case <-time.After(2 * time.Second):
			t.Fatal("Subscriber 3 timeout waiting for completion message")
		}
	}

	assert.Contains(t, receivedAll1, restoreID1)
	assert.Contains(t, receivedAll1, restoreID2)
	assert.Contains(t, receivedAll1, restoreID3)

	assert.Contains(t, receivedAll2, restoreID1)
	assert.Contains(t, receivedAll2, restoreID2)
	assert.Contains(t, receivedAll2, restoreID3)

	assert.Contains(t, receivedAll3, restoreID1)
	assert.Contains(t, receivedAll3, restoreID2)
	assert.Contains(t, receivedAll3, restoreID3)
}
