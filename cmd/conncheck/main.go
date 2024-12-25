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
	"bufio"
	"log"
	"net"
	"net/http"
	"time"
)

func main() {
	t := time.NewTicker(1000 * time.Millisecond)

	go func() {
		listener, err := net.Listen("tcp", "0.0.0.0:25565")
		if err != nil {
			log.Fatalf("listen: %v\n", err)
		}
		defer listener.Close()

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("accept: %v\n", err)
				continue
			}
			go handleConnection(conn)
		}
	}()

	for range t.C {
		httpsResp, err := http.Get("https://www.google.com")
		if err != nil {
			log.Printf("failed to fetch https: %v\n", err)
			continue
		}
		httpResp, err := http.Get("http://www.google.com")
		if err != nil {
			log.Printf("failed to fetch http: %v\n", err)
			continue
		}
		defer httpsResp.Body.Close() //nolint:staticcheck
		defer httpResp.Body.Close()  //nolint:staticcheck
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("client connected: %s\n", conn.RemoteAddr())
	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("client disconnected: %s\n", conn.RemoteAddr())
			return
		}

		if _, err = conn.Write([]byte(message)); err != nil {
			log.Printf("error writing to client: %v\n", err)
			return
		}
	}
}
