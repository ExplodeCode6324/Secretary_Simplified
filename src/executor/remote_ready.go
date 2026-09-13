package executor

import (
	"context"
	"net"
	"time"
)

// Ready is a read-only availability probe before claiming P1 work. A later
// disconnect remains RESULT_UNKNOWN and is handled by authoritative reconciliation.
func (c RemoteCore) Ready(ctx context.Context) bool {
	probe, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	conn, e := (&net.Dialer{}).DialContext(probe, "unix", c.Client.Socket)
	if e != nil {
		return false
	}
	conn.Close()
	return true
}
