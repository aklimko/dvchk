package main

import (
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	All      bool
	Insecure bool
	Timeout  int
	Verbose  bool
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
	pflag.BoolP("all", "a", false, "Print all newer versions")
	pflag.BoolP("insecure", "k", false, "Disable TLS certificates validation")
	pflag.IntP("timeout", "t", 5, "Set timeout for http requests in seconds")
	pflag.BoolP("verbose", "v", false, "Include additional logs")

	pflag.Parse()

	err := v.BindPFlags(pflag.CommandLine)
	if err != nil {
		panic(err)
	}
}
