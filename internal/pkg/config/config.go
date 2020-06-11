package config

import (
	"os"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"

	cermaFile "github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/file"
)

type ArgsConfig struct {
	SecretsPath  string
	ProjectID    string
	Interval     time.Duration
	EarlyRenewal time.Duration
	VaultConf    *VaultConfig
	LogConf      *LogConfig
	TLSConf      *TLSConfig
}

type VaultConfig struct {
	RoleName string
	Address  string
}

type LogConfig struct {
	Level  string
	Format string
}

type TLSConfig struct {
	CACertPath string
	CertPath   string
	KeyPath    string
}

var DefaultConfig = &ArgsConfig{
	SecretsPath:  "v1.1/cermati/infra/gcp-cermati/infrastructure-260106/key/cermati-infra-gcslister-gcslisterworker",
	ProjectID:    "infrastructure-260106",
	Interval:     1 * time.Minute,
	EarlyRenewal: 2 * time.Minute,
	VaultConf: &VaultConfig{
		Address:  "https://vault-test.cermati.com:8443",
		RoleName: "cermati-infra-gcslister-gcslisterworker",
	},
	LogConf: &LogConfig{
		Level:  "debug",
		Format: "text",
	},
	TLSConf: &TLSConfig{
		CACertPath: "$PKICTL_CA_CERT_FILE",
		CertPath:   "$PKICTL_CERT_FILE",
		KeyPath:    "$PKICTL_KEY_FILE",
	},
}

func ValidateFilePathValue(path string) (string, error) {
	expandedPath, err := homedir.Expand(os.ExpandEnv(path))
	if err != nil {
		return "", errors.Wrapf(err, "invalid path")
	}

	if !cermaFile.IsFileExists(expandedPath) {
		return "", errors.Errorf("file not found in %s", expandedPath)
	}
	return expandedPath, nil
}
