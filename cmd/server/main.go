// cmd/server/main.go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mailcast-service-v2/internal/infrastructure/config"
	"mailcast-service-v2/internal/infrastructure/oauth"
	"mailcast-service-v2/internal/infrastructure/persistence"
	"mailcast-service-v2/internal/interface/gmail"
	"mailcast-service-v2/internal/interface/repository"
	"mailcast-service-v2/internal/usecase"
	"mailcast-service-v2/pkg/logger"
	"mailcast-service-v2/pkg/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Create logger
	log := logger.NewLogger()
	log.Info("Starting Mailcast Service")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config", "error", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up MongoDB connection
	log.Info("Connecting to MongoDB")
	mongoClient, db, err := persistence.NewMongoClient(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB", "error", err)
	}

	// Set up repositories
	emailRepo := repository.NewMongoEmailRepository(db)
	whatsappRepo := repository.NewWhatsappRepository(log)
	flightRecordRepo := repository.NewMongoFlightRecordRepository(db)
	airlineRepository := repository.NewMongoAirlineRepository(db)
	timezoneRepository := repository.NewMongoTimezoneRepository(db)

	// Initialize email parser V2
	emailParserV2 := utils.NewEmailParserV2(timezoneRepository, log)

	// Initialize flight processor V2
	flightProcessorV2 := usecase.NewFlightProcessorV2(
		airlineRepository,
		timezoneRepository,
		emailRepo,
		whatsappRepo,
		flightRecordRepo,
		log,
		emailParserV2,
	)

	// Set up Gmail OAuth
	gmailOAuth := oauth.NewGmailOAuth(
		cfg.GmailClientID,
		cfg.GmailClientSecret,
		cfg.GmailRefreshToken,
		log,
	)
	tokenSource := gmailOAuth.GetTokenSource(ctx)

	// Set up Gmail service
	gmailService, err := gmail.NewGmailService(ctx, tokenSource, emailRepo, log, cfg.GmailPollInterval)
	if err != nil {
		log.Fatal("Failed to create Gmail service", "error", err)
	}

	// Start Gmail polling in a goroutine
	go gmailService.StartPolling(ctx)

	// Start email processor in a goroutine
	go func() {
		processTicker := time.NewTicker(30 * time.Second)
		defer processTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("Email processor stopped")
				return
			case <-processTicker.C:
				log.Info("Processing pending emails")
				if err := flightProcessorV2.ProcessPendingEmails(ctx); err != nil {
					log.Error("Error processing emails", "error", err)
				}
			}
		}
	}()

	// Set up HTTP server for metrics
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Healthy"))
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Info("Starting HTTP server", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Info("Received signal", "signal", sig)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", "error", err)
	}

	cancel() // Cancel the context to stop all goroutines

	// Disconnect from MongoDB
	if err := mongoClient.Disconnect(ctx); err != nil {
		log.Error("MongoDB disconnect error", "error", err)
	}

	log.Info("Mailcast Service stopped")
}
