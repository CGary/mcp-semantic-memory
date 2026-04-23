package search

import (
	"context"
	"database/sql"
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
	Entity string                   `json:"entity"`
	Nodes  []map[string]interface{} `json:"nodes"`
	Edges  []DependencyEdge         `json:"edges"`
}

func TraceDependencies(ctx context.Context, db *sql.DB, entityName string, direction string, maxDepth int) (*TraceResult, error) {
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

	result := &TraceResult{Entity: entityName}
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

	// Fetch node details
	for nodeID := range nodeSet {
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
