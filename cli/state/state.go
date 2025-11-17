/*
 Explorer Platform, a platform for hosting and discovering Minecraft servers.
 Copyright (C) 2024 Yannic Rieger <oss@76k.io>

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU Affero General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 GNU Affero General Public License for more details.

 You should have received a copy of the GNU Affero General Public License
 along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spacechunks/explorer/cli/fshelper"
)

var DefaultConfig = Config{
	ControlPlaneEndpoint: "api.explorer.stag.chunks.cloud:443",
	IDPIssuerEndpoint:    "https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0",
	IDPClientID:          "c740e883-16dd-4c0c-a50b-b19de508b70a",
}

type Config struct {
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint"`
	IDPIssuerEndpoint    string `json:"idpIssuerEndpoint"`
	IDPClientID          string `json:"idpClientId"`
}

type Data struct {
	IDToken              string `json:"idToken"`
	ControlPlaneAPIToken string `json:"controlPlaneApiToken"`
}

func New() (Data, error) {
	cfgHome, err := fshelper.ConfigHome()
	if err != nil {
		return Data{}, err
	}

	data, err := os.ReadFile(filepath.Join(cfgHome, "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return Data{}, nil
		}
		return Data{}, err
	}

	var state Data
	if err := json.Unmarshal(data, &state); err != nil {
		return Data{}, err
	}

	return state, nil
}

func (d *Data) Update(new Data) {
	if new.ControlPlaneAPIToken != "" {
		d.ControlPlaneAPIToken = new.ControlPlaneAPIToken
	}

	if new.IDToken != "" {
		d.IDToken = new.IDToken
	}

	// only log it, because we can still work with it in memory.
	if err := d.persist(); err != nil {
		fmt.Println("Failed to persist state data", err)
	}
}

func (s *Data) persist() error {
	cfgHome, err := fshelper.ConfigHome()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(cfgHome, "state.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}
