package tgbot

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"smOwd/logs"

	"strconv"
	"syscall"
	"time"

	"fmt"
	"smOwd2/animes"
	"smOwd2/misc"
	"smOwd2/subscriptions"
	"smOwd2/users"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type handleUpdateMode int

const (
	handleUpdateModeInit handleUpdateMode = iota
	handleUpdateModeBasic
)

func (c handleUpdateMode) String() string {
	return [...]string{"Basic", "Search"}[c]
}

type sessionData struct {
	handleUpdateModeField handleUpdateMode
	sliceAnime            []animes.Anime
	lastTgMsg             tgbotapi.MessageConfig
	sliceSubscriptions    []subscriptions.Subscription
	test                  bool
}

type userHandle struct {
	sessionDataField sessionData
}

var mapIdUserHandle = make(map[int]*userHandle)

func clearSessionData(userID int) {
	userHandlePtr := mapIdUserHandle[userID]
	userHandlePtr.sessionDataField.lastTgMsg = tgbotapi.MessageConfig{}
	userHandlePtr.sessionDataField.sliceAnime = []animes.Anime{}
	userHandlePtr.sessionDataField.sliceSubscriptions = []subscriptions.Subscription{}
}

func checkAndAddUserToMap(ctx context.Context, userID int) {
	logger := logs.DefaultFromCtx(ctx)

	_, exists := mapIdUserHandle[userID]

	if !exists {
		logger.Info("Adding user handle to map", "ID", userID)
		sessionDataObj := sessionData{handleUpdateModeField: handleUpdateModeInit,
			sliceAnime: nil}
		userHandleObj := userHandle{sessionDataField: sessionDataObj}
		mapIdUserHandle[userID] = &userHandleObj
	}
}

func createInlineKeyboard(listText []string,
	maxCols int) tgbotapi.InlineKeyboardMarkup {
	// Create an empty slice to hold the keyboard rows
	var keyboard [][]tgbotapi.InlineKeyboardButton

	// Create buttons and group them into rows of maxCols
	for i := 0; i < len(listText); i += maxCols {
		// Get the slice of text for the current row
		end := i + maxCols
		if end > len(listText) {
			end = len(listText)
		}
		row := listText[i:end]

		// Create InlineKeyboardButton for each text in the row
		var buttons []tgbotapi.InlineKeyboardButton
		for _, text := range row {
			button := tgbotapi.NewInlineKeyboardButtonData(text, text) // Using text as callback data
			buttons = append(buttons, button)
		}

		// Add the row of buttons to the keyboard
		keyboard = append(keyboard, buttons)
	}

	// Return the inline keyboard markup
	return tgbotapi.NewInlineKeyboardMarkup(keyboard...)
}

func generalMessage(chatID int, notificationsEnabled bool) *tgbotapi.MessageConfig {
	msgStr := "Please choose one of the options:\n"
	msg := tgbotapi.NewMessage(int64(chatID), msgStr)

	var keyboard tgbotapi.InlineKeyboardMarkup

	if notificationsEnabled {
		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Disable notifications", "disable"),
			),
		)
	} else {
		keyboard = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Enable notifications", "enable"),
			),
		)
	}

	msg.ReplyMarkup = keyboard

	return &msg
}

