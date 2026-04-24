package search

import (
        "context"
        "database/sql"

        "github.com/hsme/core/src/core/indexer"
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

        // Simetría Semántica: Usamos el mismo algoritmo que el indexador
        searchName, _ := indexer.CanonicalizeName(entityName)
        fuzzyName := "%" + entityName + "%"

        // CTE Recursivo: travesía bidireccional robusta con fallback de nombre
        query := `
                WITH RECURSIVE trace(id, depth) AS (
                        SELECT id, 0 FROM kg_nodes 
                        WHERE canonical_name = ? 
                        OR display_name LIKE ? -- Fallback si el canónico falla
                        UNION
                        SELECT 
                                CASE WHEN t.id = e.source_node_id THEN e.target_node_id ELSE e.source_node_id END,
                                t.depth + 1
                        FROM kg_edge_evidence e
                        JOIN trace t ON (t.id = e.source_node_id OR t.id = e.target_node_id)
                        WHERE t.depth < ? 
                        AND (
                             (? = 'both') OR 
                             (? = 'downstream' AND t.id = e.source_node_id) OR
                             (? = 'upstream' AND t.id = e.target_node_id)
                        )
                )
                SELECT DISTINCT id FROM trace
        `
        rows, err := db.QueryContext(ctx, query, searchName, fuzzyName, maxDepth, direction, direction, direction)
        if err != nil {
                return nil, err
        }
        defer rows.Close()

        result := &TraceResult{
                Entity: entityName,
                Nodes:  []map[string]interface{}{},
                Edges:  []DependencyEdge{},
        }

        reachableIDs := make(map[int64]bool)
        for rows.Next() {
                var id int64
                if err := rows.Scan(&id); err == nil {
                        reachableIDs[id] = true
                }
        }

        if len(reachableIDs) == 0 {
                return result, nil
        }

        // 1. Obtener detalles de todos los nodos alcanzados
        for nodeID := range reachableIDs {
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

        // 2. Obtener todas las aristas que conectan estos nodos
        // Construimos un IN clause dinámico o iteramos si son pocos (simplificado para este fix)
        for nodeID := range reachableIDs {
                edgeRows, err := db.QueryContext(ctx, `
                        SELECT source_node_id, target_node_id, relation_type, memory_id 
                        FROM kg_edge_evidence 
                        WHERE source_node_id = ? OR target_node_id = ?`, nodeID, nodeID)
                if err == nil {
                        for edgeRows.Next() {
                                var edge DependencyEdge
                                if err := edgeRows.Scan(&edge.SourceID, &edge.TargetID, &edge.RelationType, &edge.MemoryID); err == nil {
                                        // Solo añadir si ambos nodos de la arista están en nuestro set alcanzable
                                        if reachableIDs[edge.SourceID] && reachableIDs[edge.TargetID] {
                                                // Evitar duplicados (arista A->B vista desde A y desde B)
                                                found := false
                                                for _, existing := range result.Edges {
                                                        if existing.SourceID == edge.SourceID && existing.TargetID == edge.TargetID && existing.RelationType == edge.RelationType {
                                                                found = true
                                                                break
                                                        }
                                                }
                                                if !found {
                                                        result.Edges = append(result.Edges, edge)
                                                }
                                        }
                                }
                        }
                        edgeRows.Close()
                }
        }

        return result, nil
}
