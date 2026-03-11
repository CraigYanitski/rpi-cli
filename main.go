package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
	"time"

	cookiejar "github.com/juju/persistent-cookiejar"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/http2"
)

type SignIn struct {
	AuthToken  string  `json:"authenticity_token"`
	Email      string  `json:"email"`
	Password   string  `json:"password"`
	Commit     string  `json:"commit"`
}

type Verify struct {
	AuthToken  string  `json:"authenticity_token"`
	OTP        string  `json:"otp"`
	Commit     string  `json:"commit"`
}

type apiConfig struct {
	client     *http.Client
	cookiejar  *cookiejar.Jar
	webrtcAPI  *webrtc.API
}

func main() {
	// start program
	fmt.Println("Starting RPI-CLI")

	// open saved cookie jar
	jarOptions := &cookiejar.Options{
		Filename: "cookies.json",
	}
	jar, err := cookiejar.New(jarOptions)
	if err != nil {
		log.Fatal(err)
	}

	// initialise transport for http client
	var DefaultTransport http.RoundTripper = &http2.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2"},
		},
		AllowHTTP: false,
		IdleConnTimeout: 90 * time.Second,
	}

	// initialise client with default transport and cookie jar
	tr := DefaultTransport
	client := &http.Client{
		Jar: jar,
		Transport: &debugTransport{
			transport: tr,
			jar: jar,
		},
	}

	// initialise setting engine for webrtc detached data channel
	s := webrtc.SettingEngine{}
	s.DetachDataChannels()

	// initialise api config
	api := apiConfig{
		client: client,
		cookiejar: jar,
		webrtcAPI: webrtc.NewAPI(webrtc.WithSettingEngine(s)),
	}

	// ---- CONNECT TO SIGNALLING SERVER ----

	// sign into rpi id (required once for cookies)
	//isSignedIn := api.rpiSignIn()

	// connect to rpi device
	api.rpiConnect()

	// save cookies to file
	if err = jar.Save(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nCookies saved to %s\n", jarOptions.Filename)
}

func setHeader(r *http.Request, contentType, origin, referer string) {
	// mimic firefox request headers
	r.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	r.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	r.Header.Set("Connection", "keep-alive")
	r.Header.Set("Content-Type", contentType)
	r.Header.Set("Origin", origin)
	r.Header.Set("Priority", "u=0, i")
	r.Header.Set("Referer",referer)
	r.Header.Set("Sec-Fetch-Dest", "document")
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Sec-Fetch-User", "?1")
	r.Header.Set("Sec-GPC", "1")
	r.Header.Set("TE", "trailers")
	r.Header.Set("Upgrade-Insecure-Requests", "1")
	r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
}

type debugTransport struct {
	transport  http.RoundTripper
	jar        http.CookieJar
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Get cookies from jar for this URL
    if d.jar != nil {
        cookies := d.jar.Cookies(req.URL)
        if len(cookies) > 0 {
            fmt.Printf("Cookies from jar for %s:\n", req.URL)
            for _, c := range cookies {
                fmt.Printf("  %s: %s\n", c.Name, c.Value)
            }
        }
		fmt.Println("")
    }
    
    // Dump request BEFORE sending (cookies will be in Header)
    dump, _ := httputil.DumpRequestOut(req, true)
    fmt.Println("=== REQUEST WITH COOKIES ===")
    fmt.Println(string(dump) + "\n")
    
    // Send the request
    resp, err := d.transport.RoundTrip(req)

	// Dump response AFTER sending
	dump, _ = httputil.DumpResponse(resp, false)
	fmt.Println("=== RESPONSE ===")
	fmt.Println(string(dump) + "\n")
    return resp, err
}

func getAuth(text, filter string, verbose bool) string {
	// initialise regexp pattern and scanner for text
	scanner := bufio.NewScanner(strings.NewReader(text))
	var i int
	pattern := regexp.MustCompile(`.*name="([^"]*)" value="([^"]*)".*`)

	// filter each line in text
	for scanner.Scan() {
		line := scanner.Text()

		if verbose {
			fmt.Printf("line %d: %s\n", i, line)
		}
		i += 1

		// filter line for literal terms
		if !strings.Contains(line, "authenticity") {
			continue
		}
		if !strings.Contains(line, filter) {
			continue
		}

		fmt.Printf("found: %s\n", line)

		// use regexp pattern to extract authenticity token
		matches := pattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		return matches[2]
	}

	return ""
}
