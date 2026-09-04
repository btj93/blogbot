// wstest: monitors all 46-group Showroom rooms via WebSocket.
// Captures raw messages to compare current vs fixed parsing logic.
// When a stream starts (type 104) or ends (type 101), sends to Telegram log chat.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/btj93/blogbot/config"
	"github.com/btj93/blogbot/showroom"
)

// Credentials come from the same configuration the server uses, so this tool
// reads whatever the environment or config file already provides rather than
// carrying its own copy.
var (
	botToken  string
	logChatID string
)

type showroomJSON map[string]map[string]struct {
	ID  string `json:"ID"`
	URL string `json:"URL"`
}

type room struct {
	Group  string
	Member string
	RoomID string
	URL    string
}

func main() {
	cfgPath := flag.String("config", "./config.toml", "path to config file")

	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal("load config: ", err)
	}

	botToken = cfg.Telegram.BotToken
	logChatID = cfg.Telegram.LogChatID

	if logChatID == "" {
		log.Fatal("telegram.log_chat_id is not configured")
	}

	// Everything that can exit fatally is done before the signal context is
	// installed: log.Fatal skips deferred calls, so a failure after this point
	// would leave cancel() unrun.
	raw, err := os.ReadFile("showroom.json")
	if err != nil {
		log.Fatal("read showroom.json:", err)
	}

	var data showroomJSON
	if err := json.Unmarshal(raw, &data); err != nil {
		log.Fatal("parse showroom.json:", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var rooms []room

	for group, members := range data {
		for member, info := range members {
			if info.ID != "" && info.URL != "" {
				rooms = append(rooms, room{
					Group:  group,
					Member: member,
					RoomID: info.ID,
					URL:    info.URL,
				})
			}
		}
	}

	// Optionally add extra rooms from command line: url_key room_id name
	if len(os.Args) >= 4 {
		rooms = append(rooms, room{
			Group:  "extra",
			Member: os.Args[3],
			RoomID: os.Args[2],
			URL:    "https://www.showroom-live.com/" + os.Args[1],
		})
	}

	log.Printf("Monitoring %d rooms", len(rooms))
	sendTelegram(
		ctx,
		fmt.Sprintf("[wstest] Starting monitor for %d rooms. Waiting for any stream start/end...", len(rooms)),
	)

	// Launch a goroutine per room
	for _, r := range rooms {
		go monitorRoom(ctx, r)
	}

	<-ctx.Done()
	log.Println("Shutting down")
}

func monitorRoom(ctx context.Context, r room) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := monitorOnce(ctx, r)
		if err != nil {
			log.Printf("[%s] error: %v — retrying in 60s", r.Member, err)

			select {
			case <-ctx.Done():
				return
			case <-time.After(60 * time.Second):
			}
		}
		// nil return = normal cycle, retry immediately
	}
}

