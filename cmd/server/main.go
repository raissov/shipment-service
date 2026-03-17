package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	pb "github.com/raissov/shipment-service/gen/shipment/v1"
	"github.com/raissov/shipment-service/internal/application"
	"github.com/raissov/shipment-service/internal/config"
	"github.com/raissov/shipment-service/internal/infrastructure/postgres"
	"github.com/raissov/shipment-service/internal/logger"
	grpchandler "github.com/raissov/shipment-service/internal/transport/grpc"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Log)

	// Database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to ping database")
	}
	log.Info().Msg("connected to database")

	// Infrastructure
	shipmentRepo := postgres.NewShipmentRepository(pool)
	eventRepo := postgres.NewStatusEventRepository(pool)
	txManager := postgres.NewTxManager(pool)

	// Application
	shipmentService := application.NewShipmentService(shipmentRepo, eventRepo, txManager, log)

	// Transport
	handler := grpchandler.NewHandler(shipmentService)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPC.Port))
	if err != nil {
		log.Fatal().Err(err).Str("port", cfg.GRPC.Port).Msg("failed to listen")
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpchandler.RequestIDInterceptor(),
			grpchandler.LoggingInterceptor(log),
		),
	)
	pb.RegisterShipmentServiceServer(grpcServer, handler)

	// Health check
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("shipment.v1.ShipmentService", healthpb.HealthCheckResponse_SERVING)

	reflection.Register(grpcServer)

	// Graceful shutdown with timeout
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Info().Str("signal", sig.String()).Msg("shutting down gRPC server")

		healthServer.SetServingStatus("shipment.v1.ShipmentService", healthpb.HealthCheckResponse_NOT_SERVING)

		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			log.Info().Msg("server stopped gracefully")
		case <-time.After(time.Duration(cfg.GRPC.ShutdownTimeout) * time.Second):
			log.Warn().Msg("graceful shutdown timed out, forcing stop")
			grpcServer.Stop()
		}
	}()

	log.Info().Str("port", cfg.GRPC.Port).Msg("gRPC server started")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal().Err(err).Msg("failed to serve")
	}
}
