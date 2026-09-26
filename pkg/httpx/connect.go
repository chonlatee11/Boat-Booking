package httpx

import (
	"fmt"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
)

// ConnectOtel returns a connect.Option that wires OTel tracing/metrics (D-50)
// into a connect-go handler or client via otelconnect's interceptor — the
// same option works on either side of a connect call.
func ConnectOtel() (connect.Option, error) {
	interceptor, err := otelconnect.NewInterceptor()
	if err != nil {
		return nil, fmt.Errorf("httpx: new otelconnect interceptor: %w", err)
	}
	return connect.WithInterceptors(interceptor), nil
}