// Unified function to handle both messages and inline button callbacks
func handleUpdate(ctx context.Context, bot *tgbotapi.BotAPI,
	update tgbotapi.Update, db *sql.DB) {

	// Retrieve the logger from the context
	logger, ok := ctx.Value("logger").(*logs.Logger)
	if !ok {
		// If the logger is not found in the context, fall back to a default logger
		logger = logs.New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	}

	var user *users.User
	var chatID int
	var tgbotUser *tgbotapi.User

	var messageText string

	if update.Message != nil {

		tgbotUser = update.Message.From

		chatID = int(update.Message.Chat.ID)
		messageText = misc.RemoveFirstCharIfPresent(update.Message.Text, '/')
	} else if update.CallbackQuery != nil { // Handle inline button callback queries
		tgbotUser = update.CallbackQuery.Message.From

		chatID = int(update.CallbackQuery.Message.Chat.ID)
		messageText = update.CallbackQuery.Data
		defer bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, "Done"))
	}
	user = users.FindByChatID(ctx, db, chatID)

	if user == nil {
		logger.Info("New user", "tg_name", tgbotUser.UserName)
		user = &users.User{
			TelegramID:   tgbotUser.ID,
			ChatID:       chatID,
			FirstName:    tgbotUser.FirstName,
			LastName:     tgbotUser.LastName,
			UserName:     tgbotUser.UserName,
			LanguageCode: tgbotUser.LanguageCode,
			IsBot:        tgbotUser.IsBot,
			Enabled:      true,
		}
		user_id, err := users.Add(ctx, db, user)

		if err != nil {
			logger.Fatal("Error adding user to db",
				"Telegram ID", user.TelegramID,
				"error", err)
		}

		user.ID = user_id
	} else {
		logger.Info("Found user in db", "tg_name", tgbotUser.UserName)
	}

	checkAndAddUserToMap(ctx, user.ID)

	userHandlePtr := mapIdUserHandle[user.ID]

	session := &userHandlePtr.sessionDataField
	updateMode := &session.handleUpdateModeField

	if *updateMode == handleUpdateModeInit {
		logger.Info("Update handle mode Initial", "tgname", user.UserName)

		msg := generalMessage(chatID, user.Enabled)
		bot.Send(tgbotapi.NewMessage(int64(chatID), "Started!"))

		bot.Send(msg)

		*updateMode = handleUpdateModeBasic

	} else if *updateMode == handleUpdateModeBasic {
		clearSessionData(user.ID)

		logger.Info("Update handle mode Basic", "tgname", user.UserName)

		if messageText == "enable" {
			err := users.Enable(ctx, db, user.ID)

			if err == nil {
				logger.Info("Enabled notifications",
					"Telegram username", user.UserName)

				bot.Send(tgbotapi.NewMessage(int64(chatID), "Enabled notifications"))
				bot.Send(generalMessage(chatID, true))
			} else {
				logger.Error("Failed to enable notifications",
					"Telegram username", user.UserName,
					"error", err)

				bot.Send(generalMessage(chatID, user.Enabled))
			}

		} else if messageText == "disable" {
			err := users.Disable(ctx, db, user.ID)

			if err == nil {
				logger.Info("Disabled notifications",
					"Telegram username", user.UserName)

				bot.Send(tgbotapi.NewMessage(int64(chatID), "Disabled notifications"))
				bot.Send(generalMessage(chatID, false))

			} else {
				logger.Error("Failed to disable notifications",
					"Telegram username", user.UserName,
					"error", err)

				bot.Send(generalMessage(chatID, user.Enabled))
			}
		}
	}
}

var testReleased = false
var testNewEpisode = false

