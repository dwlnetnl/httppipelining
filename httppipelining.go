// Package httppipelining checks if HTTP/1.1 pipelining
// can be used for a particular HTTP server.
package httppipelining

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
)

// Available checks if HTTP pipelining is available.
func Available(rawurl string) (bool, error) {
	conn, host, err := Dial(rawurl)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	return Supported(conn, host)
}

// Dial dials a HTTP server and returns the connection.
// The host name is returned for use in the Host header.
func Dial(rawurl string) (conn net.Conn, host string, err error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return nil, "", err
	}

	host = url.Hostname()
	port := url.Port()
	switch url.Scheme {
	case "http":
		if port == "" {
			port = "80"
		}
		addr := net.JoinHostPort(host, port)
		conn, err = net.Dial("tcp", addr)
	case "https":
		if port == "" {
			port = "443"
		}
		addr := net.JoinHostPort(host, port)
		conn, err = tls.Dial("tcp", addr, nil)
	default:
		err = fmt.Errorf("unsupported scheme: %s", url.Scheme)
	}

	return conn, host, err
}

// Supported checks if connection rw supports HTTP pipelining.
// Host is required and used in the Host header.
func Supported(rw io.ReadWriter, host string) (bool, error) {
	if host == "" {
		panic("host is empty")
	}
	return Probe(rw, &optionsProber{host})
}

// Prober probes a connection for HTTP pipelining support.
type Prober interface {
	// NumRequests return the number of requests used in probe.
	NumRequests() uint

	// WriteRequest writes a probe request.
	WriteRequest(id uint, w *bufio.Writer) error

	// ReadRequest reads a probe request and checks if
	// it is the expected answer for the corresponding
	// request. This asserts the pipeline ordering.
	ReadRequest(id uint, r *bufio.Reader) (expected bool, err error)
}

// Probe probes connection rw for HTTP pipelining support.
func Probe(rw io.ReadWriter, p Prober) (available bool, err error) {
	type write struct {
		id  uint
		err error
	}

	n := p.NumRequests()
	writes := make(chan write, n)
	flush := make(chan error)
	go func() {
		bw := bufio.NewWriter(rw)
		for id := uint(0); id < n; id++ {
			err := p.WriteRequest(id, bw)
			writes <- write{id, err}
		}
		flush <- bw.Flush()
		close(writes)
	}()

	if err := <-flush; err != nil {
		return false, err
	}

	available = true
	br := bufio.NewReader(rw)
	for w := range writes {
		if w.err != nil {
			return false, w.err
		}
		expected, err := p.ReadRequest(w.id, br)
		if err != nil {
			return false, err
		}
		available = available && expected
	}

	return available, nil
}

type optionsProber struct {
	host string
}

var _ Prober = (*optionsProber)(nil)

func (p *optionsProber) NumRequests() uint { return 2 }
func (p *optionsProber) WriteRequest(id uint, w *bufio.Writer) (err error) {
	switch id {
	case 0:
		// expect 200 OK
		_, err = fmt.Fprintf(w, "OPTIONS * HTTP/1.1\r\nHost: %s\r\n\r\n", p.host)
	case 1:
		// expect 400 Bad Request
		_, err = fmt.Fprintf(w, "OPTIONS . HTTP/1.1\r\nHost: %s\r\n\r\n", p.host)
	default:
		panic(fmt.Sprintf("invalid id: %d", id))
	}
	return err
}

func (p *optionsProber) ReadRequest(id uint, r *bufio.Reader) (expected bool, err error) {
	if id >= 2 {
		panic(fmt.Sprintf("invalid id: %d", id))
	}
	code, err := parseStatus(r)
	if err == io.EOF {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("malformed request: %w (id=%d)", err, id)
	}
	switch id {
	case 0:
		expected = (code == 200)
	case 1:
		expected = (code == 400)
	}
	return expected, nil
}

func parseStatus(r *bufio.Reader) (status int, err error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return 0, err
	}
	n, err := fmt.Sscanf(line, "HTTP/1.1 %d", &status)
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, errors.New("malformed status line")
	}

	var contentLength int
	lengthFound := false
	for {
		line, err := r.ReadSlice('\n')
		if err != nil {
			return 0, err
		}

		if len(line) == 2 && line[0] == '\r' && line[1] == '\n' {
			break
		}
		if lengthFound || line[0] != 'C' && line[0] != 'c' {
			continue
		}

		lower := bytes.ToLower(line)
		if bytes.HasPrefix(lower, []byte("content-length:")) {
			value := string(lower[len("content-length:"):])
			n, err := fmt.Sscanf(value, "%d\r\n", &contentLength)
			if err != nil {
				return 0, err
			}
			if n != 1 {
				return 0, fmt.Errorf("no content length")
			}
			lengthFound = true
		}
	}
	if _, err := r.Discard(contentLength); err != nil {
		return 0, err
	}

	return status, nil
}
