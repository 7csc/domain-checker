package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

var (
	filePath string
	verbose  bool
)

type Domain struct {
	Name  string         `toml:"name"`
	Ports map[string]int `toml:"ports"`
}

type Config struct {
	Domains []Domain `toml:"domains"`
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check domain status and cloud usage",
	Run:   runCheck,
}

func init() {
	checkCmd.Flags().StringVarP(&filePath, "file", "f", "domains.toml", "Path to TOML file")
	checkCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
}

func runCheck(cmd *cobra.Command, args []string) {
	config, err := loadConfig(filePath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	results := checkDomains(config.Domains)
	displayResults(config.Domains, results)
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func checkDomains(domains []Domain) []map[string]string {
	client := &http.Client{Timeout: 10 * time.Second}

	var results []map[string]string

	for _, domain := range domains {
		done := make(chan struct{})
		go showLoading(domain.Name, done)

		ipAddress, finalHost := getIpAddress(domain.Name)

		status := "deactive"
		cloud := "unknown"
		service := "unknown"

		if verbose {
			log.Printf("Checking: %s", domain.Name)
		}

		if checkConnectivity(client, finalHost) {
			status = "active"
			cloud, service = detectCloudProvider(ipAddress)
		}

		close(done)

		portResults := checkPorts(ipAddress, domain.Ports)

		mxRecord, err := getMXRecord(domain.Name)
		smtpResult := "-"
		if err == nil && len(mxRecord) > 0 {
			if checkSMTP(mxRecord[0]) {
				smtpResult = "open"
			}
		}

		result := map[string]string{
			"Domain":  domain.Name,
			"Status":  status,
			"IP":      ipAddress,
			"Cloud":   cloud,
			"Service": service,
			"SMTP":    smtpResult,
		}
		for portName, res := range portResults {
			result[portName] = res
		}

		results = append(results, result)
	}
	return results
}

func getIpAddress(domain string) (string, string) {
	ips, err := net.LookupIP(domain)
	if err == nil {
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String(), domain
			}
		}
	}

	cname, err := net.LookupCNAME(domain)
	if err == nil && cname != domain+"." {
		if verbose {
			log.Printf("CNAME found: %s → %s", domain, cname)
		}

		ips, err := net.LookupIP(cname)
		if err != nil {
			for _, ip := range ips {
				if ipv4 := ip.To4(); ipv4 != nil {
					return ipv4.String(), domain
				}
			}
		}
	}

	if verbose {
		log.Printf("Failed to resolve domain:%s (%v)", domain, err)
	}

	return "N/A", domain
}

