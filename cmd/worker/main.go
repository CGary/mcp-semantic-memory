package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hsme/core/src/core/inference/ollama"
	"github.com/hsme/core/src/core/worker"
	"github.com/hsme/core/src/storage/sqlite"
)

func main() {
	dbPath := os.Getenv("SQLITE_DB_PATH")
	if dbPath == "" {
		dbPath = "data/engram.db"
	}

	db, err := sqlite.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Error inicializando DB: %v", err)
	}
	defer db.Close()

	ollamaHost := os.Getenv("OLLAMA_HOST")
	embedModel := os.Getenv("EMBEDDING_MODEL")
	if embedModel == "" {
		embedModel = "nomic-embed-text"
	}
	extractModel := os.Getenv("EXTRACTION_MODEL")
	if extractModel == "" {
		extractModel = "phi3.5"
	}

	client := ollama.NewClient(ollamaHost)
	embedder := ollama.NewEmbedder(client, embedModel, 768)
	extractor := ollama.NewExtractor(client, extractModel)

	// Spec §4.2 / §11.2: el worker NO debe arrancar si el embedder activo
	// difiere del persistido. Si lo hace, insertaría vectores de dimensión
	// incompatible y cada tarea fallaría con un error opaco.
	if err := sqlite.ValidateEmbeddingConfig(db, embedder); err != nil {
		log.Fatalf("Config de embedding inválida: %v", err)
	}

	w := worker.NewWorker(db, embedder, extractor)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Worker HSME independiente iniciado (DB: %s)\n", dbPath)
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		fmt.Println("\nApagando worker...")
		cancel()
		os.Exit(0)
	}()

	for {
		task, err := w.LeaseNextTask(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error al arrendar tarea: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if task != nil {
			fmt.Printf("[%s] Ejecutando tarea %d (Tipo: %s, Memoria: %d)...\n", time.Now().Format("15:04:05"), task.ID, task.TaskType, task.MemoryID)
			if err := w.ExecuteTask(ctx, task); err != nil {
				fmt.Fprintf(os.Stderr, "Error ejecutando tarea %d: %v\n", task.ID, err)
				// Persistir el error y liberar el lease para reintento (o marcar failed si se agotaron intentos).
				const maxAttempts = 5
				nextStatus := "pending"
				if task.AttemptCount >= maxAttempts {
					nextStatus = "failed"
				}
				if _, uerr := db.Exec(
					"UPDATE async_tasks SET status = ?, last_error = ?, leased_until = NULL, updated_at = ? WHERE id = ?",
					nextStatus, err.Error(), time.Now().Format(time.RFC3339), task.ID,
				); uerr != nil {
					fmt.Fprintf(os.Stderr, "Error registrando fallo de tarea %d: %v\n", task.ID, uerr)
				}
			} else {
				fmt.Printf("[%s] Tarea %d completada con éxito\n", time.Now().Format("15:04:05"), task.ID)
			}
		} else {
			time.Sleep(2 * time.Second)
		}
	}
}
