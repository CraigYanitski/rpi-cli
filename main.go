package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	//"net/http/httputil"
	"time"

	"github.com/CraigYanitski/rpi-cli/internal/utils"
	"github.com/briandowns/spinner"
	"github.com/charmbracelet/lipgloss"
	cookiejar "github.com/juju/persistent-cookiejar"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/http2"
)

var (
	completedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1ec001"))
	failedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cc0101"))
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
	ctx          context.Context
	//shCtx        context.Context
	rsCtx        context.Context
	closeChan    chan any
	client       *http.Client
	cookiejar    *cookiejar.Jar
	devices      [][]string
	webrtcAPI    *webrtc.API
	connections  []*webrtc.PeerConnection
	deviceInfo   *DeviceInfo
	sdpChan      chan string
}

func main() {
	// start program
	fmt.Println("Starting RPI-CLI")

	// open saved cookie jar (in default installed location)
	userHome, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	jarOptions := &cookiejar.Options{
		Filename: filepath.Join(userHome, ".rpi-cli/cookies.json"),
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
	setter := webrtc.SettingEngine{}
	setter.DetachDataChannels()

	// initialise api config
	api := apiConfig{
		closeChan: make(chan any),
		ctx:       context.Background(),
		client:    client,
		cookiejar: jar,
		webrtcAPI: webrtc.NewAPI(webrtc.WithSettingEngine(setter)),
		connections: []*webrtc.PeerConnection{},
		sdpChan:   make(chan string),
	}

	// ---- CONNECT TO SIGNALLING SERVER ----

	// start spinner notification while connecting to peer
	s := spinner.New(spinner.CharSets[11], 100 * time.Millisecond)
	s.Reverse()
	s.Color("magenta", "bold")
	s.Suffix = " Signing into RPI account"

	// sign into rpi id (required once for cookies)
	ok := api.rpiSignIn()
	if !ok {
		s.Stop()
		failMsg := failedStyle.Render("✗ Unable to sign into Raspberry Pi ID")
		fmt.Println(failMsg)
		os.Exit(1)
	}

	// update spinner progress
	s.Stop()
	fmt.Println(completedStyle.Render("✓"+s.Suffix))
	s.Suffix = " Connecting to signalling service"
	s.Start()

	// connect to rpi device
	ok = api.rpiConnect()
	if !ok {
		s.Stop()
		failMsg := failedStyle.Render("✗ Unable to obtain RPI devices")
		fmt.Println(failMsg)
		os.Exit(1)
	}

	// select device
	s.Stop()  // stop spinner for printing
	fmt.Println(completedStyle.Render("✓" + s.Suffix))
	deviceName, deviceURL := utils.GetDeviceURL(api.devices)

	// update spinner description
	s.Suffix = fmt.Sprintf(" Waiting for response from %s...", deviceName)
	s.Start()

	// negotiate peer-to-peer connection
	ok = api.connectDevice(deviceURL)
	if !ok {
		s.Stop()
		failMsg := failedStyle.Render(fmt.Sprintf("✗ Unable to connect to %s", deviceName))
		fmt.Println(failMsg)
		os.Exit(1)
	}
	defer func() {
		if err := api.connections[0].Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// stop spinner
	s.Stop()
	fmt.Println(completedStyle.Render("✓" + s.Suffix))

	// race to see which context closes first
	<-api.closeChan

	time.Sleep(100 * time.Millisecond)

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
	r.Header.Set("Referer", referer)
	r.Header.Set("Sec-Fetch-Dest", "document")
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("Sec-Fetch-User", "?1")
	r.Header.Set("Sec-GPC", "1")
	r.Header.Set("TE", "trailers")
	r.Header.Set("Upgrade-Insecure-Requests", "1")
	//r.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:147.0) Gecko/20100101 Firefox/147.0")
}

type debugTransport struct {
	transport  http.RoundTripper
	jar        http.CookieJar
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Get cookies from jar for this URL
    //if d.jar != nil {
    //    cookies := d.jar.Cookies(req.URL)
    //    if len(cookies) > 0 {
    //        fmt.Printf("Cookies from jar for %s:\n", req.URL)
    //        for _, c := range cookies {
    //            fmt.Printf("  %s: %s\n", c.Name, c.Value)
    //        }
    //    }
	//	fmt.Println("")
    //}
    
    // Dump request BEFORE sending (cookies will be in Header)
    //dump, _ := httputil.DumpRequestOut(req, true)
    //fmt.Println("=== REQUEST WITH COOKIES ===")
    //fmt.Println(string(dump) + "\n")
    
    // Send the request
    resp, err := d.transport.RoundTrip(req)

	// Dump response AFTER sending
	//dump, _ = httputil.DumpResponse(resp, false)
	//fmt.Println("=== RESPONSE ===")
	//fmt.Println(string(dump) + "\n")
    return resp, err
}

