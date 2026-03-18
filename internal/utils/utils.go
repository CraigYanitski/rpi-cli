package utils

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)


func GetCookieNames(jar http.CookieJar, domain string) ([]string, error) {
	names := []string{}
	domainURL, err := url.Parse(domain)
	if err != nil {
		return nil, err
	}
	cookies := jar.Cookies(domainURL)
	for _, cookie := range cookies {
		names = append(names, cookie.Name)
	}
	return names, nil
}

func GetAuth(text, filter string, verbose bool) string {
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

		//fmt.Printf("found: %s\n", line)

		// use regexp pattern to extract authenticity token
		matches := pattern.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		return matches[2]
	}

	return ""
}

func GetDeviceURL(devices [][]string) (string, string) {
	if devices == nil {
		return "", 
			""
	}

	if len(devices) == 1 {
		return devices[0][1], 
		fmt.Sprintf("https://connect.raspberrypi.com/devices/%s/remote-shell-session", devices[0][0])
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

	return devices[id-1][1], 
		fmt.Sprintf("https://connect.raspberrypi.com/devices/%s/remote-shell-session", devices[id-1][0])
}

