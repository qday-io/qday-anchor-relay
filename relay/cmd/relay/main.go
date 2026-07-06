// Command relay runs the QDAY2 ↔ Abelian anchor pipeline: poll DA commits,
// anchor them into Abelian memos, wait for confirmations, and record results
// in QDayAnchorRegistry (spec §1).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/qday/qday-anchor-relay/relay/internal/abel"
	"github.com/qday/qday-anchor-relay/relay/internal/config"
	"github.com/qday/qday-anchor-relay/relay/internal/da"
	"github.com/qday/qday-anchor-relay/relay/internal/registry"
	"github.com/qday/qday-anchor-relay/relay/internal/store"
	"github.com/qday/qday-anchor-relay/relay/internal/worker"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	if err := run(log); err != nil {
		log.Error("relay exiting with error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Postgres ─────────────────────────────────────────
	st, err := store.New(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	// ── DA side ──────────────────────────────────────────
	ethCli, err := ethclient.DialContext(ctx, cfg.DA.RPCURL)
	if err != nil {
		return err
	}
	defer ethCli.Close()
	commitSource, err := da.NewEventLogCommitSource(ethCli, common.HexToAddress(cfg.DA.CommitContract))
	if err != nil {
		return err
	}
	poller := da.NewPoller(commitSource, storeJobSink{st}, cfg.DA.ConfirmDepth, cfg.DA.StartBlock, log)

	recorder, err := registry.NewRecorder(ethCli, common.HexToAddress(cfg.Registry.Address),
		cfg.Registry.RelayerPrivateKey, cfg.DA.ChainID)
	if err != nil {
		return err
	}

	// ── Abelian side ─────────────────────────────────────
	abelClient, err := abel.NewClient(cfg.Abel.RPCEndpoint, cfg.Abel.RPCUser, cfg.Abel.RPCPass, cfg.Abel.TLSCertPath)
	if err != nil {
		return err
	}
	account, err := abel.LoadWallet(abel.WalletConfig{
		Network:        cfg.Abel.Network,
		Mnemonic:       cfg.Abel.WalletMnemonic,
		KeyFile:        cfg.Abel.WalletKeyFile,
		AccountPrivacy: cfg.Abel.AccountPrivacy,
	})
	if err != nil {
		return err
	}
	log.Info("relay wallet loaded", "network", cfg.Abel.Network, "privacy", cfg.Abel.AccountPrivacy)

	rings := abel.NewSDKRingProvider(abelClient)
	scanner := abel.NewScanner(abelClient, account, storeCoinSink{st}, rings, cfg.Abel.ScanStartHeight, log)
	submitter := abel.NewSubmitter(abelClient, account, storeCoinSelector{st},
		rings, abel.NewSDKCrafter(), cfg.Abel.TxFeeNeutrino, log)
	confirmer := abel.NewConfirmer(abelClient, cfg.Abel.ConfirmDepth, 30*time.Minute, log)

	// ── Pipeline + workers ───────────────────────────────
	pipeline := worker.NewPipeline(st, submitter, confirmer, recorder,
		cfg.RetryMax, int(cfg.RetryBackoffBase.Seconds()), log)

	abelTick := 15 * time.Second // Abelian block cadence is minutes; 15s is plenty
	runner := worker.NewRunner(log,
		worker.Worker{Name: "da-poller", Interval: cfg.DA.PollInterval, Tick: poller.ScanOnce},
		worker.Worker{Name: "coin-scanner", Interval: abelTick, Tick: scanner.ScanOnce},
		worker.Worker{Name: "submitter", Interval: abelTick, Tick: pipeline.SubmitTick},
		worker.Worker{Name: "confirmer", Interval: abelTick, Tick: pipeline.ConfirmTick},
		worker.Worker{Name: "recorder", Interval: abelTick, Tick: pipeline.RecordTick},
	)

	log.Info("relay started",
		"da_contract", cfg.DA.CommitContract,
		"registry", cfg.Registry.Address,
		"abel_network", cfg.Abel.Network,
		"confirm_depth_da", cfg.DA.ConfirmDepth,
		"confirm_depth_abel", cfg.Abel.ConfirmDepth,
	)
	runner.Run(ctx)
	log.Info("relay stopped gracefully")
	return nil
}
