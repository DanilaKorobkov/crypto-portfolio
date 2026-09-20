// Command portfolio is the composition root; configuration is private runtime data.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/DanilaKorobkov/crypto-portfolio/internal/adapters/rpc"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/adapters/storage"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/adapters/transport"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/adapters/zerion"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/application"
	"github.com/DanilaKorobkov/crypto-portfolio/internal/domain"
)

type rpcConfig struct {
	ChainID uint64 `json:"chain_id"`
	URL     string `json:"url"`
}

func decodeEnv(name string, target any) error {
	value := os.Getenv(name)
	if value == "" {
		value = "[]"
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return errors.New("invalid " + name)
	}
	return nil
}
func run() error {
	var configuredWallets []string
	var endpoints []rpcConfig
	if err := decodeEnv("WALLETS_JSON", &configuredWallets); err != nil {
		return err
	}
	if err := decodeEnv("RPC_CONFIG_JSON", &endpoints); err != nil {
		return err
	}
	if len(configuredWallets) > 20 || len(endpoints) > 8 {
		return errors.New("configuration limit exceeded")
	}
	reports, err := reportRepository(len(configuredWallets) > 0 || len(endpoints) > 0 || os.Getenv("ZERION_API_KEY") != "")
	if err != nil {
		return err
	}
	var wallets []domain.Address
	seen := map[domain.Address]bool{}
	for _, value := range configuredWallets {
		wallet, err := domain.ParseAddress(value)
		if err != nil {
			return err
		}
		if !seen[wallet] {
			wallets = append(wallets, wallet)
			seen[wallet] = true
		}
	}
	httpClient := transport.New()
	collector := application.Collector{Reports: reports}
	for _, endpoint := range endpoints {
		parsed, err := url.Parse(endpoint.URL)
		if err != nil || endpoint.ChainID == 0 || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
			return errors.New("invalid RPC_CONFIG_JSON endpoint")
		}
		collector.Readers = append(collector.Readers, rpc.Reader{Endpoint: endpoint.URL, ChainID: endpoint.ChainID, HTTP: httpClient})
	}
	if key := os.Getenv("ZERION_API_KEY"); key != "" {
		collector.Discoverer = zerion.New(httpClient, key)
	}
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(signalContext, 6*time.Minute)
	defer cancel()
	report, err := collector.Run(ctx, wallets)
	if err != nil {
		return errors.New("cannot persist diagnostics")
	}
	// Never print wallets, endpoints, provider payloads or secrets to public logs.
	fmt.Printf("Diagnostics saved locally; status=%s; portfolio_complete=false\n", report.Status)
	return nil
}

func reportRepository(hasConfiguration bool) (application.ReportRepository, error) {
	if recipient := os.Getenv("REPORT_RECIPIENT"); recipient != "" {
		return storage.NewAgeFile("output/diagnostics.json.age", recipient)
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" && hasConfiguration {
		return nil, errors.New("REPORT_RECIPIENT is required for configured collection in GitHub Actions")
	}
	return storage.JSONFile{Path: "output/diagnostics.json"}, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
