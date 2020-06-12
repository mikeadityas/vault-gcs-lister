package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/cvault"
	"github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/log"
	"github.com/mikeadityas/vault-gcs-lister/internal/pkg/config"
	"github.com/mikeadityas/vault-gcs-lister/internal/pkg/gcp"
	"github.com/mikeadityas/vault-gcs-lister/internal/pkg/gcs"
	"github.com/mikeadityas/vault-gcs-lister/internal/pkg/vault"
	"github.com/pkg/errors"
)

func main() {
	argsConfig := getArgsConfig()

	initLogger(argsConfig.LogConf)

	ctx := context.Background()

	vaultLeaseMgr, vaultDaemon := initVault(ctx, argsConfig.VaultConf, argsConfig.TLSConf)

	vaultRefreshPeriod := time.Duration(vaultLeaseMgr.Client().TTL()) * time.Second
	vaultRenewTime := time.Now().Add(vaultRefreshPeriod)
	log.Logger.Sugar().Infow("Next Vault token renew", "renew_time", vaultRenewTime.Format(time.RFC3339))

	gcpLeaseMgr, gcpDaemon := initGCP(
		ctx,
		argsConfig.SecretsPath,
		vaultLeaseMgr.Client(),
		argsConfig.EarlyRenewal,
	)

	vaultLeaseMgr.Register(gcpLeaseMgr)
	defer vaultLeaseMgr.Deregister(gcpLeaseMgr)

	gcsBucketListerSvc, gcsDaemon := initGCS(
		ctx,
		argsConfig.ProjectID,
		gcpLeaseMgr,
		argsConfig.Interval,
	)

	gcpLeaseMgr.Register(gcsBucketListerSvc)

	log.Logger.Sugar().Info("Starting Vault daemon...")
	if err := vaultDaemon.Start(); err != nil {
		log.Logger.Sugar().Fatalw("failed starting Vault daemon", "err", err.Error())
	}
	log.Logger.Sugar().Info("Vault daemon started")
	defer vaultDaemon.Stop()

	log.Logger.Sugar().Info("Starting GCP daemon...")
	if err := gcpDaemon.Start(); err != nil {
		log.Logger.Sugar().Fatalw("failed starting GCP daemon", "err", err.Error())
	}
	log.Logger.Sugar().Info("GCP daemon started")
	defer gcpDaemon.Stop()

	log.Logger.Sugar().Info("Starting GCS daemon...")
	if err := gcsDaemon.Start(); err != nil {
		log.Logger.Sugar().Fatalw("failed starting GCS daemon", "err", err.Error())
	}
	log.Logger.Sugar().Info("GCS daemon started")
	defer gcsDaemon.Stop()

	waitForSignal()
}

func getArgsConfig() *config.ArgsConfig {
	cfg := config.LoadFromFile("config.yml")

	flag.StringVar(&cfg.SecretsPath, "secrets-path", cfg.SecretsPath, "GCP secrets engine path")
	flag.StringVar(&cfg.ProjectID, "project-id", cfg.ProjectID, "GCP project ID")
	flag.DurationVar(&cfg.Interval, "interval", cfg.Interval, "The interval to list the GCS bucket")
	flag.DurationVar(&cfg.EarlyRenewal, "early-renewal", cfg.EarlyRenewal, "The early renewal duration")

	flag.StringVar(&cfg.VaultConf.Address, "vault.address", cfg.VaultConf.Address, "Vault address")
	flag.StringVar(&cfg.VaultConf.RoleName, "vault.role", cfg.VaultConf.RoleName, "Vault role name")

	flag.StringVar(&cfg.LogConf.Format, "log.level", cfg.LogConf.Format, "Log level (debug, info, warning, error)")
	flag.StringVar(&cfg.LogConf.Level, "log.format", cfg.LogConf.Level, "Log format (text, json)")

	flag.StringVar(&cfg.TLSConf.CACertPath, "tls.ca", cfg.TLSConf.CACertPath, "Location of CA cert file")
	flag.StringVar(&cfg.TLSConf.CertPath, "tls.cert", cfg.TLSConf.CertPath, "Location of cert file")
	flag.StringVar(&cfg.TLSConf.KeyPath, "tls.key", cfg.TLSConf.KeyPath, "Location of key file")

	flag.Parse()

	return cfg
}

