package main
import (
	"fmt"
	"log"
	"os"
	"github.com/hsme/core/src/core/indexer"
	"github.com/hsme/core/src/storage/sqlite"
)
func main() {
	db, err := sqlite.InitDB(os.Getenv("SQLITE_DB_PATH"))
	if err != nil { log.Fatal(err) }
	content, _ := os.ReadFile("Technical_Specification.md")
	id, err := indexer.StoreContext(db, string(content), "technical_spec", nil, false)
	if err != nil { log.Fatal(err) }
	fmt.Printf("Successfully ingested Spec! Memory ID: %d\n", id)
}
