package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type BotService struct {
	bot     *bot.Bot
	db      *DBStore
	ownerID int64
}

func NewBotService(token string, ownerID int64, db *DBStore) (*BotService, error) {
	s := &BotService{
		db:      db,
		ownerID: ownerID,
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(s.ownerOnly(s.handleDefault)),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telegram bot: %w", err)
	}

	s.bot = b

	// Register owner-restricted commands
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, s.ownerOnly(s.handleStart))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/chatid", bot.MatchTypeExact, s.ownerOnly(s.handleChatID))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/authchat", bot.MatchTypeExact, s.ownerOnly(s.handleAuthChat))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/unauthchat", bot.MatchTypeExact, s.ownerOnly(s.handleUnauthChat))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/addserver", bot.MatchTypePrefix, s.ownerOnly(s.handleAddServer))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/removeserver", bot.MatchTypePrefix, s.ownerOnly(s.handleRemoveServer))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/regenauth", bot.MatchTypePrefix, s.ownerOnly(s.handleRegenAuth))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/renameserver", bot.MatchTypePrefix, s.ownerOnly(s.handleRenameServer))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/listservers", bot.MatchTypeExact, s.ownerOnly(s.handleListServers))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/listchats", bot.MatchTypeExact, s.ownerOnly(s.handleListChats))

	return s, nil
}

func (s *BotService) Start(ctx context.Context) {
	s.bot.Start(ctx)
}

func (s *BotService) ownerOnly(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}
		if update.Message.From.ID != s.ownerID {
			// Silently ignore messages from non-owners to prevent spam/reconnaissance
			return
		}
		next(ctx, b, update)
	}
}

func (s *BotService) reply(ctx context.Context, update *models.Update, text string) {
	_, _ = s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   text,
	})
}

func (s *BotService) handleDefault(ctx context.Context, b *bot.Bot, update *models.Update) {
	s.reply(ctx, update, "Unknown command. Send /start to view available commands.")
}

func (s *BotService) handleStart(ctx context.Context, b *bot.Bot, update *models.Update) {
	helpText := `SSH Notify Bot Commands:

/start - Display this help message
/chatid - Get the current chat ID
/authchat - Authorize the current chat to receive login alerts
/unauthchat - Remove the current chat from authorized alert destinations
/addserver <name> - Add a new server and generate an authentication token
/removeserver <name> - Remove a server
/regenauth <name> - Regenerate the authentication token for a server
/renameserver <old_name> <new_name> - Rename a server
/listservers - List all registered servers
/listchats - List all authorized alert chats`

	s.reply(ctx, update, helpText)
}

func (s *BotService) handleChatID(ctx context.Context, b *bot.Bot, update *models.Update) {
	s.reply(ctx, update, fmt.Sprintf("Current Chat ID: %d", update.Message.Chat.ID))
}

func (s *BotService) handleAuthChat(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	title := update.Message.Chat.Title
	if title == "" {
		if update.Message.Chat.Username != "" {
			title = "@" + update.Message.Chat.Username
		} else {
			title = fmt.Sprintf("Private Chat (%d)", chatID)
		}
	}

	err := s.db.AddChat(chatID, title)
	if err != nil {
		s.reply(ctx, update, fmt.Sprintf("Error authorizing chat: %v", err))
		return
	}

	s.reply(ctx, update, fmt.Sprintf("Chat '%s' (ID: %d) successfully authorized to receive SSH login notifications.", title, chatID))
}

func (s *BotService) handleUnauthChat(ctx context.Context, b *bot.Bot, update *models.Update) {
	chatID := update.Message.Chat.ID
	err := s.db.RemoveChat(chatID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.reply(ctx, update, "This chat is not registered in the notification list.")
		} else {
			s.reply(ctx, update, fmt.Sprintf("Error unauthorizing chat: %v", err))
		}
		return
	}

	s.reply(ctx, update, "Chat successfully removed from SSH login notifications.")
}

func (s *BotService) handleAddServer(ctx context.Context, b *bot.Bot, update *models.Update) {
	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/addserver"))
	if args == "" {
		s.reply(ctx, update, "Usage: /addserver <server-name>")
		return
	}

	serverName := strings.Fields(args)[0]
	token, err := generateSecureToken()
	if err != nil {
		s.reply(ctx, update, fmt.Sprintf("Error generating token: %v", err))
		return
	}

	err = s.db.AddServer(serverName, token)
	if err != nil {
		s.reply(ctx, update, fmt.Sprintf("Error adding server: %v", err))
		return
	}

	instruction := fmt.Sprintf("Server '%s' added successfully.\nToken: %s\n\nSee docs/setup.md to configure the client script with this token.", serverName, token)
	s.reply(ctx, update, instruction)
}

