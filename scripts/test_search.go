package main
import (
	"context"
	"fmt"
	"log"
	"os"
	"github.com/hsme/core/src/core/inference/ollama"
	"github.com/hsme/core/src/core/search"
	"github.com/hsme/core/src/storage/sqlite"
)
func main() {
	db, err := sqlite.InitDB(os.Getenv("SQLITE_DB_PATH"))
	if err != nil { log.Fatal(err) }
	client := ollama.NewClient(os.Getenv("OLLAMA_HOST"))
	embedder := ollama.NewEmbedder(client, "nomic-embed-text", 768)
	ctx := context.Background()
	results, err := search.FuzzySearch(ctx, db, embedder, "How is the asynchronous worker implemented?", 3)
	if err != nil { log.Fatal(err) }
	for _, res := range results {
		fmt.Printf("Memory ID: %d, Score: %f, Coverage: %s\n", res.MemoryID, res.Score, res.VectorCoverage)
		for _, h := range res.Highlights {
			fmt.Printf("  - Highlight: %s\n", h.Text)
		}
	}
}
