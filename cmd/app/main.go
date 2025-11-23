// ...
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/2Empty/review-assigner/internal/handlers"
	"github.com/2Empty/review-assigner/internal/store"
)

func main() {

	time.Sleep(5 * time.Second)

	st, err := store.New(context.Background())
	if err != nil {
		log.Fatal("Failed to create store:", err)
	}
	defer st.Close()

	log.Println("Successfully connected to database")

	// Инициализация ручек
	h := handlers.NewHandler(st)

	// Настройка маршрутов
	mux := http.NewServeMux()
	h.SetupRoutes(mux)

	// Запуск сервера
	port := ":8080"
	if os.Getenv("PORT") != "" {
		port = ":" + os.Getenv("PORT")
	}

	log.Printf("Server starting on port %s", port)

	// Health check для Docker
	go func() {
		time.Sleep(2 * time.Second)
		log.Println("Application is healthy and running")
	}()

	log.Fatal(http.ListenAndServe(port, mux))
}
