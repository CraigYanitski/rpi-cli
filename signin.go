package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/CraigYanitski/rpi-cli/internal/utils"
	"github.com/fereidani/httpdecompressor"
	"github.com/joho/godotenv"
)

func (cfg *apiConfig) rpiSignIn() bool {
	// check if session_id cookie exists (this is long-lived and used to bypass the signin step)
	idCookies, err := utils.GetCookieNames(cfg.client.Jar, "https://id.raspberrypi.com")
	if err != nil {
		log.Print(err)
		return false
	}
	isSignedIn := slices.Contains(idCookies, "session_id")
	if isSignedIn {
		return true
	}

	err = godotenv.Load(".env")
	if err != nil {
		log.Print("failed to load .env file")
	}

	rpiEmail := os.Getenv("RPI_EMAIL")
	rpiPW := os.Getenv("RPI_PW")

	// get sign-in information, grep authority and hidden to make sign-in obj
	r, err := http.NewRequest("GET", "https://id.raspberrypi.com/sign-in", nil)
	if err != nil {
		log.Print(err)
		return false
	}

	resp, err := cfg.client.Do(r)
	if err != nil {
		log.Print(err)
		return false
	} else if resp.StatusCode >= 400 {
		log.Printf("Failed to get sign-in information: received %s", resp.Status)
		return false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return false
	}
	resp.Body.Close()

	authToken := utils.GetAuth(string(body), "hidden", true)
	signIn := SignIn{
		AuthToken: authToken, 
		Email: rpiEmail, 
		Password: rpiPW, 
		Commit: "Sign in",
	}
	signInValues := url.Values{}
	signInValues.Set("authenticity_token", signIn.AuthToken)
	signInValues.Set("email", signIn.Email)
	signInValues.Set("password", signIn.Password)
	signInValues.Set("commit", signIn.Commit)
	signInData := signInValues.Encode()
	fmt.Printf("\n%s\n\n", signInData)

	// post obj to /session, grep authority and hidden, and get OTP to make verify obj
	r, err = http.NewRequest(
		"POST", 
		"https://id.raspberrypi.com/session", 
		bytes.NewBuffer([]byte(signInData)),
	)
	if err != nil {
		log.Print(err)
		return false
	}
	setHeader(
		r, 
		"application/x-www-form-urlencoded", 
		"https://id.raspberrypi.com", 
		"https://id.raspberrypi.com/sign-in",
	)

	resp, err = cfg.client.Do(r)
	if err != nil {
		log.Print(err)
		return false
	} else if resp.StatusCode == 400 || resp.StatusCode > 401 {
		log.Printf("Failed to sign into rpi account: received %s", resp.Status)
		return false
	}
	body, err = httpdecompressor.ReadAll(resp)
	if err != nil {
		log.Print(err)
		return false
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("2FA: ")
	otp, _ := reader.ReadString('\n')

	authToken = utils.GetAuth(string(body), "hidden", true)
	verify := Verify{
		AuthToken: authToken, 
		OTP: strings.TrimSpace(otp), 
		Commit: "Verify and sign in",
	}
	verifyValues := url.Values{}
	verifyValues.Set("authenticity_token", verify.AuthToken)
	verifyValues.Set("otp", verify.OTP)
	verifyValues.Set("commit", verify.Commit)
	verifyData := verifyValues.Encode()

	// post obj to /session/verify, then follow through location in resp
	r, err = http.NewRequest(
		"POST",
		"https://id.raspberrypi.com/session/verify", 
		bytes.NewBuffer([]byte(verifyData)),
	)
	if err != nil {
		log.Print(err)
		return false
	}
	setHeader(
		r,
		"application/x-www-form-urlencoded",
		"https://id.raspberrypi.com",
		"https://id.raspberrypi.com/session",
	)

	resp, err = cfg.client.Do(r)
	if err != nil {
		log.Print(err)
		return false
	} else if resp.StatusCode >= 400 {
		log.Printf("Failed to verify the rpi account: received %s", resp.Status)
		return false
	}
	return true
}
