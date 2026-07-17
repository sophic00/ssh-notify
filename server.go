package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

type SSHLoginPayload struct {
	Username string `json:"username"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Time     string `json:"time"`
}

type HTTPServer struct {
	db         *DBStore
	botService *BotService
	server     *http.Server
}

func NewHTTPServer(addr string, db *DBStore, botService *BotService) *HTTPServer {
	mux := http.NewServeMux()
	s := &HTTPServer{
		db:         db,
		botService: botService,
	}

	mux.HandleFunc("/ssh-login", s.handleSSHLogin)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

func (s *HTTPServer) Start() error {
	log.Printf("Starting HTTP server on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")
	return s.server.Shutdown(ctx)
}

func (s *HTTPServer) handleSSHLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate client
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Unauthorized: missing authorization header", http.StatusUnauthorized)
		return
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		http.Error(w, "Unauthorized: invalid authorization header format", http.StatusUnauthorized)
		return
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
	if token == "" {
		http.Error(w, "Unauthorized: empty token", http.StatusUnauthorized)
		return
	}

	// Validate token in database
	serverName, err := s.db.GetServerByToken(token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Unauthorized: invalid server token", http.StatusUnauthorized)
		} else {
			log.Printf("Database error validating token: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Decode login payload
	var payload SSHLoginPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request: invalid json body", http.StatusBadRequest)
		return
	}

	if payload.Username == "" || payload.IP == "" {
		http.Error(w, "Bad request: username and ip are required fields", http.StatusBadRequest)
		return
	}

	// Fallback to query-matched server name if not provided in payload
	if payload.Hostname == "" {
		payload.Hostname = serverName
	}

	if payload.Time == "" {
		payload.Time = time.Now().Format("2006-01-02 15:04:05 MST")
	}

	log.Printf("Received valid SSH notification from server '%s' for user '%s' from IP '%s'", serverName, payload.Username, payload.IP)

	// Fetch authorized chats
	chats, err := s.db.ListChats()
	if err != nil {
		log.Printf("Failed to retrieve authorized chats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(chats) == 0 {
		log.Println("Warning: SSH alert received, but no chats are authorized. Use /authchat in Telegram to authorize a chat.")
	}

	// Send notification to all authorized chats
	go func() {
		ctx := context.Background()
		for _, chat := range chats {
			err := s.botService.SendNotification(ctx, chat.ChatID, payload.Hostname, payload.Username, payload.IP, payload.Time)
			if err != nil {
				log.Printf("Failed to send telegram notification to chat %d (%s): %v", chat.ChatID, chat.Title, err)
			}
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"success"}`))
}
