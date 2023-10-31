package config

import (
	"fmt"
	"log"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Server is the ezvpn server configuration.
type Server struct {
	EnableTLS         bool
	EnablePprof       bool
	EnableInlineSocks bool
	ControlAddress    string
	ConfigFile        string
	CaFile            string
	CertFile          string
	KeyFile           string
	SocksServer       string
}

// AllowedAgents is the configuration for the allowed agents.
type AllowedAgents struct {
	Agents []struct {
		Name        string   `mapstructure:"name"`
		AuthKey     string   `mapstructure:"auth_key"`
		ApprovedCNs []string `mapstructure:"approved_cns"`
	}
}

var AGENTS, AGENTS_TMP AllowedAgents
var SERVER Server

func (server *Server) HotReload() {
	var err error
	s := viper.New()
	s.SetConfigFile(server.ConfigFile)

	if err = s.ReadInConfig(); err != nil {
		log.Panic(err)
	}
	if err = s.Unmarshal(&AGENTS); err != nil {
		log.Panic(err)
	}
	//fmt.Println("1: ", AGENTS)

	s.WatchConfig()

	s.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Config file changed: %s, reload it.", e.Name)
		AGENTS_TMP = AllowedAgents{}
		if err := s.Unmarshal(&AGENTS_TMP); err != nil {
			log.Println(err)
		} else {
			if err := check(&AGENTS_TMP); err != nil {
				log.Println(err)
			} else {
				// here we can reload the config safely
				AGENTS = AGENTS_TMP
				fmt.Println("2: ", AGENTS)
			}
		}
	})
}

// basic check for the config
func check(a *AllowedAgents) error {
	if a == nil {
		return fmt.Errorf("nil pointer")
	}
	for _, agent := range a.Agents {
		if agent.AuthKey == "" {
			return fmt.Errorf("empty auth key")
		}
		if len(agent.ApprovedCNs) == 0 {
			return fmt.Errorf("empty approved CNs")
		}
	}
	return nil
}
