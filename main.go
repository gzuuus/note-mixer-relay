package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fiatjaf/eventstore/sqlite3"
	"github.com/fiatjaf/khatru"
	"github.com/fiatjaf/khatru/policies"
	"github.com/joho/godotenv"
	"github.com/nbd-wtf/go-nostr"
)

var version = "0.0.1"

type Config struct {
	RelayName          string
	RelayPubkey        string
	RelayPrivateKey    string
	RelayIcon          string
	RelayDescription   string
	AllowedKinds       []int
	DBPath             string
	Port               string
	WhitelistedPubkeys map[string]bool
	RebroadcastRelays  []string
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	InitTemplates()

	relay := config.InitializeRelay()

	db, err := initializeDatabase(config.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	setupRelayHandlers(relay, db, config)

	mux := setupHTTPHandlers(relay, config, db)

	log.Printf("Running on :%s\n", config.Port)
	if err := http.ListenAndServe(":"+config.Port, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func loadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file. Using environment variables.")
	}

	config := &Config{
		RelayName:          getEnv("RELAY_NAME", "Khatru Mixer Relay"),
		RelayPubkey:        getEnv("RELAY_PUBKEY", ""),
		RelayPrivateKey:    os.Getenv("RELAY_PRIVATE_KEY"),
		RelayIcon:          getEnv("RELAY_ICON", ""),
		RelayDescription:   getEnv("RELAY_DESCRIPTION", "A Nostr relay that mixes and anonymizes events"),
		DBPath:             getEnv("DB_PATH", "./khatru-sqlite.db"),
		Port:               getEnv("PORT", "3334"),
		WhitelistedPubkeys: make(map[string]bool),
	}

	if config.RelayPrivateKey == "" {
		return nil, fmt.Errorf("RELAY_PRIVATE_KEY not set in environment")
	}

	allowedKindsStr := getEnv("ALLOWED_KINDS", "1,30023")
	config.AllowedKinds = parseAllowedKinds(allowedKindsStr)

	whitelistStr := getEnv("WHITELISTED_PUBKEYS", "")
	whitelistedPubkeys := strings.Split(whitelistStr, ",")
	config.WhitelistedPubkeys = make(map[string]bool)
	for _, pubkey := range whitelistedPubkeys {
		trimmedPubkey := strings.TrimSpace(pubkey)
		if trimmedPubkey != "" {
			config.WhitelistedPubkeys[trimmedPubkey] = true
		}
	}

	rebroadcastRelaysStr := getEnv("REBROADCAST_RELAYS", "")
	config.RebroadcastRelays = strings.Split(rebroadcastRelaysStr, ",")
	for i, url := range config.RebroadcastRelays {
		config.RebroadcastRelays[i] = strings.TrimSpace(url)
	}

	return config, nil
}

func (c *Config) InitializeRelay() *khatru.Relay {
	relay := khatru.NewRelay()
	relay.Info.Name = c.RelayName
	relay.Info.PubKey = c.RelayPubkey
	relay.Info.Icon = c.RelayIcon
	relay.Info.Description = c.RelayDescription
	relay.Info.Software = "https://github.com/gzuuus/note-mixer-relay"
	relay.Info.Version = version
	return relay
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func initializeDatabase(dbPath string) (*sqlite3.SQLite3Backend, error) {
	db := &sqlite3.SQLite3Backend{DatabaseURL: dbPath}
	if err := db.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	return db, nil
}

func setupRelayHandlers(relay *khatru.Relay, db *sqlite3.SQLite3Backend, config *Config) {
	relay.StoreEvent = append(relay.StoreEvent, func(ctx context.Context, event *nostr.Event) error {
		return mixAndStoreEvent(ctx, event, relay, db, config.RelayPrivateKey, config.RebroadcastRelays)
	})
	relay.QueryEvents = append(relay.QueryEvents, db.QueryEvents)
	relay.CountEvents = append(relay.CountEvents, db.CountEvents)
	relay.DeleteEvent = append(relay.DeleteEvent, db.DeleteEvent)

	relay.RejectEvent = append(relay.RejectEvent,
		createRejectNonWhitelistedPubkeys(config.WhitelistedPubkeys),
		createRejectUnsupportedKinds(config.AllowedKinds),
		policies.RejectEventsWithBase64Media,
		policies.EventIPRateLimiter(5, time.Minute*1, 30),
	)

	relay.RejectFilter = append(relay.RejectFilter,
		policies.NoEmptyFilters,
		policies.NoComplexFilters,
	)

	relay.RejectConnection = append(relay.RejectConnection,
		policies.ConnectionRateLimiter(10, time.Minute*2, 30),
	)

	relay.OnConnect = append(relay.OnConnect, func(ctx context.Context) {
		log.Printf("New WebSocket connection established")
	})

	relay.OnDisconnect = append(relay.OnDisconnect, func(ctx context.Context) {
		log.Printf("WebSocket connection closed")
	})

	relay.OnEventSaved = append(relay.OnEventSaved, func(ctx context.Context, event *nostr.Event) {
		eventJSON, _ := json.Marshal(event)
		log.Printf("Saved mixed event: %s", string(eventJSON))
	})
}

func mixAndStoreEvent(ctx context.Context, event *nostr.Event, relay *khatru.Relay, db *sqlite3.SQLite3Backend, privateKey string, rebroadcastRelays []string) error {
	mixedEvent := &nostr.Event{
		Content:   event.Content,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      event.Kind,
		Tags:      event.Tags,
	}

	if err := mixedEvent.Sign(privateKey); err != nil {
		return fmt.Errorf("failed to sign mixed event: %w", err)
	}

	if err := db.SaveEvent(ctx, mixedEvent); err != nil {
		return fmt.Errorf("failed to save mixed event: %w", err)
	}

	relay.BroadcastEvent(mixedEvent)
	go rebroadcastEvent(mixedEvent, rebroadcastRelays)
	return nil
}

func createRejectUnsupportedKinds(allowedKinds []int) func(context.Context, *nostr.Event) (bool, string) {
	return func(ctx context.Context, event *nostr.Event) (bool, string) {
		for _, kind := range allowedKinds {
			if event.Kind == kind {
				return false, ""
			}
		}
		return true, fmt.Sprintf("event kind %d is not supported", event.Kind)
	}
}

func createRejectNonWhitelistedPubkeys(whitelist map[string]bool) func(context.Context, *nostr.Event) (bool, string) {
	return func(ctx context.Context, event *nostr.Event) (bool, string) {
		if len(whitelist) == 0 {
			return false, ""
		}
		if !whitelist[event.PubKey] {
			return true, "pubkey not whitelisted"
		}
		return false, ""
	}
}

func parseAllowedKinds(kindsStr string) []int {
	kindStrs := strings.Split(kindsStr, ",")
	kinds := make([]int, 0, len(kindStrs))

	for _, kindStr := range kindStrs {
		kind, err := strconv.Atoi(strings.TrimSpace(kindStr))
		if err == nil {
			kinds = append(kinds, kind)
		}
	}

	return kinds
}

func rebroadcastEvent(event *nostr.Event, relays []string) []string {
	ctx := context.Background()
	errors := make([]string, 0)
	for _, url := range relays {
		err := func(url string) error {
			relay, err := nostr.RelayConnect(ctx, url)
			if err != nil {
				return fmt.Errorf("failed to connect to relay %s: %v", url, err)
			}
			defer relay.Close()

			if err := relay.Publish(ctx, *event); err != nil {
				return fmt.Errorf("failed to publish event to relay %s: %v", url, err)
			}
			log.Printf("Event rebroadcasted to relay: %s", url)
			return nil
		}(url)
		if err != nil {
			errors = append(errors, err.Error())
		}
	}
	return errors
}

func setupHTTPHandlers(relay *khatru.Relay, config *Config, db *sqlite3.SQLite3Backend) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Header.Get("Upgrade") != "websocket" {
			http.Redirect(w, r, "/home", http.StatusSeeOther)
			return
		}
		relay.ServeHTTP(w, r)
	})

	mux.HandleFunc("/home", createHomeHandler(config))
	mux.HandleFunc("/submit-note", createSubmitNoteHandler(relay, config, db))

	return mux
}

