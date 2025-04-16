package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	AuthServiceURL string
	BlogServiceURL string
	UserServiceURL string
	AspServiceURL  string
}

// Gateway struct
type Gateway struct {
	Config    *Config
	Logger    *log.Logger
	AuthProxy *httputil.ReverseProxy
	BlogProxy *httputil.ReverseProxy
	UserProxy *httputil.ReverseProxy
	AspProxy  *httputil.ReverseProxy
	Client    *http.Client
}

type AuthValidateResponse struct {
	UserID   string `json:"userID"`
	Role     string `json:"role"`
	Username string `json:"username"`
	Error    string `json:"error,omitempty"`
}

// NewGateway initializes the gateway
func NewGateway(config *Config, logger *log.Logger) (*Gateway, error) {
	authURL, err := url.Parse(config.AuthServiceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid auth service URL: %w", err)
	}
	blogURL, err := url.Parse(config.BlogServiceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid blog service URL: %w", err)
	}
	userURL, err := url.Parse(config.UserServiceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid user service URL: %w", err)
	}
	aspURL, err := url.Parse(config.AspServiceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid asp service URL: %w", err)
	}

	return &Gateway{
		Config:    config,
		Logger:    logger,
		AuthProxy: httputil.NewSingleHostReverseProxy(authURL),
		BlogProxy: httputil.NewSingleHostReverseProxy(blogURL),
		UserProxy: httputil.NewSingleHostReverseProxy(userURL),
		AspProxy:  httputil.NewSingleHostReverseProxy(aspURL),
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// authMiddleware validates JWT for protected routes
func (g *Gateway) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for /api/auth/*
		if strings.HasPrefix(r.URL.Path, "/api/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "invalid Authorization format", http.StatusUnauthorized)
			return
		}
		userID, role, username, err := g.validateJWT(parts[1])
		if err != nil {
			g.Logger.Printf("JWT validation failed: %v", err)
			http.Error(w, "invalid JWT", http.StatusUnauthorized)
			return
		}

		// Add userID, role, and username to request headers
		r.Header.Set("X-User-ID", userID)
		r.Header.Set("X-User-Role", role)
		r.Header.Set("X-Username", username)

		next.ServeHTTP(w, r)
	})
}

// proxyHandler forwards requests to the target proxy
func (g *Gateway) ProxyHandler(proxy *httputil.ReverseProxy, targetURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g.Logger.Printf("Forwarding %s %s to %s", r.Method, r.URL.Path, targetURL)
		proxy.ServeHTTP(w, r)
	}
}

// validateJWT sends a request to AuthService to validate the JWT
func (g *Gateway) validateJWT(token string) (string, string, string, error) {
	g.Logger.Printf("Authorizing... Forwarding requet")

	req, err := http.NewRequest("POST", g.Config.AuthServiceURL+"/api/auth/jwt", nil)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.Client.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to contact AuthService: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var authResp AuthValidateResponse
		if err := json.NewDecoder(resp.Body).Decode(&authResp); err == nil && authResp.Error != "" {
			return "", "", "", fmt.Errorf("AuthService error: %s", authResp.Error)
		}
		return "", "", "", fmt.Errorf("AuthService returned status: %d", resp.StatusCode)
	}

	var authResp AuthValidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", "", "", fmt.Errorf("failed to decode AuthService response: %w", err)
	}

	if authResp.UserID == "" || authResp.Role == "" {
		return "", "", "", fmt.Errorf("invalid AuthService response: missing userID or role")
	}

	return authResp.UserID, authResp.Role, authResp.Username, nil
}
