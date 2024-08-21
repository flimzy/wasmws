package wasmws

import "net/http"

// DialOption configures how we set up the connection.
type DialOption interface {
	apply(*dialOptions)
}

type dialOptions struct {
	client *http.Client
}

type httpClientOption struct {
	client *http.Client
}

func (o *httpClientOption) apply(do *dialOptions) {
	do.client = o.client
}

// WithHTTPClient returns a DialOption that sets the HTTP client for the
// connection. Has no effect on the browser-based implementation.
func WithHTTPClient(client *http.Client) DialOption {
	return &httpClientOption{client: client}
}
