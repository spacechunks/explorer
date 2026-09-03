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

package publish

import (
	"io"
	"time"
)

type progressReader struct {
	size     float64
	uploaded float64
	inner    io.Reader
	t        *time.Ticker
}

func (p *progressReader) Read(b []byte) (n int, err error) {
	p.uploaded += float64(len(b))
	if p.uploaded >= p.size {
		p.t.Stop()
	}
	return p.inner.Read(b)
}

func (p *progressReader) OnProgress(f func(uint)) {
	p.t = time.NewTicker(100 * time.Millisecond)
	go func() {
		for range p.t.C {
			progress := uint((p.uploaded / p.size) * 100)
			f(progress)
		}
	}()
}

func (p *progressReader) StopReporting() {
	if p.t != nil {
		p.t.Stop()
	}
}