func processUsers(ctx context.Context, db *sql.DB, bot *tgbotapi.BotAPI) {
	logger := logs.DefaultFromCtx(ctx)

	sliceSubscriptions := subscriptions.SelectAll(ctx, db)

	if len(sliceSubscriptions) == 0 {
		logger.Info("No subscrtiptions in db")
	} else {
		for _, s := range sliceSubscriptions {
			user := users.FindByTelegramID(ctx, db, s.TelegramID)

			if !user.Enabled {
				continue
			}

			// userHandle, ok := mapIdUserHandle[user.ID]
			// session := &userHandle.sessionDataField
			// updateMode := &session.handleUpdateModeField

			sliceAnime, err := animes.SearchAnimeByShikiIDs(ctx, []string{s.ShikiID})

			var a animes.Anime

			if err != nil {
				logger.Error("Error searching anime by shiki ID",
					"Shiki ID", s.ShikiID,
					"error", err)

			} else if sliceAnime == nil {
				logger.Error("Error: no anime found",
					"Shiki ID", s.ShikiID)
			} else {
				a = sliceAnime[0]
				logger.Info("Found anime", "Anime name", a.English)

				chatID := user.ChatID

				if a.Status == "released" {
					logger.Info("Anime status RELEASED!", "Anime name", a.English)
					outputMsg := tgbotapi.NewMessage(int64(chatID),
						fmt.Sprintf("%s\n%s \nStatus Released!"+
							"\nYou are no longer subscribed to this anime",
							a.English, a.URL))

					outputMsg.DisableWebPagePreview = true

					err = subscriptions.Remove(ctx, db, s.ID)

					if err != nil {
						logger.Error("Error removing subscription",
							"Telegram ID", s.TelegramID,
							"Shiki ID", s.ShikiID)
					} else {
						bot.Send(outputMsg)
					}
				} else if a.EpisodesAired > s.LastEpisodeNotified {
					logger.Info("New Episode!",
						"Anime name", a.English,
						"Episode", a.EpisodesAired)

					outputMsg := tgbotapi.NewMessage(int64(chatID),
						fmt.Sprintf("%s\n%s \nNew Episode %d!",
							a.English, a.URL, a.EpisodesAired))

					outputMsg.DisableWebPagePreview = true

					bot.Send(outputMsg)

					subscriptions.SetLastEpisode(ctx, db, s.ID, a.EpisodesAired)
				} else if testReleased {
					logger.Info("Anime status RELEASED! ----TEST----", "Anime name", a.English)
					outputMsg := tgbotapi.NewMessage(int64(chatID),
						fmt.Sprintf("%s\n%s \nStatus Released!"+
							"\nYou are no longer subscribed to this anime",
							a.English, a.URL))
					outputMsg.DisableWebPagePreview = true

					// err = subscriptions.Remove(ctx, db, s.ID)

					if err != nil {
						logger.Error("Error removing subscription ----TEST----",
							"Telegram ID", s.TelegramID,
							"Shiki ID", s.ShikiID)
					} else {
						bot.Send(outputMsg)
					}

					ss := subscriptions.FindAll(ctx, db, user.TelegramID)

					for _, s := range ss {
						logger.Info("Subscrtiption",
							"Telegram ID", s.TelegramID,
							"Shiki ID", s.ShikiID)
					}

					testReleased = false

				} else if testNewEpisode {
					logger.Info("New Episode! ----TEST----",
						"Anime name", a.English,
						"Episode", a.EpisodesAired)

					outputMsg := tgbotapi.NewMessage(int64(chatID),
						fmt.Sprintf("%s\n%s \nNew Episode %d!",
							a.English, a.URL, a.EpisodesAired))
					outputMsg.DisableWebPagePreview = true

					bot.Send(outputMsg)

					// subscriptions.SetLastEpisode(ctx, db, s.ID, a.EpisodesAired)

					ss := subscriptions.FindAll(ctx, db, user.TelegramID)

					for _, s := range ss {
						logger.Info("Subscrtiption",
							"Telegram ID", s.TelegramID,
							"Shiki ID", s.ShikiID)
					}

					testNewEpisode = false
				}

			}
		}
	}

}

func StartBotAndHandleUpdates(ctx context.Context, cancel context.CancelFunc,
	db *sql.DB) {
	logger, ok := ctx.Value("logger").(*logs.Logger)
	if !ok {
		logger = logs.New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		logger.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logger.Fatal("Failed to initialize bot", "error", err)
	}

	// Set bot to debug mode (optional)
	bot.Debug = true
	logger.Info("Authorized on account", "UserName", bot.Self.UserName)

	// Configure the update channel (long polling)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 160

	// Get updates (messages and callback queries) from Telegram
	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		logger.Fatal("Failed to get updates", "error", err)
	}
	// Create a channel to synchronize processUsers with handleUpdate
	processUsersChan := make(chan bool)

	// Set up a goroutine to listen for OS signals and trigger shutdown
	go func() {
		// Channel for receiving OS termination signals (e.g., CTRL+C or kill command)
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
		<-signalChan // Block until a signal is received
		logger.Info("Received shutdown signal, shutting down gracefully...")
		cancel() // Trigger the shutdown process
	}()

	// Start a goroutine to handle periodic user processing every second
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				processUsersChan <- true // Send signal to process users every second
			case <-ctx.Done():
				logger.Info("Stopping user processing due to shutdown signal.")
				return
			}
		}
	}()

	// Main loop: process incoming updates and handle periodic user processing
	for {
		select {
		case update := <-updates:
			// Handle incoming updates (messages and callback queries)
			handleUpdate(ctx, bot, update, db)
		case <-processUsersChan:
			// This block is triggered every 1 second to process users
			processUsers(ctx, db, bot)
		case <-ctx.Done():
			// Graceful shutdown of the main loop
			logger.Info("Shutting down the bot.")
			return
		}
	}
}
