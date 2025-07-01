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
	"mailcast-service-v2/internal/infrastructure/router"
	"mailcast-service-v2/internal/interface/gmail"
	"mailcast-service-v2/internal/interface/repository"
	"mailcast-service-v2/internal/usecase"
	"mailcast-service-v2/pkg/logger"
	"mailcast-service-v2/pkg/utils"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	log := logger.NewLogger()
	log.Info("Starting Mailcast Service")

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config", "error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Info("Connecting to MongoDB")
	mongoClient, db, err := persistence.NewMongoClient(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB", "error", err)
	}

	// Repositories
	emailRepo := repository.NewMongoEmailRepository(db)
	whatsappRepo := repository.NewWhatsappRepository(log)
	flightRecordRepo := repository.NewMongoFlightRecordRepository(db)
	airlineRepository := repository.NewMongoAirlineRepository(db)
	timezoneRepository := repository.NewMongoTimezoneRepository(db)

	// Parsers
	emailParserV1 := utils.NewEmailParser(timezoneRepository, log)
	emailParserV2 := utils.NewEmailParserV2(timezoneRepository, log)

	// Processors
	flightProcessorV1 := usecase.NewFlightProcessor(
		airlineRepository,
		timezoneRepository,
		emailRepo,
		whatsappRepo,
		flightRecordRepo,
		log,
		emailParserV1,
	)
	flightProcessorV2 := usecase.NewFlightProcessorV2(
		airlineRepository,
		timezoneRepository,
		emailRepo,
		whatsappRepo,
		flightRecordRepo,
		log,
		emailParserV2,
	)

	subjectRouter := router.NewSubjectRouter(log)

	// Register FlightProcessor V1 for specific patterns
	subjectRouter.Register(usecase.NewFlightHandlerV1Adapter(
		flightProcessorV1,
		"FlightProcessorV1",
		[]string{"PREFLIGHT INFO GALILEO"},
	))

	// Register FlightProcessor V2 for different patterns
	subjectRouter.Register(usecase.NewFlightHandlerV2Adapter(
		flightProcessorV2,
		"FlightProcessorV2",
		[]string{"[PREFLIGHT GALILEO]"},
	))

	// Create email orchestrator
	orchestrator := usecase.NewEmailOrchestrator(emailRepo, subjectRouter, log)

	// Set up Gmail OAuth
	gmailOAuth := oauth.NewGmailOAuth(
		cfg.GmailClientID,
		cfg.GmailClientSecret,
		cfg.GmailRefreshToken,
		log,
	)
	tokenSource := gmailOAuth.GetTokenSource(ctx)

	// Create Gmail service V2 with orchestrator
	gmailService, err := gmail.NewGmailServiceV2(
		ctx, tokenSource, emailRepo, orchestrator, log, cfg.GmailPollInterval,
	)
	if err != nil {
		log.Fatal("Failed to create Gmail service", "error", err)
	}

	go gmailService.StartPolling(ctx)

	// Periodic cleanup of stuck emails
	// go func() {
	// 	cleanupTicker := time.NewTicker(5 * time.Minute)
	// 	defer cleanupTicker.Stop()

	// 	for {
	// 		select {
	// 		case <-ctx.Done():
	// 			return
	// 		case <-cleanupTicker.C:
	// 			if err := orchestrator.ProcessPendingEmails(ctx); err != nil {
	// 				log.Error("Failed to process pending emails", "error", err)
	// 			}
	// 		}
	// 	}
	// }()

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

	// Start HTTP server
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

	cancel()

	if err := mongoClient.Disconnect(ctx); err != nil {
		log.Error("MongoDB disconnect error", "error", err)
	}

	log.Info("Mailcast Service stopped")
}
