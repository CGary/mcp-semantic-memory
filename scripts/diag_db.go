package main
import (
	"fmt"
	"log"
	"os"
	"time"
	"github.com/hsme/core/src/storage/sqlite"
)
func main() {
	start := time.Now()
	fmt.Println("Attempting to open DB...")
	db, err := sqlite.InitDB(os.Getenv("SQLITE_DB_PATH"))
	if err != nil {
		log.Fatalf("FAILED: %v", err)
	}
	fmt.Printf("DB Initialized successfully in %v\n", time.Since(start))
	
	var val int
	err = db.QueryRow("SELECT vec_version()").Scan(&val)
	if err != nil {
		fmt.Printf("Warning: vec_version() failed (extension not working): %v\n", err)
	} else {
		fmt.Printf("sqlite-vec version: %v\n", val)
	}
	db.Close()
}
