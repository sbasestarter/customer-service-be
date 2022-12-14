package config

import (
	"sync"

	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libconfig"
	"github.com/sgostarter/liblogrus"
	"github.com/sgostarter/libservicetoolset/servicetoolset"
)

type Config struct {
	Logger l.Wrapper `yaml:"-"`

	Listen        string                            `yaml:"Listen"`
	GRPCTLSConfig *servicetoolset.GRPCTlsFileConfig `yaml:"GRPCTLSConfig"`

	CustomerListen     string `yaml:"CustomerListen"`
	CustomerUserListen string `yaml:"CustomerUserListen"`
	ServicerListen     string `yaml:"ServicerListen"`
	ServicerUserListen string `yaml:"ServicerUserListen"`

	MongoConfig MongoConfig `yaml:"MongoConfig"`
	RabbitMQURL string      `yaml:"RabbitMQURL"`

	UserMongoDSN string `yaml:"UserMongoDSN"`

	CustomerTokenSecret    string `yaml:"CustomerTokenSecret"`
	ServicerTokenSecret    string `yaml:"ServicerTokenSecret"`
	ServicerPasswordSecret string `yaml:"ServicerPasswordSecret"`
}

type MongoConfig struct {
	Server   string `yaml:"Server"`
	DB       string `yaml:"DB"`
	UserName string `yaml:"UserName"`
	Password string `yaml:"Password"`
}

var (
	_cfg  Config
	_once sync.Once
)

func GetConfig() *Config {
	_once.Do(func() {
		_cfg.Logger = l.NewWrapper(liblogrus.NewLogrus())
		_cfg.Logger.GetLogger().SetLevel(l.LevelDebug)

		_, err := libconfig.Load("config.yaml", &_cfg)
		if err != nil {
			panic("load config: " + err.Error())
		}
	})

	return &_cfg
}