func (s *BotService) handleRemoveServer(ctx context.Context, b *bot.Bot, update *models.Update) {
	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/removeserver"))
	if args == "" {
		s.reply(ctx, update, "Usage: /removeserver <server-name>")
		return
	}

	serverName := strings.Fields(args)[0]
	err := s.db.RemoveServer(serverName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.reply(ctx, update, fmt.Sprintf("Server '%s' not found.", serverName))
		} else {
			s.reply(ctx, update, fmt.Sprintf("Error removing server: %v", err))
		}
		return
	}

	s.reply(ctx, update, fmt.Sprintf("Server '%s' successfully removed.", serverName))
}

func (s *BotService) handleRegenAuth(ctx context.Context, b *bot.Bot, update *models.Update) {
	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/regenauth"))
	if args == "" {
		s.reply(ctx, update, "Usage: /regenauth <server-name>")
		return
	}

	serverName := strings.Fields(args)[0]
	newToken, err := generateSecureToken()
	if err != nil {
		s.reply(ctx, update, fmt.Sprintf("Error generating token: %v", err))
		return
	}

	err = s.db.RegenerateServerToken(serverName, newToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.reply(ctx, update, fmt.Sprintf("Server '%s' not found.", serverName))
		} else {
			s.reply(ctx, update, fmt.Sprintf("Error regenerating token: %v", err))
		}
		return
	}

	s.reply(ctx, update, fmt.Sprintf("Token regenerated for server '%s'.\nNew Token: %s", serverName, newToken))
}

func (s *BotService) handleRenameServer(ctx context.Context, b *bot.Bot, update *models.Update) {
	args := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/renameserver"))
	fields := strings.Fields(args)
	if len(fields) < 2 {
		s.reply(ctx, update, "Usage: /renameserver <old-name> <new-name>")
		return
	}

	oldName := fields[0]
	newName := fields[1]

	err := s.db.RenameServer(oldName, newName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.reply(ctx, update, fmt.Sprintf("Server '%s' not found.", oldName))
		} else {
			s.reply(ctx, update, fmt.Sprintf("Error renaming server: %v", err))
		}
		return
	}

	s.reply(ctx, update, fmt.Sprintf("Server successfully renamed from '%s' to '%s'.", oldName, newName))
}

func (s *BotService) handleListServers(ctx context.Context, b *bot.Bot, update *models.Update) {
	servers, err := s.db.ListServers()
	if err != nil {
		s.reply(ctx, update, fmt.Sprintf("Error listing servers: %v", err))
		return
	}

	if len(servers) == 0 {
		s.reply(ctx, update, "No servers registered.")
		return
	}

	var sb strings.Builder
	sb.WriteString("Registered Servers:\n\n")
	for _, srv := range servers {
		sb.WriteString(fmt.Sprintf("- Name: %s\n  Token: %s\n  Created: %s\n\n", srv.Name, srv.Token, srv.CreatedAt.Format("2006-01-02 15:04:05")))
	}

	s.reply(ctx, update, sb.String())
}

func (s *BotService) handleListChats(ctx context.Context, b *bot.Bot, update *models.Update) {
	chats, err := s.db.ListChats()
	if err != nil {
		s.reply(ctx, update, fmt.Sprintf("Error listing chats: %v", err))
		return
	}

	if len(chats) == 0 {
		s.reply(ctx, update, "No chats authorized to receive alerts.")
		return
	}

	var sb strings.Builder
	sb.WriteString("Authorized Chats:\n\n")
	for _, ch := range chats {
		sb.WriteString(fmt.Sprintf("- Title: %s\n  ID: %d\n  Authorized At: %s\n\n", ch.Title, ch.ChatID, ch.CreatedAt.Format("2006-01-02 15:04:05")))
	}

	s.reply(ctx, update, sb.String())
}

func (s *BotService) SendNotification(ctx context.Context, chatID int64, serverName, username, ip, loginTime string) error {
	text := fmt.Sprintf(
		"<b>SSH Login Notification</b>\n\n<b>Server:</b> %s\n<b>User:</b> %s\n<b>IP:</b> %s\n<b>Time:</b> %s",
		escapeHTML(serverName),
		escapeHTML(username),
		escapeHTML(ip),
		escapeHTML(loginTime),
	)

	_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	return err
}

func generateSecureToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
