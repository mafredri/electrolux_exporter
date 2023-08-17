package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/mafredri/electrolux-ocp/ocpapi"
	"github.com/mafredri/electrolux_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

const (
	elxOneAppAPIKey       = "UcGF9pmUMKUqBL6qcQvTu4K4WBmQ5KJqJXprCTdc"
	elxOneAppClientID     = "ElxOneApp"
	elxOneAppClientSecret = "8UKrsKD7jH9zvTV7rz5HeCLkit67Mmj68FvRVTlYygwJYy4dW6KF2cVLPKeWzUQUd6KJMtTifFf4NkDnjI7ZLdfnwcPtTSNtYvbP7OzEkmQD9IjhMOf5e1zeAQYtt2yN"
	elxOneAppBrand        = "electrolux"
)

// Appended to by envOrDefault.
var availableEnvs []string

func main() {
	// Exporter flags.
	addr := flag.String("addr", envOrDefault("ELECTROLUX_EXPORTER_ADDR", ":8080"), "Listen on this address")

	// OCP API flags.
	apiKey := flag.String("api-key", envOrDefault("ELECTROLUX_EXPORTER_API_KEY", elxOneAppAPIKey), "API key")
	clientID := flag.String("client-id", envOrDefault("ELECTROLUX_EXPORTER_CLIENT_ID", elxOneAppClientID), "Client ID")
	clientSecret := flag.String("client-secret", envOrDefault("ELECTROLUX_EXPORTER_CLIENT_SECRET", elxOneAppClientSecret), "Client secret")
	brand := flag.String("brand", envOrDefault("ELECTROLUX_EXPORTER_BRAND", elxOneAppBrand), "Brand, one of: \"electrolux\", \"aeg\"")
	email := flag.String("email", envOrDefault("ELECTROLUX_EXPORTER_EMAIL", ""), "Email address (required)")
	password := flag.String("password", envOrDefault("ELECTROLUX_EXPORTER_PASSWORD", ""), "Password (required)")
	countryCode := flag.String("country", envOrDefault("ELECTROLUX_EXPORTER_COUNTRY_CODE", "FI"), "Country code where the exporter is running (used for API calls)")
	clientStateFile := flag.String("client-state-file", envOrDefault("ELECTROLUX_EXPORTER_CLIENT_STATE_FILE", "electrolux_exporter_client_state.json"), "Path to file where client state is stored (optional)")

	// Misc flags.
	vocMolecularWeight := flag.Float64(
		"voc-molecular-weight",
		must(strconv.ParseFloat(envOrDefault("ELECTROLUX_EXPORTER_VOC_MOLECULAR_WEIGHT", "30.026"), 64)),
		"Molecular weight of gas, in g/mol. Used for TVOC (ppb) conversion VOC density (μg/m^3). Formaldehyde is 30.026 g/mol.",
	)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(flag.CommandLine.Output(), "\nAvailable environment variables:\n")
		sort.Strings(availableEnvs) // For consistency with flag output.
		for _, env := range availableEnvs {
			fmt.Fprintf(flag.CommandLine.Output(), "  %s\n", env)
		}
	}

	flag.Parse()

	if *email == "" || *password == "" {
		flag.Usage()
		os.Exit(1)
	}

	var state ocpapi.State
	if _, err := os.Stat(*clientStateFile); err == nil {
		log.Printf("Restoring client state from %s", *clientStateFile)
		f, err := os.Open(*clientStateFile)
		if err == nil {
			err = json.NewDecoder(f).Decode(&state)
			if err != nil {
				log.Printf("Warning: decode client state: %v", err)
			} else {
				log.Println("Client state restored successfully")
			}
			f.Close()
		} else {
			log.Printf("Warning: open client state file: %v", err)
		}
	}

	client, err := ocpapi.New(ocpapi.Config{
		APIKey:       *apiKey,
		Brand:        *brand,
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		CountryCode:  *countryCode,
		State:        state,
	})
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	retryDelay := time.Minute
	for {
		log.Printf("Logging in as %s", *email)
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = client.Login(reqCtx, *email, *password)
		cancel()
		if err == nil {
			break
		}
		log.Printf("Login failed: %v", err)
		log.Printf("Retrying in %s...", retryDelay)
		select {
		case <-time.After(retryDelay):
		case <-ctx.Done():
			log.Println("Interrupt received, shutting down...")
			os.Exit(1)
		}
	}

	prometheus.MustRegister(version.NewCollector("electrolux_exporter"))
	collector := collector.NewCollector(client, &collector.Options{
		MolecularWeight: *vocMolecularWeight,
	})
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr: *addr,
	}

	done := make(chan struct{})
	go func() {
		defer stop()
		defer close(done)

		log.Printf("Listening on %s", *addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen and serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = srv.Shutdown(ctx)
	if err != nil {
		log.Printf("shutdown: %v", err)
	}
	<-done

	log.Printf("Writing client state to %s", *clientStateFile)
	state = client.State()
	f, err := os.Create(*clientStateFile)
	if err != nil {
		log.Fatalf("Error: create client state file: %v", err)
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(state)
	if err != nil {
		log.Fatalf("Error: encode client state: %v", err)
	}
	log.Println("Client state saved successfully")
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func envOrDefault(env, def string) string {
	availableEnvs = append(availableEnvs, env)
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}
