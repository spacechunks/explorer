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

package main

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

func main() {
	fs := flag.NewFlagSet("tokengen", flag.ExitOnError)
	var (
		issuer     = fs.String("issuer", "", "Issuer for the token")
		expires    = fs.Duration("expires", 24*time.Hour, "Expiration date of the token")
		userID     = fs.String("user-id", "", "User ID")
		email      = fs.String("email", "", "Email")
		signingKey = fs.String("signing-key", "", "Signing key")
	)

	if err := fs.Parse(os.Args[1:]); err != nil {
		die("failed to parse flags", err)
	}

	iss := time.Now()
	apiTok, err := jwt.NewBuilder().
		IssuedAt(iss).
		Issuer(*issuer).
		Audience([]string{*issuer}).
		Expiration(iss.Add(*expires)).
		Claim("user_id", userID).
		Claim("email", email).
		Build()
	if err != nil {
		die("create token", err)
	}

	data, err := os.ReadFile(*signingKey)
	if err != nil {
		die("read signing key", err)
	}

	pemBlock, _ := pem.Decode(data)

	key, err := x509.ParseECPrivateKey(pemBlock.Bytes)
	if err != nil {
		die("parse ec private key", err)
	}

	signed, err := jwt.Sign(apiTok, jwt.WithKey(jwa.ES256(), key))
	if err != nil {
		die("sign token", err)
	}

	fmt.Println(string(signed))
}

func die(msg string, err error) {
	fmt.Println(msg, err)
	os.Exit(1)
}
