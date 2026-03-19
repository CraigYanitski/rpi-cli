package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	//"os"
	"regexp"
	"strings"
	"time"

	"github.com/CraigYanitski/rpi-cli/internal/utils"
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

func (cfg *apiConfig) rpiConnect() bool {
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

	authToken := utils.GetAuth(string(body), "Raspberry", false)
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
	cfg.devices = [][]string{}
	pattern := regexp.MustCompile(devicesPattern)
	matches := pattern.FindAllStringSubmatch(string(body), -1)
	for _, match := range matches {
		cfg.devices = append(cfg.devices, []string{match[1], match[2]})
	}

	return true
}

func (cfg *apiConfig) connectDevice(deviceURL string) bool {
	r, err := http.NewRequest(
		"GET",
		deviceURL,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	} else if resp.StatusCode >= 400 {
		log.Fatalf("Failed to connect to device terminal: received %s", resp.Status)
	}

	body, err := httpdecompressor.ReadAll(resp)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()



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

	// peer connection
	peerConnection, err := cfg.webrtcAPI.NewPeerConnection(config)
	if err != nil {
		log.Fatal(err)
	}

	// add peer connection to api
	cfg.connections = append(cfg.connections, peerConnection)

	// notify of state change
	cfg.connections[0].OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		// fmt.Printf("Peer connection state change: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed {
			fmt.Println("Peer connection has failed; exiting")
			//os.Exit(0)
		}

		if state == webrtc.PeerConnectionStateClosed {
			fmt.Println("Peer connection closed")
			//os.Exit(0)
		}
	})

	// register data channel handlers
	cfg.connections[0].OnDataChannel(func(d *webrtc.DataChannel) {
		// fmt.Printf("Data channel \"%s\" (id: %d)\n", d.Label(), *d.ID())

		if d.Label() == "resize" {
			d.OnOpen(func() {
				// Create context for data channel
				ctx, cancel := context.WithCancel(cfg.ctx)
				cfg.rsCtx = ctx

				d.OnMessage(func(msg webrtc.DataChannelMessage) {
					cancel()
					cfg.closeChan<- true
				})

				// start resize watch loop
				go cfg.watchResize(d)
			})
		}
	})

	// create shell data channel
	shellChannel, err := cfg.connections[0].CreateDataChannel("shell", &webrtc.DataChannelInit{
		Ordered: ptrBool(true),
		Negotiated: ptrBool(false),
		ID: ptrUint16(uint16(1)),
	})
	if err != nil {
		log.Fatal(err)
	}

	// register shell data channel
	shellChannel.OnOpen(func() {
		//fmt.Printf("Data channel \"%s\" (id: %d)\n", shellChannel.Label(), *shellChannel.ID())

		// detach data channel
		raw, err := shellChannel.Detach()
		if err != nil {
			log.Fatal(err)
		}

		// Create context for data channel
		//ctx, cancel := context.WithCancel(cfg.ctx)
		//cfg.shCtx = ctx

		// start read loop
		go cfg.ReadLoop(raw)

		// start write loop
		go cfg.WriteLoop(raw)
	})


	offer, err := cfg.connections[0].CreateOffer(nil)
	if err != nil {
		log.Fatal(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(cfg.connections[0])

	err = cfg.connections[0].SetLocalDescription(offer)
	if err != nil {
		log.Fatal(err)
	}

	<-gatherComplete

	// send sdp to server
	go cfg.createDeviceSession(*cfg.connections[0].LocalDescription())

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP: <-cfg.sdpChan,
	}

	// print local and remote SDPs
	//fmt.Println("LOCAL")
	//fmt.Println(peerConnection.LocalDescription().SDP)
	//fmt.Println("\nREMOTE")
	//fmt.Println(answer.SDP)

	err = cfg.connections[0].SetRemoteDescription(answer)
	if err != nil {
		log.Fatal(err)
	}

	// block forever
	//select{}
	return true
}

