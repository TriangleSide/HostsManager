package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"strings"
	"unicode"
)

// TODO: make this OS and OS version aware.
const hostFilePath = "/etc/hosts"

const (
	startManagedHosts = "# DO NOT MODIFY MANUALLY. Managed hosts start.\n"
	endManagedHosts   = "# DO NOT MODIFY MANUALLY. Managed hosts end.\n"
)

// The base host file has these prepended in the StevenBlack project, ignore them.
var ignoreListInDownloadedFiles = map[string]bool{
	"localhost":             true,
	"localhost.localdomain": true,
	"local":                 true,
	"broadcasthost":         true,
	"ip6-localhost":         true,
	"ip6-loopback":          true,
	"ip6-localnet":          true,
	"ip6-mcastprefix":       true,
	"ip6-allnodes":          true,
	"ip6-allrouters":        true,
	"ip6-allhosts":          true,
	"0.0.0.0":               true,
	"::":                    true,
	"::0":                   true,
}

type HostFileSource struct {
	Name   string
	Source string
}

var hostFileSources = []HostFileSource{
	{
		Name:   "Unified (adware + malware)",
		Source: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
	},
	{
		Name:   "Fake News",
		Source: "https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/fakenews-only/hosts",
	},
	{
		Name:   "Gambling",
		Source: "https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/gambling-only/hosts",
	},
	{
		Name:   "Pornography",
		Source: "https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/porn-only/hosts",
	},
	{
		Name:   "Social Media",
		Source: "https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/social-only/hosts",
	},
}

func downloadHostSource(source HostFileSource) string {
	resp, err := http.Get(source.Source)
	if err != nil {
		fmt.Println("Error making the HTTPS call to get the hosts file: ", err)
		os.Exit(1)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			fmt.Println("Error closing response body: ", err)
			os.Exit(1)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Server returned non-200 status: %d %s\n", resp.StatusCode, resp.Status)
		os.Exit(1)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		os.Exit(1)
	}

	return string(data)
}

func main() {
	fmt.Println("Welcome to the hosts manager.")

	fmt.Println("Ensuring the program has the correct permissions.")
	currentUser, err := user.Current()
	if err != nil {
		fmt.Println("Error getting the current user: ", err)
		os.Exit(1)
	}
	if currentUser.Uid != "0" {
		fmt.Println("This application needs root permissions to run.")
		os.Exit(1)
	}

	fmt.Println("Select hosts sources to use.")
	hostsToDownload := make([]HostFileSource, 0)
	for _, source := range hostFileSources {
		for {
			fmt.Println("Would you like to include the hosts file titled '" + source.Name + "'?")
			fmt.Print("[Y/n]: ")

			var userInput string
			_, err := fmt.Scanln(&userInput)
			if err != nil {
				fmt.Println("Error reading input: ", err)
				os.Exit(1)
			}
			userInput = strings.ToLower(userInput)
			userInput = strings.TrimSpace(userInput)

			if userInput == "y" {
				hostsToDownload = append(hostsToDownload, source)
				break
			} else if userInput == "n" {
				break
			} else {
				fmt.Println("Invalid input.")
			}
		}
	}
	fmt.Println("")

	fmt.Println("Ensuring a least one source is selected.")
	if len(hostsToDownload) == 0 {
		fmt.Println("No host source selected.")
		os.Exit(1)
	}
	fmt.Println("")

	fmt.Println("Downloading hosts files.")
	rawHostFiles := make(map[string]string)
	for _, source := range hostsToDownload {
		fmt.Println("Downloading source for '" + source.Name + "'")
		rawHostFiles[source.Name] = downloadHostSource(source)
	}
	fmt.Println("")

	fmt.Println("Processing host files.")
	hosts := make([]string, 0)
	for sourceName, rawHostFile := range rawHostFiles {
		fmt.Printf("Processing %s.\n", sourceName)
		hostsEntries := strings.Split(rawHostFile, "\n")
		for _, originalEntry := range hostsEntries {
			entry := strings.Split(originalEntry, "#")[0]
			entry = strings.TrimSpace(entry)
			if len(entry) == 0 {
				continue
			}
			host := ""
			for i := len(entry) - 1; i >= 0; i-- {
				if unicode.IsSpace(rune(entry[i])) {
					host = entry[i+1:]
				}
			}
			if len(host) == 0 {
				fmt.Println("Error parsing entry '" + originalEntry + "'.")
			}
			hosts = append(hosts, host)
		}
	}
	fmt.Println("")

	fmt.Println("Ensuring host entries are unique.")
	uniqueHosts := make(map[string]bool)
	for _, host := range hosts {
		if _, ok := ignoreListInDownloadedFiles[host]; !ok {
			uniqueHosts[host] = true
		}
	}
	fmt.Println("")

	fmt.Println("Reading local host file.")
	hostFileDataBytes, err := os.ReadFile(hostFilePath)
	if err != nil {
		fmt.Printf("Failed to read host file: %s", err)
		os.Exit(1)
	}
	hostFileDataStr := string(hostFileDataBytes)

	fmt.Println("Verifying if hosts file is previously managed.")
	if strings.Contains(hostFileDataStr, startManagedHosts) {
		hostFileDataStr = strings.Replace(hostFileDataStr, "\n\n"+startManagedHosts, "\n"+startManagedHosts, 1)
		startIndex := strings.Index(hostFileDataStr, startManagedHosts)
		if startIndex == -1 {
			fmt.Println("Malformatted host file. Could not find start index of block.")
			os.Exit(1)
		}
		endIndex := strings.LastIndex(hostFileDataStr, endManagedHosts)
		if startIndex == -1 {
			fmt.Println("Malformatted host file. Could not find end index of block.")
			os.Exit(1)
		}
		hostFileDataStr = hostFileDataStr[0:startIndex] + hostFileDataStr[endIndex+len(endManagedHosts):]
	}

	fmt.Println("Backing up old host file.")
	if _, err := os.Stat(hostFilePath); err != nil {
		fmt.Println("Could not find existing host file.")
		os.Exit(1)
	}
	err = os.Rename(hostFilePath, hostFilePath+".bak")
	if err != nil {
		fmt.Println("Failed to backup the hosts file.")
		os.Exit(1)
	}

	fmt.Println("Creating new hosts file.")
	file, err := os.OpenFile(hostFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("Failed to create the new hosts file.")
		os.Exit(1)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)

	fmt.Println("Writing new hosts file.")
	write := func(text string) {
		_, err = writer.WriteString(text)
		if err != nil {
			fmt.Println("Failed while writing to the new host file.", err)
			os.Exit(1)
		}
	}
	write(hostFileDataStr + "\n")
	write(startManagedHosts)
	for host, _ := range uniqueHosts {
		write("0.0.0.0" + " " + host + "\n")
		write("::0" + " " + host + "\n")
	}
	write(endManagedHosts)
	if err := writer.Flush(); err != nil {
		fmt.Println("Failed to flush the hosts file.", err)
		os.Exit(1)
	}
	fmt.Println("")

	fmt.Printf("Added %d hosts entries.\n", len(uniqueHosts))
	os.Exit(0)
}
