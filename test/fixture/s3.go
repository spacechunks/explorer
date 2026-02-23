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

package fixture

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/require"
)

const (
	Bucket = "explorer"
)

type FakeS3 struct {
	Endpoint string
	server   *http.Server
}

func RunFakeS3(t *testing.T) FakeS3 {
	s := &http.Server{
		Addr:    ":3080",
		Handler: gofakes3.New(s3mem.New(), gofakes3.WithAutoBucket(true)).Server(),
	}

	f := FakeS3{
		Endpoint: "localhost:3080",
		server:   s,
	}

	require.NoError(t, os.Setenv("AWS_ENDPOINT_URL", "http://localhost:3080"))
	require.NoError(t, os.Setenv("AWS_REGION", "us-east-1"))

	go s.ListenAndServe()

	t.Cleanup(func() {
		s.Shutdown(context.Background())
	})

	return f
}

func NewS3Client(t *testing.T, ctx context.Context) *s3.Client {
	s3cfg, err := awscfg.LoadDefaultConfig(
		ctx,
		// we have to set anything otherwise it doesn't work
		awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("key", "secret", ""),
		),
	)
	require.NoError(t, err)

	return s3.NewFromConfig(s3cfg)
}

func (f FakeS3) UploadObject(t *testing.T, key string, data []byte) {
	var (
		ctx = context.Background()
		c   = NewS3Client(t, ctx)
	)

	_, err := c.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	require.NoError(t, err)
}

func (f FakeS3) RequireObjectExists(t *testing.T, key string) {
	var (
		ctx = context.Background()
		c   = NewS3Client(t, ctx)
	)

	_, err := c.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var s3err smithy.APIError
		if errors.As(err, &s3err) && s3err.ErrorCode() == "NotFound" {
			t.Fatalf("object %s does not exist", key)
			return
		}
		t.Fatalf("head object failed: %v", err)
	}
}
