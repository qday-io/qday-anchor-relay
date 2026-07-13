// Command relay runs the QDAY2 ↔ Abelian anchor pipeline: poll DA commits,
// anchor them into Abelian memos, wait for confirmations, and record results
// in QDayAnchorRegistry (spec §1).
package main

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	abelian "github.com/pqabelian/abelian-sdk-go-v2/abelian"

	"github.com/qday/qday-anchor-relay/relay/internal/abel"
	"github.com/qday/qday-anchor-relay/relay/internal/admin"
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
	// Self-migrate on startup (embedded SQL; idempotent). Image-based deploys
	// need no separate migrate service.
	if err := store.Migrate(cfg.Postgres.DSN()); err != nil {
		return err
	}
	log.Info("database migrations up to date")

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
	commitSource, err := da.NewEventLogCommitSource(ethCli, common.HexToAddress(cfg.DA.CommitContract), cfg.DA.RollupID)
	if err != nil {
		return err
	}
	poller := da.NewPoller(commitSource, storeJobSink{st}, cfg.DA.ConfirmDepth, cfg.DA.StartBlock, log)
	if cfg.DA.RollupID == 0 {
		// batch_number is UNIQUE in the DB; a rollup manager multiplexes
		// rollups, so without the filter the first event for batch N from ANY
		// rollup wins silently.
		log.Warn("DA_ROLLUP_ID is 0 (accept all rollups) — set QDAY2's rollup id before production use")
	}

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

	// Wallet-derived facts used by both ANS registration and the admin API.
	fixedAddr, hasFixed, err := abel.FixedWalletAddress(abel.WalletConfig{
		Network:        cfg.Abel.Network,
		Mnemonic:       cfg.Abel.WalletMnemonic,
		KeyFile:        cfg.Abel.WalletKeyFile,
		AccountPrivacy: cfg.Abel.AccountPrivacy,
	})
	if err != nil {
		return err
	}
	if !hasFixed {
		fixedAddr = nil
	}

	// ANS registration is independent of the admin API: the funding address
	// must resolve in wallets regardless of dev tooling.
	var ans admin.ANSRegistrar
	if cfg.Abel.ANSURL != "" {
		ansClient := abel.NewANSClient(cfg.Abel.ANSURL)
		ans = ansClient
		if fixedAddr != nil {
			go func() {
				regCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				short, err := ansClient.Register(regCtx, hex.EncodeToString(fixedAddr))
				if err != nil {
					log.Error("ans: funding address registration failed", "err", err)
					return
				}
				if len(short) > 16 {
					short = short[:16] + "…"
				}
				log.Info("ans: funding address registered", "short", short)
			}()
		}
	}

	// Internal admin API (dev tooling; never published outside compose).
	if cfg.AdminAddr != "" {
		netID := abelian.NetName2NetID[cfg.Abel.Network]
		privacy := abelian.AccountPrivacyLevelPseudonym
		if cfg.Abel.AccountPrivacy == "full-privacy" {
			privacy = abelian.AccountPrivacyLevelFullPrivacy
		}
		walletSource := "mnemonic"
		if cfg.Abel.WalletMnemonic == "" {
			walletSource = "keyfile"
		}
		adminSrv := &http.Server{
			Addr:    cfg.AdminAddr,
			Handler: admin.NewServer(netID, privacy, account, walletSource, fixedAddr, ans, log).Handler(),
		}
		go func() {
			log.Info("admin api listening", "addr", cfg.AdminAddr)
			if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("admin api failed", "err", err)
			}
		}()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = adminSrv.Shutdown(shutdownCtx)
		}()
	}

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
