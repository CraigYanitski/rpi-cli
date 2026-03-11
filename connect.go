package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fereidani/httpdecompressor"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/pion/randutil"
	"github.com/pion/webrtc/v4"
)

type DeviceInfo struct {
	controller  string
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

func (cfg *apiConfig) rpiConnect() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	deviceURL := os.Getenv("RPI_DEVICE_URL")

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

	authToken := getAuth(string(body), "Raspberry", true)
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
	fmt.Printf("\n=== Final Response from %s ===\n\n%s\n", r.Header.Get("Host"), string(body))

	// extract and decode device information
	shellInfo := getSessionInformation(string(body))
	deviceInfo := &DeviceInfo{}
	fmt.Print("\n--- Device Information ---\n")
	if shellInfo != nil {
		s := html.UnescapeString(shellInfo[0])
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
		fmt.Printf("  string: %s\n", s)
		fmt.Printf("  controller: %s\n", deviceInfo.controller)
		fmt.Printf("  session-id: %s\n", deviceInfo.sessionId)
		fmt.Printf("  device: %s\n", deviceInfo.device)
		fmt.Printf("  ice-config: %s\n", deviceInfo.iceConfig)
	}

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

	// register data channel
	peerConnection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		fmt.Printf("New data channel: %s %d\n", dataChannel.Label(), *dataChannel.ID())

		// register opening
		dataChannel.OnOpen(func() {
			fmt.Printf("Data channel '%s - %d' open\n", dataChannel.Label(), dataChannel.ID())

			// detach data channel
			raw, dErr := dataChannel.Detach()
			if dErr != nil {
				log.Fatal(err)
			}

			// start read loop
			go ReadLoop(raw)

			// start write loop
			go WriteLoop(raw)
		})
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

	<-gatherComplete

	fmt.Println(encode(peerConnection.LocalDescription()))

	// send sdp to server
	//go cfg.createDeviceSession(offer)

	answer := webrtc.SessionDescription{}
	decode(<-sdpChan, &answer)

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
		"https://connect.raspberrypi.com/connections",
		bytes.NewBuffer([]byte(spdData)),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode >= 400 {
		log.Fatalf("Failed to create device session: %s\n", resp.Status)
	}

	// continue ICE
	return
}

const (
	messageSize = 16
)

func ReadLoop(d io.Reader) {
	for {
		buffer := make([]byte, messageSize)
		n, err := d.Read(buffer)
		if err != nil {
			fmt.Printf("data channel closed: %s\n", err)
			return
		}
		fmt.Printf("Received: %s", buffer[:n])
	}
}

func WriteLoop(d io.Writer) {
	ticker := time.NewTicker(5*time.Second)
	defer ticker.Stop()
	for range ticker.C {
		message, err := randutil.GenerateCryptoRandomString(
			messageSize, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Sending: %s\n", message)
		if _, err := d.Write([]byte(message)); err != nil {
			log.Fatal(err)
		}
	}
}

func encode(spd *webrtc.SessionDescription) string {
	b, err := json.Marshal(spd)
	if err != nil {
		log.Fatal(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

func decode(spdString string, spd *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(spdString)
	if err != nil {
		log.Fatal(err)
	}
	if err = json.Unmarshal(b, spd); err != nil {
		log.Fatal(err)
	}
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

