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
	mongoRepo "mailcast-service-v2/internal/interface/repository"
	"mailcast-service-v2/pkg/logger"
	"mailcast-service-v2/pkg/utils"

	airlineRepo "mailcast-service-v2/internal/interface/repository"
	timezoneRepo "mailcast-service-v2/internal/interface/repository"
	flightUsecase "mailcast-service-v2/internal/usecase"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

	// Get database
	// db := persistence.GetDatabase(mongoClient, cfg.MongoDB)

	gormDB, err := gorm.Open(postgres.Open(cfg.PostgresURI), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL", "error", err)
	}

	// Set up airline and timezone repositories
	airlineRepository := airlineRepo.NewGormAirlineRepository(gormDB)
	timezoneRepository := timezoneRepo.NewGormTimezoneRepository(gormDB)

	// Set up repositories
	emailRepo := mongoRepo.NewMongoEmailRepository(db)
	whatsappRepo := mongoRepo.NewWhatsappRepository(log)
	flightRecordRepo := mongoRepo.NewMongoFlightRecordRepository(db)

	emailParserV2 := utils.NewEmailParserV2(timezoneRepository, log)
	flightProcessorV2 := flightUsecase.NewFlightProcessorV2(airlineRepository, timezoneRepository, emailRepo, whatsappRepo, flightRecordRepo, log, emailParserV2)

	// // Set up Gmail OAuth
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