func createHomeHandler(config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			RelayName        string
			RelayDescription string
			AllowedKinds     []int
			WhitelistEnabled bool
			Host             string
		}{
			RelayName:        config.RelayName,
			RelayDescription: config.RelayDescription,
			AllowedKinds:     config.AllowedKinds,
			WhitelistEnabled: len(config.WhitelistedPubkeys) > 0,
			Host:             r.Host,
		}
		RenderTemplate(w, "", data)
	}
}

func createSubmitNoteHandler(relay *khatru.Relay, config *Config, db *sqlite3.SQLite3Backend) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check if the relay is using a whitelist
		if len(config.WhitelistedPubkeys) > 0 {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`<p class="error">This relay is not open for public submissions</p>`))
			return
		}

		content := r.FormValue("content")
		if content == "" {
			w.Write([]byte(`<p class="error">Content cannot be empty</p>`))
			return
		}

		event := &nostr.Event{
			Kind:      1,
			CreatedAt: nostr.Timestamp(time.Now().Unix()),
			Content:   content,
			Tags:      nostr.Tags{},
		}

		err := event.Sign(config.RelayPrivateKey)
		if err != nil {
			w.Write([]byte(`<p class="error">Failed to sign event</p>`))
			return
		}

		err = mixAndStoreEvent(r.Context(), event, relay, db, config.RelayPrivateKey, config.RebroadcastRelays)
		if err != nil {
			w.Write([]byte(`<p class="error">Failed to store event</p>`))
			return
		}

		rebroadcastErrors := rebroadcastEvent(event, config.RebroadcastRelays)

		response := `<p class="success">Note submitted successfully!</p>`
		if len(rebroadcastErrors) > 0 {
			response += `<p class="warning">Rebroadcast issues:</p><ul>`
			for _, err := range rebroadcastErrors {
				response += fmt.Sprintf(`<li class="warning">%s</li>`, err)
			}
			response += `</ul>`
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(response))
	}
}
