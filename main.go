package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MicroSOA-09/gateway-service/handler"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Warning: Could not load .env file, using defaults:", err)
	}
	
	config := &handler.Config{
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
		BlogServiceURL: os.Getenv("BLOG_SERVICE_URL"),
		UserServiceURL: os.Getenv("USER_SERVICE_URL"),
		AspServiceURL:  os.Getenv("ASP_SERVICE_URL"),
	}

	if config.AuthServiceURL == "" || config.BlogServiceURL == "" || config.UserServiceURL == "" || config.AspServiceURL == "" {
		log.Fatal("Missing required environment variables")
	}

	logger := log.New(os.Stdout, "[gateway] ", log.LstdFlags)

	gateway, err := handler.NewGateway(config, logger)
	if err != nil {
		logger.Fatal("Failed to initialize gateway:", err)
	}

	router := mux.NewRouter()
	router.Use(gateway.AuthMiddleware)

	// Routes with authentication middleware
	authRouter := router.PathPrefix("/api/auth").Subrouter()
	authRouter.HandleFunc("/{path:.*}", gateway.ProxyHandler(gateway.AuthProxy, config.AuthServiceURL))

	blogRouter := router.PathPrefix("/api/blog").Subrouter()
	blogRouter.HandleFunc("/{path:.*}", gateway.ProxyHandler(gateway.BlogProxy, config.BlogServiceURL))

	userRouter := router.PathPrefix("/api/user").Subrouter()
	userRouter.HandleFunc("/{path:.*}", gateway.ProxyHandler(gateway.UserProxy, config.UserServiceURL))

	aspRouter := router.PathPrefix("/api/").Subrouter()
	aspRouter.HandleFunc("/{path:.*}", gateway.ProxyHandler(gateway.AspProxy, config.AspServiceURL))
	// Apply auth middleware to all routes

	// Definiši CORS opcije
	cors := handlers.CORS(
		handlers.AllowedOrigins([]string{"http://localhost:4200"}), // Specifično za Angular frontend
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
		handlers.AllowCredentials(),
	)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      cors(router),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	logger.Printf("Starting gateway on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed:", err)
	}
}
