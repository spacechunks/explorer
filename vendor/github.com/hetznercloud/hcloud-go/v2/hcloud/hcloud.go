/*
Package hcloud is a library for the Hetzner Cloud API.

The Hetzner Cloud API reference is available at https://docs.hetzner.cloud.

# Retry mechanism

The [Client.Do] method will retry failed requests that match certain criteria. The
default retry interval is defined by an exponential backoff algorithm truncated to 60s
with jitter. The default maximal number of retries is 5.

The following rules defines when a request can be retried:

When the [http.Client] returned a network timeout error.

When the API returned an HTTP error, with the status code:
  - [http.StatusBadGateway]
  - [http.StatusGatewayTimeout]

When the API returned an application error, with the code:
  - [ErrorCodeConflict]
  - [ErrorCodeRateLimitExceeded]

Changes to the retry policy might occur between releases, and will not be considered
breaking changes.
*/
package hcloud

// Version is the library's version following Semantic Versioning.
const Version = "2.16.0" // x-release-please-version
