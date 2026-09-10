package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"pandora-exporter/internal/config"
	"pandora-exporter/internal/pandora"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	listen := flag.String("web.listen-address", ":9349", "HTTP listen address")
	base := flag.String("pandora.base-url", "https://pro.p-on.ru", "Pandora API base URL")
	interval := flag.Duration("poll-interval", 5*time.Minute, "Background poll interval (minimum 10s)")
	timeout := flag.Duration("request-timeout", 20*time.Second, "Timeout per API request")
	maxAge := flag.Duration("max-data-age", 10*time.Minute, "Threshold for data_stale")
	coordinates := flag.Bool("collect-coordinates", true, "Export GPS coordinates")
	flag.Parse()
	if err := config.LoadEnv(".env"); err != nil {
		return err
	}
	cliInterval := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "poll-interval" {
			cliInterval = true
		}
	})
	pollInterval, err := config.PollInterval(os.Getenv("PANDORA_POLL_INTERVAL"), *interval, cliInterval)
	if err != nil {
		return err
	}
	username, password := os.Getenv("PANDORA_USERNAME"), os.Getenv("PANDORA_PASSWORD")
	if path := os.Getenv("PANDORA_PASSWORD_FILE"); path != "" {
		if password != "" {
			return errors.New("set only one of PANDORA_PASSWORD and PANDORA_PASSWORD_FILE")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return errors.New("cannot read password file")
		}
		password = strings.TrimRight(string(b), "\r\n")
	}
	if username == "" || password == "" {
		return errors.New("PANDORA_USERNAME and PANDORA_PASSWORD (or PANDORA_PASSWORD_FILE) are required")
	}
	if *timeout <= 0 || *maxAge <= 0 {
		return errors.New("invalid interval, timeout or maximum data age")
	}
	client, err := pandora.NewClient(*base, username, password, *timeout)
	if err != nil {
		return err
	}
	exporter := pandora.NewExporter(client, *coordinates, *maxAge)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		for {
			if err := exporter.Poll(ctx); err != nil && ctx.Err() == nil {
				log.Printf("Pandora poll failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollInterval):
			}
		}
	}()
	mux := http.NewServeMux()
	mux.Handle("/metrics", exporter)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
	server := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	log.Printf("Listening on %s; API poll interval: %s", *listen, pollInterval)
	err = server.ListenAndServe()
	stop()
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
