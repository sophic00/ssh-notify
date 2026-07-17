package main

import (
	"database/sql"
	"errors"
	"os"
	"testing"
)

func TestDBStore(t *testing.T) {
	dbPath := "test_ssh_notify.db"
	defer os.Remove(dbPath)

	store, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	// Test adding a server
	testServer := "test-server-1"
	testToken := "abcdef1234567890abcdef1234567890"

	err = store.AddServer(testServer, testToken)
	if err != nil {
		t.Fatalf("Failed to add server: %v", err)
	}

	// Verify lookups
	matchedName, err := store.GetServerByToken(testToken)
	if err != nil {
		t.Fatalf("Failed to get server by token: %v", err)
	}
	if matchedName != testServer {
		t.Errorf("Expected server name '%s', got '%s'", testServer, matchedName)
	}

	// Verify listing
	servers, err := store.ListServers()
	if err != nil {
		t.Fatalf("Failed to list servers: %v", err)
	}
	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	} else if servers[0].Name != testServer || servers[0].Token != testToken {
		t.Errorf("Mismatch in listed server: %+v", servers[0])
	}

	// Test renaming a server
	newName := "test-server-renamed"
	err = store.RenameServer(testServer, newName)
	if err != nil {
		t.Fatalf("Failed to rename server: %v", err)
	}

	// Lookup by old name should fail or list should show new name
	servers, err = store.ListServers()
	if err != nil {
		t.Fatalf("Failed to list servers after rename: %v", err)
	}
	if len(servers) != 1 || servers[0].Name != newName {
		t.Errorf("Rename failed. Listed servers: %+v", servers)
	}

	// Test regenerating token
	newToken := "9876543210fedcbasyz"
	err = store.RegenerateServerToken(newName, newToken)
	if err != nil {
		t.Fatalf("Failed to regenerate server token: %v", err)
	}

	matchedName, err = store.GetServerByToken(newToken)
	if err != nil {
		t.Fatalf("Failed to get server by new token: %v", err)
	}
	if matchedName != newName {
		t.Errorf("Expected server name '%s', got '%s'", newName, matchedName)
	}

	// Old token should be invalid
	_, err = store.GetServerByToken(testToken)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Expected ErrNoRows for old token, got: %v", err)
	}

	// Test chat registration
	chatID := int64(987654321)
	chatTitle := "Security Notifications Group"
	err = store.AddChat(chatID, chatTitle)
	if err != nil {
		t.Fatalf("Failed to add authorized chat: %v", err)
	}

	chats, err := store.ListChats()
	if err != nil {
		t.Fatalf("Failed to list chats: %v", err)
	}
	if len(chats) != 1 || chats[0].ChatID != chatID || chats[0].Title != chatTitle {
		t.Errorf("Mismatch in listed chat: %+v", chats)
	}

	// Test removing chat
	err = store.RemoveChat(chatID)
	if err != nil {
		t.Fatalf("Failed to remove chat: %v", err)
	}
	chats, err = store.ListChats()
	if err != nil {
		t.Fatalf("Failed to list chats after removal: %v", err)
	}
	if len(chats) != 0 {
		t.Errorf("Expected 0 chats, got %d", len(chats))
	}

	// Test removing server
	err = store.RemoveServer(newName)
	if err != nil {
		t.Fatalf("Failed to remove server: %v", err)
	}

	servers, err = store.ListServers()
	if err != nil {
		t.Fatalf("Failed to list servers after removal: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(servers))
	}
}