func monitorOnce(ctx context.Context, r room) error {
	status, err := showroom.FetchRoomStatus(ctx, r.URL)
	if err != nil || status.BroadcastHost == "" {
		// Quiet retry — don't spam logs for all 82 rooms
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(60 * time.Second):
		}

		return nil
	}

	alreadyLive := strings.Contains(status.BroadcastKey, ":")
	if alreadyLive {
		log.Printf("[%s] ALREADY LIVE (key=%s) — connecting to capture end event", r.Member, status.BroadcastKey)
		sendTelegram(ctx, fmt.Sprintf("[wstest] %s is ALREADY LIVE — connecting to capture messages", r.Member))
	} else {
		log.Printf("[%s] offline (key=%s) — connecting to wait for stream start", r.Member, status.BroadcastKey)
	}

	// Connect WebSocket
	wsURL := fmt.Sprintf("wss://%s/", status.BroadcastHost)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("ws dial: %w", err)
	}
	defer conn.Close()

	subMsg := fmt.Sprintf("SUB\t%s", status.BroadcastKey)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(subMsg)); err != nil {
		return fmt.Errorf("ws send: %w", err)
	}

	// Ping/pong keepalive
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	pingDone := make(chan struct{})

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-pingDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	defer close(pingDone)

	if !alreadyLive {
		log.Printf("[%s] WebSocket connected, waiting for stream start...", r.Member)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("ws read: %w", err)
		}

		rawStr := string(msg)
		tabCount := strings.Count(rawStr, "\t")

		// === CURRENT PARSING (Go - SplitN 2) ===
		var currentType int

		partsCurrent := strings.SplitN(rawStr, "\t", 2)
		if len(partsCurrent) >= 2 {
			var wsMsg struct {
				T int `json:"t"`
			}
			if err := json.Unmarshal([]byte(partsCurrent[len(partsCurrent)-1]), &wsMsg); err == nil {
				currentType = wsMsg.T
			}
		}

		// === FIXED PARSING (Python-style - split all, take last) ===
		var fixedType int

		partsFixed := strings.Split(rawStr, "\t")
		lastPart := partsFixed[len(partsFixed)-1]

		var wsMsgFixed struct {
			T int `json:"t"`
		}
		if err := json.Unmarshal([]byte(lastPart), &wsMsgFixed); err == nil {
			fixedType = wsMsgFixed.T
		}

		detectedType := fixedType
		if detectedType == 0 {
			detectedType = currentType
		}

		// Only log interesting events (104=start, 101=end) in detail
		switch detectedType {
		case 104:
			log.Printf("\n=== STREAM STARTED: %s (%s) ===", r.Member, r.Group)
			log.Printf("  RAW: %q", rawStr)
			log.Printf("  TAB COUNT: %d", tabCount)
			log.Printf("  CURRENT PARSE (SplitN 2) type=%d", currentType)
			log.Printf("  FIXED PARSE (Split all)  type=%d", fixedType)

			if currentType != fixedType {
				log.Print("  !!! BUG CONFIRMED: current parse MISSED type 104 !!!")
			}

			m3u8, _ := showroom.FetchStreamingURL(ctx, r.RoomID)
			sendTelegram(ctx, fmt.Sprintf(
				"[wstest] 🔴 STREAM STARTED: #%s (%s)\n\n"+
					"URL: %s\nHLS: %s\n\n"+
					"RAW MSG: %s\n"+
					"TAB COUNT: %d\n"+
					"CURRENT PARSE (SplitN 2): type=%d\n"+
					"FIXED PARSE (Split all): type=%d\n\n"+
					"BUG: current=%d fixed=%d",
				r.Member, r.Group, r.URL, m3u8,
				truncate(rawStr, 300), tabCount,
				currentType, fixedType, currentType, fixedType))

			return nil

		case 101:
			log.Printf("\n=== STREAM ENDED: %s (%s) ===", r.Member, r.Group)
			log.Printf("  RAW: %q", rawStr)
			log.Printf("  TAB COUNT: %d", tabCount)
			log.Printf("  CURRENT PARSE (SplitN 2) type=%d", currentType)
			log.Printf("  FIXED PARSE (Split all)  type=%d", fixedType)

			sendTelegram(ctx, fmt.Sprintf(
				"[wstest] ⚫ STREAM ENDED: %s (%s)\n\n"+
					"RAW MSG: %s\n"+
					"TAB COUNT: %d\n"+
					"CURRENT PARSE: type=%d\n"+
					"FIXED PARSE: type=%d",
				r.Member, r.Group,
				truncate(rawStr, 300), tabCount,
				currentType, fixedType))

			// Wait 5 min then reconnect (same as production)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Minute):
			}

			return nil
		}
	}
}

func sendTelegram(ctx context.Context, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	body := fmt.Sprintf(`{"chat_id":%s,"text":%s}`, logChatID, jsonStr(text))

	// A bounded context so a hung Telegram request cannot stall the monitor,
	// and so shutdown cancels an in-flight send.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		log.Printf("Telegram request error: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Telegram send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Telegram HTTP %d", resp.StatusCode)
	}
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "..."
}
