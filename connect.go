package main

import (
	"bytes"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/fereidani/httpdecompressor"
	"github.com/joho/godotenv"
)

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
	fmt.Print("\n--- Device Information ---\n")
	if shellInfo != nil {
		s := html.UnescapeString(shellInfo[0])
		c := html.UnescapeString(shellInfo[1])
		si := html.UnescapeString(shellInfo[2])
		d := html.UnescapeString(shellInfo[3])
		ic := html.UnescapeString(shellInfo[4])
		fmt.Printf("  string: %s\n", s)
		fmt.Printf("  controller: %s\n", c)
		fmt.Printf("  session-id: %s\n", si)
		fmt.Printf("  device: %s\n", d)
		fmt.Printf("  ice-config: %s\n", ic)
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

