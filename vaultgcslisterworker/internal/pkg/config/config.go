package config

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	cermaFile "github.com/cermati/devops-toolkit/common-libs/toolkit-go/pkg/file"
)

type ArgsConfig struct {
	SecretsPath  string        `yaml:"secrets_path,omitempty"`
	ProjectID    string        `yaml:"project_id,omitempty"`
	Interval     time.Duration `yaml:"interval,omitempty"`
	EarlyRenewal time.Duration `yaml:"early_renewal,omitempty"`
	VaultConf    *VaultConfig  `yaml:"vault,omitempty"`
	LogConf      *LogConfig    `yaml:"log,omitempty"`
	TLSConf      *TLSConfig    `yaml:"tls,omitempty"`
}

type VaultConfig struct {
	RoleName string `yaml:"role_name,omitempty"`
	Address  string `yaml:"address,omitempty"`
}

type LogConfig struct {
	Level  string `yaml:"level,omitempty"`
	Format string `yaml:"format,omitempty"`
}

type TLSConfig struct {
	CACertPath string `yaml:"ca,omitempty"`
	CertPath   string `yaml:"cert,omitempty"`
	KeyPath    string `yaml:"key,omitempty"`
}

var defaultConfig = ArgsConfig{
	SecretsPath:  "v1.1/cermati/infra/gcp-cermati/infrastructure-260106/key/cermati-infra-gcslister-gcslisterworker",
	ProjectID:    "infrastructure-260106",
	Interval:     1 * time.Minute,
	EarlyRenewal: 2 * time.Minute,
	VaultConf: &VaultConfig{
		Address:  "https://vault-test.cermati.com:9443",
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

func LoadFromFile(configFile string) *ArgsConfig {
	defaultCfgClone, err := copystructure.Copy(defaultConfig)
	if err != nil {
		return &defaultConfig
	}

	cfg, ok := defaultCfgClone.(ArgsConfig)
	if !ok {
		return &defaultConfig
	}

	configBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return &defaultConfig
	}

	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		return &defaultConfig
	}

	return &cfg
}