func initLogger(logConf *config.LogConfig) {
	switch logConf.Format {
	case "text":
		log.InitLogfmtLogger(logConf.Level)
	case "json":
		log.InitLogger(logConf.Level)
	default:
		log.InitLogfmtLogger(logConf.Level)
	}
}

func initVault(
	ctx context.Context,
	vaultConf *config.VaultConfig,
	tlsConf *config.TLSConfig,
) (*vault.VaultLeaseManager, vault.Daemon) {
	log.Logger.Sugar().Info("Validating TLS config")
	if err := validateTLSConfig(tlsConf); err != nil {
		log.Logger.Sugar().Fatal(err)
	}
	log.Logger.Sugar().Info("TLS config valid!")

	log.Logger.Sugar().Info("Initializing Vault lease manager")
	if err := vault.NewVaultLeaseManager(vaultConf, tlsConf); err != nil {
		log.Logger.Sugar().Fatal(err)
	}
	log.Logger.Sugar().Info("Vault lease manager initialized")

	vaultLeaseMgr := vault.GetInstance()

	vaultCtx, vaultCancel := context.WithCancel(ctx)
	vaultRefreshPeriod := time.Duration(vaultLeaseMgr.Client().TTL()) * time.Second
	vaultDaemon := vaultLeaseMgr.Daemonize(vaultCtx, vaultCancel, vaultRefreshPeriod)

	return vaultLeaseMgr, vaultDaemon
}

func initGCP(
	ctx context.Context,
	secretsPath string,
	vaultClient cvault.CVault,
	earlyRenewal time.Duration,
) (*gcp.GCPLeaseManager, gcp.Daemon) {
	log.Logger.Sugar().Info("Initializing GCP lease manager")
	gcpLeaseMgr := gcp.NewGCPLeaseManager("gcp-01", secretsPath, vaultClient)
	log.Logger.Sugar().Info("GCP lease manager initialized")

	gcpCtx, gcpCancel := context.WithCancel(ctx)
	gcpRefreshPeriod := time.Duration(gcpLeaseMgr.GetTTL()) * time.Second
	gcpDaemon := gcpLeaseMgr.Daemonize(gcpCtx, gcpCancel, gcpRefreshPeriod, earlyRenewal)

	return gcpLeaseMgr, gcpDaemon
}

func initGCS(
	ctx context.Context,
	projectID string,
	gcpLeaseMgr *gcp.GCPLeaseManager,
	interval time.Duration,
) (*gcs.BucketListerService, gcs.Daemon) {
	gcsCtx, gcsCancel := context.WithCancel(ctx)

	log.Logger.Sugar().Info("Initializing GCS bucket lister service")
	gcsBucketListerSvc := gcs.NewBucketListerService(
		gcsCtx,
		gcsCancel,
		"gcs-01",
		projectID,
		gcpLeaseMgr,
	)
	log.Logger.Sugar().Info("GCS bucket lister service initialized")

	return gcsBucketListerSvc, gcsBucketListerSvc.Daemonize(interval)
}

func validateTLSConfig(tlsConf *config.TLSConfig) error {
	var err error
	tlsConf.CACertPath, err = config.ValidateFilePathValue(tlsConf.CACertPath)
	if err != nil {
		return errors.Wrap(err, "invalid CA cert")
	}

	tlsConf.CertPath, err = config.ValidateFilePathValue(tlsConf.CertPath)
	if err != nil {
		return errors.Wrap(err, "invalid cert")
	}

	tlsConf.KeyPath, err = config.ValidateFilePathValue(tlsConf.KeyPath)
	if err != nil {
		return errors.Wrap(err, "invalid key")
	}

	return nil
}

func waitForSignal() {
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
