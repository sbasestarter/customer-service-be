package config

import (
	"sync"

	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libconfig"
	"github.com/sgostarter/liblogrus"
	"github.com/sgostarter/libservicetoolset/clienttoolset"
)

type WSConfig struct {
	Logger l.Wrapper `yaml:"-"`

	CustomerListen               string                          `yaml:"CustomerListen"`
	CustomerGRPCClientConfig     *clienttoolset.GRPCClientConfig `yaml:"CustomerGRPCClientConfig"`
	CustomerUserGRPCClientConfig *clienttoolset.GRPCClientConfig `yaml:"CustomerUserGRPCClientConfig"`

	ServicerListen               string                          `yaml:"ServicerListen"`
	ServicerGRPCClientConfig     *clienttoolset.GRPCClientConfig `yaml:"ServicerGRPCClientConfig"`
	ServicerUserGRPCClientConfig *clienttoolset.GRPCClientConfig `yaml:"ServicerUserGRPCClientConfig"`
}

var (
	_wsCfg  WSConfig
	_wsOnce sync.Once
)

func GetWSConfig() *WSConfig {
	_wsOnce.Do(func() {
		_wsCfg.Logger = l.NewWrapper(liblogrus.NewLogrus())
		_wsCfg.Logger.GetLogger().SetLevel(l.LevelDebug)

		_, err := libconfig.Load("ws_config.yaml", &_wsCfg)
		if err != nil {
			panic("load config: " + err.Error())
		}
	})

	return &_wsCfg
}
