package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fereidani/httpdecompressor"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
)

const (
	devicesPattern = `(?s)<tr>.*?href="\/devices\/([0-9a-f\-]*)">([^<]*)<\/a>`
)

type DeviceInfo struct {
	controller  string
	csrfToken   string
	sessionId   uuid.UUID
	device      DeviceData
	iceConfig   ICEConfig
}

type DeviceData struct {
	Id      uuid.UUID  `json:"id"`
	Name    string     `json:"name"`
	UserId  uuid.UUID  `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	SigningSecret  string  `json:"signing_secret"`
	SerialNumber   string  `json:"serial_number"`
	DaemonRetryAfter  any  `json:"daemon_retry_after"`
	OwnerType      string  `json:"owner_type,omitempty"`
	DaemonVersion  string  `json:"daemon_version"`
	DaemonDisconnectedAt  *time.Time  `json:"daemon_disconnected_at,omitempty"`
	LastAuthenticatedAt   time.Time  `json:"last_authenticated_at"`
	BoardRevision  any  `json:"board_revision,omitempty"`
	OsName         any  `json:"os_name,omitempty"`
	OsVersion      any  `json:"os_version,omitempty"`
	Arch           any  `json:"arch,omitempty"`
}

func (dd DeviceData) String() string {
	return fmt.Sprintf("\n        name (id): %s (%s)\n", dd.Name, dd.Id) +
	fmt.Sprintf("        secret: %s\n", dd.SigningSecret)
}

type ICEConfig struct {
	IceCandidatePoolSize  int                 `json:"iceCandidatePoolSize"`
	IceServers            []webrtc.ICEServer  `json:"iceServers"`
}

func (ic ICEConfig) String() string {
	str := strings.Builder{}
	fmt.Fprintf(&str, "\n    candidate pool size: %d\n", ic.IceCandidatePoolSize)
	for _, server := range ic.IceServers {
		str.WriteString("      urls:\n")
		for _, url := range server.URLs {
			fmt.Fprintf(&str, "        %s\n", url)
		}
		if server.Username != "" {
			fmt.Fprintf(&str, "      username: %s\n", server.Username)
		}
		if server.Credential != nil {
			fmt.Fprintf(&str, "      credential: %v\n", server.Credential)
		}
	}
	return str.String()
}

type SDPResponse struct {
	Id      string  `json:"id"`
	Answer  *struct {
		Type  string  `json:"type"`
		Sdp   string  `json:"sdp"`
	}  `json:"answer"`
}

func ptrBool(val bool) *bool {
	return &val
}

func ptrUint16(val uint16) *uint16 {
	return &val
}

func (cfg *apiConfig) rpiConnect() {
	// start spinner notification while connecting to peer
	s := spinner.New(spinner.CharSets[11], 100 * time.Millisecond)
	s.Reverse()
	s.Color("magenta", "bold")
	s.Suffix = " Connecting to signalling service"
	s.Start()

	// find way to list devices at connect.raspberrypi.com/devices
	r, err := http.NewRequest(
		"GET",
		"https://connect.raspberrypi.com/devices",
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	} else if resp.StatusCode >= 400 {
		log.Fatalf("Failed to sign into rpi account: received %s", resp.Status)
	}

	body, err := httpdecompressor.ReadAll(resp)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	authToken := getAuth(string(body), "Raspberry", false)
	authValues := url.Values{}
	authValues.Set("authenticity_token", authToken)
	authData :=  authValues.Encode()

	r, err = http.NewRequest(
		"POST",
		"https://connect.raspberrypi.com/auth/raspberry_pi",
		bytes.NewBuffer([]byte(authData)),
	)
	if err != nil {
		log.Fatal(err)
	}
	setHeader(
		r,
		"application/x-www-form-urlencoded",
		"https://connect.raspberrypi.com",
		"https://connect.raspberrypi.com/sign-in",
	)

	resp, err = cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	} else if resp.StatusCode >= 400 {
		log.Fatalf("Failed to authenticate for rpi connect: received %s", resp.Status)
	}

	body, err = httpdecompressor.ReadAll(resp)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	// get available devices
	s.Stop()  // stop spinner for printing
	devices := [][]string{}
	pattern := regexp.MustCompile(devicesPattern)
	matches := pattern.FindAllStringSubmatch(string(body), -1)
	for _, match := range matches {
		devices = append(devices, []string{match[1], match[2]})
	}

	// select device
	deviceURL := getDeviceURL(devices)

	r, err = http.NewRequest(
		"GET",
		deviceURL,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err = cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	} else if resp.StatusCode >= 400 {
		log.Fatalf("Failed to connect to device terminal: received %s", resp.Status)
	}

	body, err = httpdecompressor.ReadAll(resp)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	
	// available devices should appear in this response
	// fmt.Printf("\n=== Final Response from %s ===\n\n%s\n", r.Header.Get("Host"), string(body))



	// ===== WEBRTC =====


	// extract and decode device information
	shellInfo := getSessionInformation(string(body))
	deviceInfo := &DeviceInfo{}
	//fmt.Print("\n--- Device Information ---\n")
	if shellInfo != nil {
		// str := html.UnescapeString(shellInfo[0])
		deviceInfo.controller = html.UnescapeString(shellInfo[1])
		deviceInfo.sessionId, _ = uuid.Parse(html.UnescapeString(shellInfo[2]))
		d := html.UnescapeString(shellInfo[3])
		deviceData := &DeviceData{}
		if err = json.Unmarshal([]byte(d), deviceData); err != nil {
			log.Fatal(err)
		}
		deviceInfo.device = *deviceData
		ic := html.UnescapeString(shellInfo[4])
		iceConfig := &ICEConfig{}
		if err = json.Unmarshal([]byte(ic), iceConfig); err != nil {
			log.Fatal(err)
		}
		deviceInfo.iceConfig = *iceConfig
		// fmt.Printf("  string: %s\n", str)
		//fmt.Printf("  controller: %s\n", deviceInfo.controller)
		//fmt.Printf("  session-id: %s\n", deviceInfo.sessionId)
		//fmt.Printf("  device: %s\n", deviceInfo.device)
		//fmt.Printf("  ice-config: %s\n", deviceInfo.iceConfig)
	} else {
		log.Fatal("Unable to collect device information")
	}

	// update spinner description
	s.Suffix = fmt.Sprintf(" Waiting for response from %s...", deviceInfo.device.Name)
	s.Start()

	sessionToken := getSessionToken(string(body))
	if sessionToken != "" {
		deviceInfo.csrfToken = sessionToken
		//fmt.Printf("  csrf-token: %s\n", deviceInfo.csrfToken)
	} else {
		log.Fatal("Unable to collect CSRF token for session")
	}

	cfg.deviceInfo = deviceInfo

	config := webrtc.Configuration{
		ICEServers: deviceInfo.iceConfig.IceServers,
	}

	peerConnection, err := cfg.webrtcAPI.NewPeerConnection(config)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if cErr := peerConnection.Close(); cErr != nil {
			log.Fatal(err)
		}
	}()

	// notify of state change
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer connection state change: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed {
			fmt.Println("Peer connection has failed; exiting")
			os.Exit(0)
		}

		if state == webrtc.PeerConnectionStateClosed {
			fmt.Println("Peer connection closed")
			os.Exit(0)
		}
	})

	// create shell data channel
	shellChannel, err := peerConnection.CreateDataChannel("shell", &webrtc.DataChannelInit{
		Ordered: ptrBool(true),
		Negotiated: ptrBool(false),
		ID: ptrUint16(uint16(1)),
	})
	if err != nil {
		log.Fatal(err)
	}

	// create resize data channel
	resizeChannel, err := peerConnection.CreateDataChannel("resize", &webrtc.DataChannelInit{
		Ordered: ptrBool(true),
		Negotiated: ptrBool(false),
		ID: ptrUint16(uint16(0)),
	})
	if err != nil {
		log.Fatal(err)
	}

	// register shell data channel
	shellChannel.OnOpen(func() {
		fmt.Printf("Data channel (%d) '\"%s\" - %d' open\n", *shellChannel.ID(), shellChannel.Label(), shellChannel.ID())

		// detach data channel
		raw, err := shellChannel.Detach()
		if err != nil {
			log.Fatal(err)
		}

		// Create context for data channel
		ctx, cancel := context.WithCancel(cfg.ctx)
		cfg.shCtx = ctx
		defer cancel()

		// start read loop
		go cfg.ReadLoop(raw, cancel)

		// start write loop
		go cfg.WriteLoop(raw, cancel)
	})

	// register resize data channel
	resizeChannel.OnOpen(func() {
		fmt.Printf("Data channel (%d) '\"%s\" - %d' open\n", *resizeChannel.ID(), resizeChannel.Label(), resizeChannel.ID())

		// detach data channel
		//raw, dErr := resizeChannel.Detach()
		//if dErr != nil {
		//	log.Fatal(err)
		//}

		// Create context for data channel
		ctx, cancel := context.WithCancel(cfg.ctx)
		cfg.rsCtx = ctx
		defer cancel()

		resizeChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			cancel()
		})

		// start resize watch loop
		go cfg.watchResize(resizeChannel)
	})


	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		log.Fatal(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		log.Fatal(err)
	}

	//time.Sleep(1 * time.Second)

	<-gatherComplete

	//fmt.Println(string(encode(peerConnection.LocalDescription())))

	// send sdp to server
	go cfg.createDeviceSession(*peerConnection.LocalDescription())

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP: <-cfg.sdpChan,
	}
	//decode(<-cfg.sdpChan, &answer)

	// stop spinner
	s.Stop()

	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		log.Fatal(err)
	}

	// block forever
	select{}
}

func (cfg *apiConfig) createDeviceSession(offer webrtc.SessionDescription) {
	spdData := encode(&offer)
	r, err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://connect.raspberrypi.com/devices/%s/connections", cfg.deviceInfo.device.Id),
		bytes.NewBuffer([]byte(spdData)),
	)
	if err != nil {
		log.Fatal(err)
	}
	setHeader(
		r,
		"application/octet-stream",
		"https://connect.raspberrypi.com",
		fmt.Sprintf("https://connect.raspberrypi.com/devices/%s/remote-shell-session", cfg.deviceInfo.device.Id),
	)
	r.Header.Set("X-CSRF-Token", cfg.deviceInfo.csrfToken)

	resp, err := cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode >= 400 {
		log.Fatalf("Failed to create device session: %s\n", resp.Status)
	}
	location := resp.Header.Get("location")
	if location == "" {
		log.Fatal("Failed to create device session: No location in response\n")
	}

	sdpResponse := SDPResponse{}

	for {
		time.Sleep(500 * time.Millisecond)

		r, err := http.NewRequest(
			"GET",
			"https://connect.raspberrypi.com" + location,
			nil,
		)
		if err != nil {
			log.Fatal(err)
		}
		setHeader(
			r,
			"",
			"https://connect.raspberrypi.com",
			fmt.Sprintf(
				"https://connect.raspberrypi.com/devices/%s/remote-shell-session", 
				cfg.deviceInfo.device.Id,
			),
		)
		r.Header.Set("Accept", "*/*")
		r.Header.Set("Priority", "u=4")
		r.Header.Set("Sec-Fetch-Dest", "empty")
		r.Header.Set("Sec-Fetch-Mode", "cors")
		r.Header.Del("Content-Type")
		r.Header.Del("Origin")

		resp, err = cfg.client.Do(r)
		if err != nil {
			log.Fatal(err)
		}
		if resp.StatusCode >= 400 {
			log.Fatalf("Error checking client SDP status: %s\n", resp.Status)
		}

		body, err := httpdecompressor.ReadAll(resp)
		if err != nil {
			log.Fatal(err)
		}
		resp.Body.Close()

		err = json.Unmarshal(body, &sdpResponse)
		if err != nil {
			log.Fatal(err)
		}
		if sdpResponse.Answer != nil {
			break
		}
		sdpResponse = SDPResponse{}
	}

	// fmt.Printf("Accept: %s\n", sdpResponse.Answer.Sdp)

	cfg.sdpChan <- sdpResponse.Answer.Sdp
}

//func (cfg *apiConfig) WatchLoop(d io.Writer) {
//	rsChan := make(chan *WindowSize, 1)
//	go watchResize(rsChan)
//
//	for size := range rsChan {
//		if err := sendResize(d, size); err != nil {
//			log.Fatal(err)
//		}
//	}
//}

func encode(sdp *webrtc.SessionDescription) []byte {
	b, err := json.Marshal(sdp)
	if err != nil {
		log.Fatal(err)
	}

	return b //base64.StdEncoding.EncodeToString(b)
}

func getSessionInformation(body string) []string {
	pattern := regexp.MustCompile(
		`(?s).*data-controller="([^"]*)"` +
		`.*data-shell-session-id-value="([^"]*)"` +
		`.*data-shell-device-value="([^"]*)"` +
		`.*data-shell-ice-configuration-value="([^"]*)"` +
		`.*`,
	)
	matches := pattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 1 {
		return matches[0]
	}
	return nil
}

func getSessionToken(body string) string {
	pattern := regexp.MustCompile(`name="csrf-token" content="([^"]*)"`)
	matches := pattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 1 {
		return matches[0][1]
	}
	return ""
}

func getDeviceURL(devices [][]string) string {
	if devices == nil {
		return ""
	}

	if len(devices) == 1 {
		return fmt.Sprintf("https://connect.raspberrypi.com/devices/%s/remote-shell-session", devices[0][0])
	}

	fmt.Println("\nDevices\n-------")
	for i, name := range devices {
		fmt.Printf("%d: %s\n", i+1, name[1])
	}

	id := 0
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\nChoose from %d device(s): ", len(devices))
	for id == 0 {
		idStr, err := reader.ReadString('\n')
		if err != nil {
			err = nil
			fmt.Print("Error; please enter an integer: ")
			continue
		}

		id, err = strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			err = nil
			fmt.Print("Please enter an integer: ")
			continue
		} else if id <= 0 || id > len(devices) {
			id = 0
			fmt.Print("Please choose one of the available values: ")
			continue
		}
	}

	return fmt.Sprintf("https://connect.raspberrypi.com/devices/%s/remote-shell-session", devices[id-1][0])
}

