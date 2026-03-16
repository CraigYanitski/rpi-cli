package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/fereidani/httpdecompressor"
	"github.com/pion/webrtc/v4"
)


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
