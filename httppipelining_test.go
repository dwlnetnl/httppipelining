package httppipelining

import (
	"bufio"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAvailable(t *testing.T) {
	s := httptest.NewServer(nil)
	t.Cleanup(s.Close)

	available, err := Available(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		t.Error("pipelining not available")
	}
}

func Test_parseStatus(t *testing.T) {
	cases := []struct {
		code int
		resp string
	}{
		{200, "HTTP/1.1 200 OK\r\nServer: Apache\r\nContent-Length: 0\r\n\r\n"},
		{400, "HTTP/1.1 400 Bad Request\r\nServer: Apache\r\nContent-Length: 4\r\n\r\nbody"},
	}
	for i, c := range cases {
		r := bufio.NewReader(strings.NewReader(c.resp))
		code, err := parseStatus(r)
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}
		if code != c.code {
			t.Errorf("%d: got %d, want: %d", i, code, c.code)
		}
	}
}
