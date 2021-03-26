package conf

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type serverConf struct {
	Server nodeConf `yaml:"server"`
	Log    logConf  `yaml:"log"`
}

type nodeConf struct {
	SelfAddr string `yaml:"self_addr"`
	PeerAddr string `yaml:"peer_addr"`
}

type logConf struct {
	Level string `yaml:"level"`
	Dir   string `yaml:"dir"`
}

var Conf serverConf

func InitConf(path string) error {
	fmt.Printf("path: %s\n", path)
	d, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(d, &Conf)
	if err != nil {
		panic(err)
	}
	fmt.Printf("conf: %+v\n", Conf)
	return nil
}
