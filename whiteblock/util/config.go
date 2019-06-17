package util

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Config groups all of the global configuration parameters into
// a single struct
type Config struct {
	APIURL      string `mapstructure:"apiURL"`
	Verbosity   string `mapstructure:"verbosity"`
	HTTPTimeout int64  `mapstructure:"httpTimeout"`
	HTTPRetries int    `mapstructure:"httpRetries"`
}

var conf = new(Config)

func setViperEnvBindings() {
	viper.BindEnv("apiURL", "API_URL")
	viper.BindEnv("verbosity", "VERBOSITY")
	viper.BindEnv("httpTimeout", "HTTP_TIMEOUT")
	viper.BindEnv("httpRetries", "HTTP_RETRIES")
}
func setViperDefaults() {
	viper.SetDefault("apiURL", "https://api.whiteblock.io")
	viper.SetDefault("verbosity", "ERROR")
	viper.SetDefault("httpTimeout", 10000)
	viper.SetDefault("httpRetries", 5)
}

func init() {
	setViperDefaults()
	setViperEnvBindings()
	viper.AddConfigPath("/etc/whiteblock/")          // path to look for the config file in
	viper.AddConfigPath("$HOME/.config/whiteblock/") // call multiple times to add many search paths
	viper.SetConfigName("whiteblock")
	viper.SetConfigType("yaml")
	err := viper.ReadInConfig()

	if err != nil {
		log.WithFields(log.Fields{"error": err}).Debug("could not find the config file")
	}
	err = viper.Unmarshal(&conf)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	lvl, err := log.ParseLevel(conf.Verbosity)
	if err != nil {
		log.SetLevel(log.InfoLevel)
		log.Warn(err)
	}
	log.SetLevel(lvl)
}

// GetConfig gets a pointer to the global config object.
// Do not modify conf object
func GetConfig() *Config {
	return conf
}
