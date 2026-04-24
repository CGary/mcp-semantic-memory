package search

import (
	"context"
	"database/sql"
	"sort"
)

func GraphSearch(ctx context.Context, db *sql.DB, query string, limit int) ([]SearchResult, error) {
	// Minimal graph search logic: find nodes matching name and return their evidence memories
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT e.memory_id, 1.0
		FROM kg_nodes n
		JOIN kg_edge_evidence e ON n.id = e.source_node_id OR n.id = e.target_node_id
		WHERE n.canonical_name LIKE ?
		LIMIT ?
	`, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		if err := rows.Scan(&res.ID, &res.Score); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

type DependencyEdge struct {
	SourceID     int64  `json:"source_id"`
	TargetID     int64  `json:"target_id"`
	RelationType string `json:"relation_type"`
	MemoryID     int64  `json:"memory_id"`
}

type TraceResult struct {
	Entity    string                   `json:"entity"`
	Nodes     []map[string]interface{} `json:"nodes"`
	Edges     []DependencyEdge         `json:"edges"`
	Truncated bool                     `json:"truncated"`
}

func TraceDependencies(ctx context.Context, db *sql.DB, entityName string, direction string, maxDepth int, maxNodes int) (*TraceResult, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if maxNodes <= 0 {
		maxNodes = 100
	}

	// Simplified recursive CTE for tracing dependencies
	query := `
		WITH RECURSIVE trace(id, depth) AS (
			SELECT id, 0 FROM kg_nodes WHERE canonical_name = ?
			UNION
			SELECT 
				CASE 
					WHEN ? = 'downstream' THEN e.target_node_id
					WHEN ? = 'upstream' THEN e.source_node_id
					ELSE CASE WHEN t.id = e.source_node_id THEN e.target_node_id ELSE e.source_node_id END
				END,
				t.depth + 1
			FROM kg_edge_evidence e
			JOIN trace t ON (
				(? = 'downstream' AND t.id = e.source_node_id) OR
				(? = 'upstream' AND t.id = e.target_node_id) OR
				(? = 'both' AND (t.id = e.source_node_id OR t.id = e.target_node_id))
			)
			WHERE t.depth < ?
		)
		SELECT DISTINCT e.source_node_id, e.target_node_id, e.relation_type, e.memory_id
		FROM kg_edge_evidence e
		JOIN trace t ON t.id = e.source_node_id OR t.id = e.target_node_id
	`
	rows, err := db.QueryContext(ctx, query, entityName, direction, direction, direction, direction, direction, maxDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &TraceResult{
		Entity: entityName,
		Nodes:  []map[string]interface{}{},
		Edges:  []DependencyEdge{},
	}
	nodeSet := make(map[int64]bool)

	for rows.Next() {
		var edge DependencyEdge
		if err := rows.Scan(&edge.SourceID, &edge.TargetID, &edge.RelationType, &edge.MemoryID); err != nil {
			return nil, err
		}
		result.Edges = append(result.Edges, edge)
		nodeSet[edge.SourceID] = true
		nodeSet[edge.TargetID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	nodeIDs := make([]int64, 0, len(nodeSet))
	for nodeID := range nodeSet {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Slice(nodeIDs, func(i, j int) bool { return nodeIDs[i] < nodeIDs[j] })

	if len(nodeIDs) > maxNodes {
		result.Truncated = true
		allowed := make(map[int64]struct{}, maxNodes)
		for _, nodeID := range nodeIDs[:maxNodes] {
			allowed[nodeID] = struct{}{}
		}
		filteredEdges := make([]DependencyEdge, 0, len(result.Edges))
		for _, edge := range result.Edges {
			_, okSource := allowed[edge.SourceID]
			_, okTarget := allowed[edge.TargetID]
			if okSource && okTarget {
				filteredEdges = append(filteredEdges, edge)
			}
		}
		result.Edges = filteredEdges
		nodeIDs = nodeIDs[:maxNodes]
	}

	// Fetch node details
	for _, nodeID := range nodeIDs {
		var nodeType, displayName string
		err := db.QueryRowContext(ctx, "SELECT type, display_name FROM kg_nodes WHERE id = ?", nodeID).Scan(&nodeType, &displayName)
		if err == nil {
			result.Nodes = append(result.Nodes, map[string]interface{}{
				"id":   nodeID,
				"type": nodeType,
				"name": displayName,
			})
		}
	}

	return result, nil
}
