package config

import (
	"fmt"
	"net"
	"strings"

	"github.com/readium/go-toolkit/cmd/server/internal/consts"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds all configuration for our program
type Config struct {
	EnvName         string
	BindAddr        net.IP
	BindPort        uint
	SentryDSN       string
	CacheDSN        string
	PublicationPath string
	StaticPath      string
	Origins         []string

	LogFile   string
	LogFormat string
	LogLevel  string
}

// NewConfig creates a Config instance
func NewConfig() Config {
	cnf := Config{
		EnvName:         "local",
		BindAddr:        net.ParseIP("127.0.0.1"),
		BindPort:        15080,
		SentryDSN:       "",
		CacheDSN:        "",
		PublicationPath: "./publications",
		StaticPath:      "./public",
		Origins:         []string{},

		LogFile:   "stdout",
		LogFormat: "text",
		LogLevel:  "info",
	}
	return cnf
}

// InDev determines whether the server is being run in a dev env
func (cnf *Config) InDev() bool {
	return cnf.EnvName == "local"
}

// InProfile determines whether the server is being run in a env that should be profiled
func (cnf *Config) InProfile() bool {
	return cnf.EnvName == "profile" || cnf.EnvName == "local"
}

// addFlags adds all the flags from the command line
// TODO upload and storage path
func (cnf *Config) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&cnf.EnvName, "env-name", cnf.EnvName, "The environment of the application. "+
		"Used to load the right config file.")
	fs.IPVar(&cnf.BindAddr, "bind-address", cnf.BindAddr, "The IP address to listen at.")
	fs.UintVar(&cnf.BindPort, "bind-port", cnf.BindPort, "The port to listen at.")
	fs.StringVar(&cnf.SentryDSN, "sentry-dsn", cnf.SentryDSN, "Sentry DSN.")
	fs.StringVar(&cnf.CacheDSN, "cache-dsn", cnf.CacheDSN, "Cache DSN.")
	fs.StringVar(&cnf.PublicationPath, "publication-path", cnf.PublicationPath, "Publication storage path.")
	fs.StringVar(&cnf.StaticPath, "static-path", cnf.StaticPath, "Static assets path.")
	fs.StringArrayVar(&cnf.Origins, "cors-origins", cnf.Origins, "List of origins to allow for CORS.")

	fs.StringVar(&cnf.LogFile, "log-file", cnf.LogFile, "The log file to write to. "+
		"'stdout' means log to stdout, 'stderr' means log to stderr and 'null' means discard log messages.")
	fs.StringVar(&cnf.LogFormat, "log-format", cnf.LogFormat,
		"The log format. Valid format values are: text, json.")
	fs.StringVar(&cnf.LogLevel, "log-level", cnf.LogLevel, "The granularity of log outputs. "+
		"Valid log levels: debug, info, warning, error and critical.")
}

// wordSepNormalizeFunc changes all flags that contain "_" separators
func wordSepNormalizeFunc(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if strings.Contains(name, "_") {
		return pflag.NormalizedName(strings.Replace(name, "_", "-", -1))
	}
	return pflag.NormalizedName(name)
}

// BindFlags normalizes and parses the command line flags
func (cnf *Config) BindFlags() {
	cnf.addFlags(pflag.CommandLine)
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		m := fmt.Sprintf("Error binding flags: %v", err)
		logrus.Panic(m)
		panic(m)
	}

	pflag.CommandLine.SetNormalizeFunc(wordSepNormalizeFunc)
	pflag.Parse()

	viper.SetEnvPrefix(consts.AppAcronym)
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	configName := fmt.Sprintf("config.%s", strings.ToLower(viper.GetString("env-name")))
	viper.SetConfigName(configName)
	viper.SetConfigType("toml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("./cmd/server/configs")
	viper.AddConfigPath("/configs")
	viper.AddConfigPath("/etc/" + consts.AppAcronym)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logrus.Errorf("Config file not found: %v", err)
		} else {
			logrus.Panicf("Couldn't load config file: %v", err)
		}
	}

	viper.Set("dev", cnf.InDev())
	viper.Set("profile", cnf.InProfile())
}
