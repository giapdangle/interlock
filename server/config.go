// INTERLOCK | https://github.com/inversepath/interlock
// Copyright (c) 2015 Inverse Path S.r.l.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type config struct {
	// exported
	Debug       bool     `json:"debug"`
	SetTime     bool     `json:"set_time"`
	BindAddress string   `json:"bind_address"`
	TLSCert     string   `json:"tls_cert"`
	TLSKey      string   `json:"tls_key"`
	KeyPath     string   `json:"key_path"`
	VolumeGroup string   `json:"volume_group"`
	Ciphers     []string `json:"ciphers"`

	// internal
	availableCiphers map[string]cipherInterface
	enabledCiphers   map[string]cipherInterface
	mountPoint       string
	testMode         bool
	logFile          *os.File
}

var conf config

func (c *config) SetAvailableCipher(cipher cipherInterface) {
	if c.availableCiphers == nil {
		c.availableCiphers = make(map[string]cipherInterface)
	}

	c.availableCiphers[cipher.GetInfo().Name] = cipher
}

func (c *config) GetCipher(cipherName string) (cipher cipherInterface, err error) {
	cipher, ok := conf.enabledCiphers[cipherName]

	if !ok {
		err = errors.New("invalid cipher")
		return
	}

	// get a fresh instance
	cipher = cipher.New()

	return
}

func (c *config) GetCipherByExt(ext string) (cipher cipherInterface, err error) {
	for _, value := range c.enabledCiphers {
		if value.GetInfo().Extension == ext {
			cipher = value
			return
		}
	}

	err = errors.New("invalid cipher")

	return
}

func (c *config) EnableCiphers() {
	if c.enabledCiphers == nil {
		c.enabledCiphers = make(map[string]cipherInterface)
	}

	if len(c.Ciphers) == 0 {
		c.PrintAvailableCiphers()
		log.Fatalf("missing cipher specification")
	}

	for i := 0; i < len(c.Ciphers); i++ {
		if val, ok := c.availableCiphers[c.Ciphers[i]]; ok {
			c.enabledCiphers[c.Ciphers[i]] = val
		} else {
			c.PrintAvailableCiphers()
			log.Fatalf("unsupported cipher name %s", c.Ciphers[i])
		}
	}
}

func (c *config) PrintAvailableCiphers() {
	log.Println("supported ciphers:")

	for k := range c.availableCiphers {
		log.Printf("\t%s", k)
	}
}

func (c *config) SetDefaults() {
	c.Debug = false
	c.SetTime = false
	c.TLSCert = "certs/cert.pem"
	c.TLSKey = "certs/key.pem"
	c.KeyPath = "keys"
	c.Ciphers = []string{"OpenPGP", "AES-256-OFB"}
	c.testMode = false
	c.VolumeGroup = "lvmvolume"
}

func (c *config) SetMountPoint() error {
	c.mountPoint = filepath.Join(os.Getenv("HOME"), ".interlock-mnt")

	return os.MkdirAll(c.mountPoint, 0700)
}

func (c *config) Set(configPath string) (err error) {
	f, err := os.Open(configPath)

	if err != nil {
		return
	}

	b, err := ioutil.ReadAll(f)

	if err != nil {
		return
	}

	err = json.Unmarshal(b, &c)

	return
}

func (c *config) Print() {
	j, _ := json.MarshalIndent(c, "", "\t")

	log.Println("applied configuration:")
	log.Printf("\n%s", string(j))
}

func setTime(w http.ResponseWriter, r *http.Request) (res jsonObject) {
	req, err := parseRequest(r)

	if err != nil {
		return errorResponse(err, "")
	}

	var epoch int64

	err = validateRequest(req, []string{"epoch:n"})

	if err != nil {
		return errorResponse(err, "")
	}

	switch t := req["epoch"].(type) {
	case json.Number:
		epoch, err = t.Int64()
	default:
		return errorResponse(errors.New("invalid epoch format"), "")
	}

	args := []string{"-s", "@" + strconv.FormatInt(epoch, 10)}
	cmd := "/bin/date"

	if conf.SetTime {
		_, err = execCommand(cmd, args, true, "")

		if err != nil {
			return errorResponse(err, "")
		}

		hour, min, sec := time.Now().Clock()

		status.Log(syslog.LOG_NOTICE, "adjusted device time to %02d:%02d:%02d", hour, min, sec)
	}

	res = jsonObject{
		"status":   "OK",
		"response": nil,
	}

	return
}