func checkConnectivity(client *http.Client, finalHost string) bool {

	urls := []string{
		"https://" + finalHost,
		"http://" + finalHost,
	}

	for _, url := range urls {

		req, _ := http.NewRequest("HEAD", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			if verbose {
				log.Printf("Failed: %s (%v)", url, err)
			}
		} else {
			defer resp.Body.Close()
			if resp.StatusCode < 400 {
				if verbose {
					log.Printf("HEAD Success: %s (Status: %d)", url, resp.StatusCode)
				}
				return true
			}
		}

		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err = client.Do(req)
		if err != nil {
			if verbose {
				log.Printf("GET Failed: %s (%v)", url, err)
			}
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode < 400 {
			if verbose {
				log.Printf("GET Success: %s (Status: %d)", url, resp.StatusCode)
			}
			return true
		}
	}
	return false
}

func checkPorts(ip string, ports map[string]int) map[string]string {
	results := make(map[string]string)
	for name, port := range ports {
		if checkPortOpen(ip, port) {
			results[name] = "open"
		} else {
			results[name] = "-"
		}
	}
	return results
}

func checkPortOpen(ip string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func getMXRecord(domain string) ([]string, error) {
	mxRecord, err := net.LookupMX(domain)
	if err != nil {
		if verbose {
			log.Printf("Failed to lookup MX for %s: %v", domain, err)
		}
		return nil, err
	}
	var hosts []string
	for _, mx := range mxRecord {
		hosts = append(hosts, mx.Host)
	}
	return hosts, nil
}

func checkSMTP(host string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "25"), 5*time.Second)
	if err != nil {
		if verbose {
			log.Printf("Failed to connect SMTP: %s (%v)", host, err)
		}
		return false
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	buf := make([]byte, 1024)

	n, err := conn.Read(buf)
	if err != nil || !strings.HasPrefix(string(buf[:n]), "220") {
		return false
	}

	return true

}

func detectCloudProvider(ip string) (cloud, serive string) {
	if detectedService := getAWSService(ip); detectedService != "" {
		return "AWS", detectedService
	}

	addrs, err := net.LookupHost(ip)
	if err != nil {
		return "unknown", "unknown"
	}

	for _, addr := range addrs {
		if isGCPIP(addr) {
			return "GCP", "unknown"
		} else if isAzureIP(addr) {
			return "Azure", "unknown"
		}
	}
	return "unkown", "unknown"
}

func getAWSService(ip string) string {
	resp, err := http.Get("https://ip-ranges.amazonaws.com/ip-ranges.json")
	if err != nil {
		if verbose {
			log.Printf("Failed to fetch AWS IP ranges: %v", err)
		}
		return ""
	}
	defer resp.Body.Close()

	var data struct {
		Prefixes []struct {
			IPPrefix string `json:"ip_prefix"`
			Service  string `json:"service"`
		} `json:"prefixes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}

	parsedIP := net.ParseIP(ip)
	for _, prefix := range data.Prefixes {
		_, ipNet, err := net.ParseCIDR(prefix.IPPrefix)
		if err != nil {
			continue
		}
		if ipNet.Contains(parsedIP) {
			if prefix.Service == "AMAZON" {
				return "shared"
			}
			return prefix.Service
		}
	}
	return ""
}

func isGCPIP(ip string) bool {
	return checkIpInRanges(ip, "https://www.gstatic.com/ipranges/cloud.json", "prefixes", "ipv4Prefix")
}

func isAzureIP(ip string) bool {
	return strings.HasPrefix(ip, "20.") || strings.HasPrefix(ip, "40.") || strings.HasPrefix(ip, "52.")
}

func checkIpInRanges(ip, url, arrayKey, ipKey string) bool {
	resp, err := http.Get(url)
	if err != nil {
		if verbose {
			log.Printf("Failed to fetch IP ranges from %s: %v", url, err)
		}
		return false
	}
	defer resp.Body.Close()

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return false
	}

	ranges, ok := raw[arrayKey].([]any)
	if !ok {
		return false
	}

	parsedIP := net.ParseIP(ip)
	for _, r := range ranges {
		rmap := r.(map[string]any)
		cidr, ok := rmap[ipKey].(string)
		if !ok {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(parsedIP) {
			return true
		}
	}
	return false
}

func showLoading(domain string, done <-chan struct{}) {
	frames := []string{"/", "-", "\\", "|"}
	fmt.Printf("fetching %s ", domain)

	ticker := time.NewTimer(50 * time.Millisecond)
	defer ticker.Stop()

	for i := 0; ; i = (i + 1) % len(frames) {
		select {
		case <-done:
			fmt.Printf("\r\033[K")
			return
		case <-ticker.C:
			fmt.Printf("\rfetching %s ...%s", domain, frames[i])
		}
	}
}

const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBlue   = "\033[34m"
	colorReset  = "\033[0m"
)

func colorizeStatus(status string) string {
	switch status {
	case "active":
		return colorGreen + status + colorReset
	case "deactive":
		return colorRed + status + colorReset
	default:
		return status
	}
}

func colorizeCloud(cloud string) string {
	switch cloud {
	case "AWS":
		return colorYellow + cloud + colorReset
	case "Azure":
		return colorCyan + cloud + colorReset
	case "GCP":
		return colorBlue + cloud + colorReset
	default:
		return cloud
	}
}

func displayResults(domains []Domain, results []map[string]string) {
	table := tablewriter.NewWriter(os.Stdout)

	allPorts := collectAllPorts(domains)

	header := []string{"Domain", "Status", "Cloud", "Service", "IP", "SMTP"}

	for _, portName := range allPorts {
		header = append(header, strings.ToUpper(portName))
	}
	table.SetHeader(header)

	for _, result := range results {
		row := []string{
			result["Domain"],
			colorizeStatus(result["Status"]),
			colorizeCloud(result["Cloud"]),
			result["Service"],
			result["IP"],
			result["SMTP"],
		}
		for _, portName := range allPorts {
			if value, exists := result[portName]; exists {
				row = append(row, value)
			} else {
				row = append(row, "undefined")
			}
		}
		table.Append(row)
	}

	table.Render()
}

func collectAllPorts(domains []Domain) []string {
	portSet := make(map[string]struct{})
	for _, domain := range domains {
		for portName := range domain.Ports {
			portSet[portName] = struct{}{}
		}
	}

	var allPorts []string
	for portName := range portSet {
		allPorts = append(allPorts, portName)
	}

	sort.Strings(allPorts)

	return allPorts
}
