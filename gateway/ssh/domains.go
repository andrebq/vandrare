package ssh

import (
	"fmt"
	"net"
)

// WrapIP updates entries and rewrites any IPv4 pair IP:Port
// as [IP]:Port, otherwise returns the original value.
func WrapIP(entries []string) {
	for i, v := range entries {
		entries[i] = wrapIP(v)
	}
}

func wrapIP(v string) string {
	host, port, err := net.SplitHostPort(v)
	if err != nil {
		return v
	}
	return fmt.Sprintf("[%v]:%v", host, port)
}
