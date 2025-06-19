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

package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

var paperServerReadyRegex = regexp.MustCompile(`Done \(\d+\.\d+s\)! For help, type "help"`)

type logReader struct {
	exec remotecommand.Executor

	cancel context.CancelFunc
	regex  *regexp.Regexp
}

func newLogReader(exec remotecommand.Executor) *logReader {
	return &logReader{
		exec: exec,
	}
}

func (r *logReader) WaitForRegex(ctx context.Context, regex *regexp.Regexp) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.cancel = cancel
	r.regex = regex

	if err := r.exec.StreamWithContext(cancelCtx, remotecommand.StreamOptions{
		Stdout: r,
	}); err != nil {
		if !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return nil
}

func (r *logReader) Write(p []byte) (n int, err error) {
	if r.regex.Match(p) {
		r.cancel()
	}
	return len(p), nil
}

func spdyExecutor(urlStr string) (remotecommand.Executor, error) {
	streamURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	restCfg := &rest.Config{TLSClientConfig: rest.TLSClientConfig{}}

	exec, err := remotecommand.NewSPDYExecutor(restCfg, http.MethodPost, streamURL)
	if err != nil {
		return nil, fmt.Errorf("create executor: %w", err)
	}

	return exec, nil
}
