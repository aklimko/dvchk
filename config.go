package main

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	InsecureTls bool
}

func ReadConfig() Config {
	v := viper.New()

	setupEnvVars(v)
	setupFlags(v)

	var cfg Config
	err := v.Unmarshal(&cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

func setupEnvVars(v *viper.Viper) {
	v.AutomaticEnv()
}

func setupFlags(v *viper.Viper) {
	pflag.BoolP("insecureTls", "k", false, "Disables TLS certificates validation")

	pflag.Parse()

	err := v.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(err)
	}
}
