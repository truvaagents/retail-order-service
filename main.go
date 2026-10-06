package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/truvaagents/truva-g3/core"
	"github.com/truvaagents/truva-g3/telemetry"
)

const serviceName = "retail-order-service"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context) (runErr error) {
	build, err := loadBuildInfo()
	if err != nil {
		return err
	}
	token := os.Getenv("ORDER_API_TOKEN")
	if len(token) < 32 {
		return errors.New("ORDER_API_TOKEN must have at least 32 characters")
	}
	appConfig := core.DefaultConfig()
	if err := appConfig.LoadFromEnv(); err != nil {
		return errors.New("invalid application configuration")
	}
	config := telemetry.UseProfile(telemetry.ProfileDevelopment)
	config.ServiceName = serviceName
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		config.Endpoint = endpoint
	}
	if err := telemetry.Initialize(config); err != nil {
		return errors.New("checkout telemetry initialization failed")
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telemetry.Shutdown(shutdown)
	}()
	connection, err := core.ResolveRedisConnectionConfig(nil, os.LookupEnv)
	if err != nil {
		return errors.New("invalid checkout Redis configuration")
	}
	client, err := core.NewRedisUniversalClient(connection)
	if err != nil {
		return errors.New("checkout Redis initialization failed")
	}
	defer func() {
		if err := client.Close(); err != nil {
			runErr = errors.Join(runErr, errors.New("checkout Redis shutdown failed"))
		}
	}()
	keys, err := core.NewRedisKeyspace(os.Getenv("TRUVAG3_REDIS_NAMESPACE"))
	if err != nil {
		return errors.New("invalid checkout namespace")
	}
	logger := core.NewProductionLogger(appConfig.Logging, appConfig.Development, serviceName)
	if bridge, ok := logger.(interface{ EnableMetrics() }); ok {
		bridge.EnableMetrics()
	}
	if scoped, ok := logger.(core.ComponentAwareLogger); ok {
		logger = scoped.WithComponent("application/orders")
	}
	app := &service{store: &redisStore{client: client, keys: keys}, logger: logger, token: token, build: build}
	server := &http.Server{Addr: ":8398", Handler: telemetry.TracingMiddlewareWithConfig(serviceName,
		&telemetry.TracingMiddlewareConfig{ExcludedPaths: []string{"/health", "/ready"}})(app.routes()),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}
	finished := make(chan error, 1)
	go func() { finished <- server.ListenAndServe() }()
	select {
	case err := <-finished:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("checkout HTTP server failed")
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return errors.New("checkout shutdown timed out")
		}
		<-finished
	}
	return nil
}
