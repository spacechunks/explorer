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

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spacechunks/explorer/cli/fshelper"
)

var DefaultConfig = Config{
	ControlPlaneEndpoint: "explorer.api.chunks.space:443",
	IDPIssuerEndpoint:    "https://iam.chunks.space",
	IDPClientID:          "385828015612952672",
	IDPScopes: []string{
		oidc.ScopeOpenID,
		"profile",
		"email",
		"urn:zitadel:iam:org:id:385828012106448992",
	},
}

type Config struct {
	ControlPlaneEndpoint string   `json:"controlPlaneEndpoint"`
	IDPIssuerEndpoint    string   `json:"idpIssuerEndpoint"`
	IDPClientID          string   `json:"idpClientId"`
	IDPScopes            []string `json:"idpScopes"`
}

type ProfileData struct {
	AccessToken string `json:"accessToken"`
}

type Data struct {
	ActiveProfile string                 `json:"activeProfile"`
	Profiles      map[string]ProfileData `json:"profiles"`
}

func New() (Data, error) {
	cfgHome, err := fshelper.ConfigHome()
	if err != nil {
		return Data{}, err
	}

	data, err := os.ReadFile(filepath.Join(cfgHome, "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return Data{
				ActiveProfile: "default",
				Profiles:      map[string]ProfileData{},
			}, nil
		}
		return Data{}, err
	}

	var state Data
	if err := json.Unmarshal(data, &state); err != nil {
		return Data{}, err
	}

	// as introduced profiles at a later point, there could be
	// configs out in the wild without anything set so we need to
	// take care to establish default values
	if state.ActiveProfile == "" {
		state.ActiveProfile = "default"
	}

	if state.Profiles == nil {
		state.Profiles = make(map[string]ProfileData)
	}

	return state, nil
}

func (d *Data) SetActiveProfile(profile string) {
	if profile == "" {
		d.ActiveProfile = "default"
		return
	}

	d.ActiveProfile = profile

	if err := d.persist(); err != nil {
		fmt.Println("Failed to persist state data", err)
	}
}

func (d *Data) ActiveProfileAccessToken() string {
	return d.Profiles[d.ActiveProfile].AccessToken
}

func (d *Data) UpdateActiveProfile(profile string) {
	if profile != "" {
		d.ActiveProfile = profile
	}

	// only log it, because we can still work with it in memory.
	if err := d.persist(); err != nil {
		fmt.Println("Failed to persist state data", err)
	}
}

func (d *Data) UpdateProfileData(profile string, new ProfileData) {
	defer func() {
		if err := d.persist(); err != nil {
			fmt.Println("Failed to persist state data", err)
		}
	}()

	curr, ok := d.Profiles[profile]
	if !ok {
		d.Profiles[profile] = new
		return
	}

	if curr.AccessToken != "" {
		curr.AccessToken = new.AccessToken
	}

	d.Profiles[profile] = curr
}

func (d *Data) persist() error {
	cfgHome, err := fshelper.ConfigHome()
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(cfgHome, "state.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(d)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}

	return nil
}
