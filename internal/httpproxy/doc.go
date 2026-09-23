// Package httpproxy implements a minimal HTTP/1.1 CONNECT proxy for local use,
// with target dialing through an SSH tunnel. It mirrors the socks package: the
// client connects over plain TCP and its traffic is tunneled to the target on
// the remote side of the SSH session.
package httpproxy
