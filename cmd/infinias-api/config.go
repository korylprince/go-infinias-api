package main

type Config struct {
	API struct {
		Prefix   string `yaml:"prefix"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"api"`
	DB struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Instance string `yaml:"instance"`
		Database string `yaml:"database"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"db"`
	HTTP struct {
		ListenAddr string `yaml:"listen_addr"`
		APIKey     string `yaml:"api_key"`
	} `yaml:"http"`
}
