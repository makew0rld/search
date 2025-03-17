package config

import "github.com/BurntSushi/toml"

type ConfigStruct struct {
	PandocPath      string `toml:"pandoc_path"`
	PdfToTextPath   string `toml:"pdftotext_path"`
	RecrawlInterval int    `toml:"recrawl_interval"`
	UserAgent       string `toml:"user_agent"`
	Address         string `toml:"address"`
}

var Config ConfigStruct

func Init() error {
	_, err := toml.DecodeFile("config.toml", &Config)
	return err
}
