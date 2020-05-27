package config

import (
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	bnet "github.com/bio-routing/bio-rd/net"
)

// RISConfig is the config of RIS instance
type RISConfig struct {
	BMPServers []BMPServer `yaml:"bmp_servers"`
	BGPConfig  BGPConfig   `yaml:"bgp"`
}

// BMPServer represent a BMP enable Router
type BMPServer struct {
	Address string `yaml:"address"`
	Port    uint16 `yaml:"port"`
}

// BGPConfig represents this local bgp router and its neighbors
type BGPConfig struct {
	LocalASN       uint32 `yaml:"local_asn"`
	RouterID       string `yaml:"router_id"`
	RouterIDUint32 uint32
	Neighbors      []*BGPNeighbor `yaml:"neighbors"`
}

// BGPNeighbor represents the config for a neighbor... speaking BGP!
type BGPNeighbor struct {
	LocalAddress   string `yaml:"local_address"`
	LocalAddressIP *bnet.IP
	PeerAddress    string `yaml:"peer_address"`
	PeerAddressIP  *bnet.IP
	PeerAS         uint32 `yaml:"peer_as"`
	MultiHop       bool   `yaml:"multi_hop"`
	Description    string `yaml:"description"`
}

// LoadConfig loads a RIS config
func LoadConfig(filepath string) (*RISConfig, error) {
	f, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read config file")
	}

	cfg := &RISConfig{}
	err = yaml.Unmarshal(f, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal failed")
	}

	err = cfg.load()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

//LoadConfig

func (c *RISConfig) load() error {
	if c.BGPConfig.LocalASN != 0 {
		err := c.BGPConfig.load()
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *BGPConfig) load() error {
	addr, err := bnet.IPFromString(c.RouterID)
	if err != nil {
		return errors.Wrap(err, "Unable to parse router id")
	}
	c.RouterIDUint32 = uint32(addr.Lower())

	for _, neighbor := range c.Neighbors {
		fmt.Printf("Loading neighbor %v\n", neighbor)
		err := neighbor.load()
		if err != nil {
			return err
		}
	}

	return nil
}

func (neighbor *BGPNeighbor) load() error {
	addr, err := bnet.IPFromString(neighbor.PeerAddress)
	if err != nil {
		return errors.Wrapf(err, "Unable to parse neighbor peer address %s", neighbor.PeerAddress)
	}
	neighbor.PeerAddressIP = addr.Dedup()

	addr, err = bnet.IPFromString(neighbor.LocalAddress)
	if err != nil {
		return errors.Wrapf(err, "Unable to parse neighbor local address %s", neighbor.LocalAddress)
	}
	neighbor.LocalAddressIP = addr.Dedup()

	return nil
}
