package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"net"
)

type SSH struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Port     int    `json:"port"`
	Type     string `json:"type"`
	Password string `json:"password"`
	KeyFile  string `json:"key"`
}

func (s *SSH) GetSSH(key string) (*ssh.Client, error) {
	address := fmt.Sprintf("%s:%d", s.Host, s.Port)
	config := &ssh.ClientConfig{
		User: s.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	if key == "KEY" {
		if k, err := ioutil.ReadFile(s.KeyFile); err != nil {
			return nil, err
		} else {
			signer, err := ssh.ParsePrivateKey(k)
			if err != nil {
				return nil, err
			}
			config.Auth = []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			}
		}
	} else {
		config.Auth = []ssh.AuthMethod{
			ssh.Password(s.Password),
		}
	}

	return ssh.Dial("tcp", address, config)
}

type Dialer struct {
	client *ssh.Client
}

func (v *Dialer) Dial(address string) (net.Conn, error) {
	return v.client.Dial("tcp", address)
}
