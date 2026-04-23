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
	
	id, err := indexer.StoreContext(db, os.Args[1], "historical_session", nil, false)
	if err != nil { log.Fatal(err) }
	fmt.Printf("%d", id)
}
