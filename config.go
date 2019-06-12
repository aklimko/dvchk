package main

import (
	"github.com/spf13/viper"
)

type Config struct {
	InsecureTls bool
}

func ReadConfig() Config {
	v := viper.New()

	setupDefaults(v)
	setupEnvVars(v)

	var cfg Config
	err := v.Unmarshal(&cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

func setupDefaults(v *viper.Viper) {
	v.SetDefault("insecureTls", false)
}

func setupEnvVars(v *viper.Viper) {
	v.AutomaticEnv()
}
