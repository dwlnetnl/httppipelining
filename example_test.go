package httppipelining_test

import (
	"fmt"

	"github.com/dwlnetnl/httppipelining"
)

func Example_Available() {
	websites := []string{
		"http://www.example.com",
		"https://www.cloudflare.com",
		"http://httpd.apache.org",
		"https://www.nginx.org",
		"https://www.haproxy.org",
	}

	for _, website := range websites {
		available, err := httppipelining.Available(website)
		if err != nil {
			fmt.Println("HTTP pipelining check failed:", err)
			continue
		}

		if available {
			fmt.Println(website, "supports HTTP pipelining")
		} else {
			fmt.Println(website, "does not support HTTP pipelining")
		}
	}

	// Output:
	// http://www.example.com supports HTTP pipelining
	// https://www.cloudflare.com does not support HTTP pipelining
	// http://httpd.apache.org does not support HTTP pipelining
	// https://www.nginx.org does not support HTTP pipelining
	// https://www.haproxy.org supports HTTP pipelining
}
