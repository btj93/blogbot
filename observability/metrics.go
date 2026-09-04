package observability

import (
	"expvar"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// expvar counters (existing — keep as-is)
	ScrapeRunsTotal     = expvar.NewInt("blogbot_scrape_runs_total")
	ScrapeErrorsTotal   = expvar.NewInt("blogbot_scrape_errors_total")
	ScrapePostsFound    = expvar.NewInt("blogbot_scrape_posts_found")
	NextLiveChecksTotal = expvar.NewInt("blogbot_nextlive_checks_total")
	NextLiveErrors      = expvar.NewInt("blogbot_nextlive_errors_total")
	TelegramRetries     = expvar.NewInt("blogbot_telegram_retries_total")
	TelegramSendErrors  = expvar.NewInt("blogbot_telegram_send_errors_total")
	WebhookRequests     = expvar.NewInt("blogbot_webhook_requests_total")
	WSConnections       = expvar.NewInt("blogbot_ws_connections_total")
	WSReconnects        = expvar.NewInt("blogbot_ws_reconnects_total")
	StreamStarts        = expvar.NewInt("blogbot_stream_starts_total")

	// Prometheus metrics
	PromScrapeRunsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_scrape_runs_total",
		Help: "Total number of scrape runs",
	})
	PromScrapeErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "blogbot_scrape_errors_total",
		Help: "Total scrape errors by group",
	}, []string{"group"})
	PromScrapePostsFound = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "blogbot_scrape_posts_found_total",
		Help: "Total blog posts found by group",
	}, []string{"group"})
	PromScrapeDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "blogbot_scrape_duration_seconds",
		Help:    "Duration of scrape runs in seconds",
		Buckets: prometheus.DefBuckets,
	})
	PromNextLiveChecksTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_nextlive_checks_total",
		Help: "Total next_live check runs",
	})
	PromNextLiveDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "blogbot_nextlive_duration_seconds",
		Help:    "Duration of next_live check in seconds",
		Buckets: prometheus.DefBuckets,
	})
	PromTelegramRetries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_telegram_retries_total",
		Help: "Total Telegram API retries",
	})
	PromTelegramSendErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_telegram_send_errors_total",
		Help: "Total Telegram send failures after retries",
	})
	PromWebhookRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_webhook_requests_total",
		Help: "Total webhook requests received",
	})
	PromWSConnections = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_ws_connections_total",
		Help: "Total WebSocket connections established",
	})
	PromWSReconnects = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_ws_reconnects_total",
		Help: "Total WebSocket reconnection attempts",
	})
	PromStreamStarts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blogbot_stream_starts_total",
		Help: "Total Showroom stream start events detected",
	})
)
