# Idea: Solving the Recency Gap in Semantic Memory

## The Problem
Semantic search (`search_fuzzy`) prioritizes relevance over chronology. In an active coding session, the most "relevant" memory might be a design decision from three days ago, while the agent actually needs the "last thing we did" five minutes ago.

## Proposed Solutions

1. **Session-Tagged Summaries**:
   - Every session summary (type: `note`) must include a unique Session ID and a timestamp in its content.
   - When an agent asks "What did we do last?", it should first perform a lexical search (`search_exact`) for the latest "Session Summary" entry.

2. **RRF Time-Decay (Future Feature)**:
   - Modify the Reciprocal Rank Fusion (RRF) algorithm to include a "recency booster" based on the `created_at` field.
   - This ensures that if two memories have similar semantic scores, the newer one wins.

3. **Contextual Anchoring**:
   - The agent should maintain a "Session Stack" in its local context, only flushing it to HSME at key milestones.
