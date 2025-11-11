package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// IPEnrichment contains all enrichment data for an IP address
type IPEnrichment struct {
	IP          string `json:"ip"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	City        string `json:"city,omitempty"`
	ASN         int    `json:"asn,omitempty"`
	ASOrg       string `json:"as_org,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	IsPrivate   bool   `json:"is_private"`
}

// EnrichmentCache provides thread-safe caching for enrichment data
type EnrichmentCache struct {
	mu    sync.RWMutex
	cache map[string]*IPEnrichment
	ttl   time.Duration
}

var enrichmentCache = &EnrichmentCache{
	cache: make(map[string]*IPEnrichment),
	ttl:   24 * time.Hour, // Cache for 24 hours
}

// GeoIPRecord represents the MaxMind GeoIP data structure
type GeoIPRecord struct {
	Country struct {
		Names   map[string]string `maxminddb:"names"`
		ISOCode string            `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

// Get retrieves cached enrichment data
func (ec *EnrichmentCache) Get(ip string) (*IPEnrichment, bool) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if data, exists := ec.cache[ip]; exists {
		return data, true
	}
	return nil, false
}

// Set stores enrichment data in cache
func (ec *EnrichmentCache) Set(ip string, data *IPEnrichment) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.cache[ip] = data
}

// enrichIPWithGeoIP adds country and city information from MaxMind database
func enrichIPWithGeoIP(ip string, enrichment *IPEnrichment) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return fmt.Errorf("invalid IP address: %w", err)
	}

	var record GeoIPRecord
	err = config.Mmdb.Lookup(addr).Decode(&record)
	if err != nil {
		// Not an error if IP is not in database (e.g., private IPs)
		return nil
	}

	// Get country name (prefer English)
	if countryName, ok := record.Country.Names["en"]; ok {
		enrichment.Country = countryName
	}
	enrichment.CountryCode = record.Country.ISOCode

	// Get city name (prefer English)
	if cityName, ok := record.City.Names["en"]; ok {
		enrichment.City = cityName
	}

	return nil
}

// enrichIPWithReverseDNS adds hostname via reverse DNS lookup
func enrichIPWithReverseDNS(ip string, enrichment *IPEnrichment) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resolver := &net.Resolver{}
	names, err := resolver.LookupAddr(ctx, ip)
	if err != nil {
		// Not an error if reverse DNS fails
		return nil
	}

	if len(names) > 0 {
		hostname := strings.TrimSuffix(names[0], ".")
		enrichment.Hostname = hostname

		// Try to extract service name from hostname
		enrichment.ServiceName = extractServiceNameFromHostname(hostname)
	}

	return nil
}

// extractServiceNameFromHostname tries to identify well-known services from hostname
func extractServiceNameFromHostname(hostname string) string {
	hostname = strings.ToLower(hostname)

	// Common service patterns
	servicePatterns := map[string][]string{
		"Google":     {"google.com", "googleapis.com", "googleusercontent.com", "gstatic.com", "youtube.com", "ytimg.com"},
		"Amazon/AWS": {"amazon.com", "amazonaws.com", "awsstatic.com", "cloudfront.net"},
		"Microsoft":  {"microsoft.com", "msn.com", "live.com", "outlook.com", "office.com", "azure.com"},
		"Netflix":    {"netflix.com", "nflxvideo.net", "nflximg.net", "nflxext.com"},
		"Facebook":   {"facebook.com", "fbcdn.net", "instagram.com", "whatsapp.com"},
		"Apple":      {"apple.com", "icloud.com", "mzstatic.com", "applemusic.com"},
		"Cloudflare": {"cloudflare.com", "cloudflare.net"},
		"Akamai":     {"akamai.net", "akamaiedge.net", "akamaihd.net"},
		"Twitter/X":  {"twitter.com", "twimg.com", "x.com"},
		"LinkedIn":   {"linkedin.com", "licdn.com"},
		"Zoom":       {"zoom.us", "zoom.com"},
		"Dropbox":    {"dropbox.com", "dropboxapi.com"},
		"Spotify":    {"spotify.com", "scdn.co"},
		"Adobe":      {"adobe.com", "adobedc.net"},
		"Salesforce": {"salesforce.com", "force.com"},
		"Oracle":     {"oracle.com", "oraclecloud.com"},
		"IBM":        {"ibm.com", "ibmcloud.com"},
		"GitHub":     {"github.com", "githubusercontent.com"},
		"Reddit":     {"reddit.com", "redd.it", "redditmedia.com"},
		"Wikipedia":  {"wikipedia.org", "wikimedia.org"},
		"Twitch":     {"twitch.tv", "ttvnw.net"},
		"Discord":    {"discord.com", "discordapp.com"},
		"TikTok":     {"tiktok.com", "tiktokcdn.com"},
		"WhatsApp":   {"whatsapp.com", "whatsapp.net"},
		"Telegram":   {"telegram.org", "t.me"},
	}

	for service, patterns := range servicePatterns {
		for _, pattern := range patterns {
			if strings.Contains(hostname, pattern) {
				return service
			}
		}
	}

	// Try to extract from common patterns like "service.example.com"
	parts := strings.Split(hostname, ".")
	if len(parts) >= 2 {
		domain := parts[len(parts)-2]
		// Capitalize first letter
		if len(domain) > 0 {
			return strings.ToUpper(domain[:1]) + domain[1:]
		}
	}

	return ""
}

// isPrivateIP checks if an IP is private/local
func isPrivateIP(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	return addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast()
}

// EnrichIP performs full enrichment on an IP address
func EnrichIP(ip string) *IPEnrichment {
	// Check cache first
	if cached, exists := enrichmentCache.Get(ip); exists {
		return cached
	}

	enrichment := &IPEnrichment{
		IP:        ip,
		IsPrivate: isPrivateIP(ip),
	}

	// Skip enrichment for private IPs (except local service names)
	if enrichment.IsPrivate {
		// Check if it's in the services table for local names
		if serviceName := getLocalServiceName(ip); serviceName != "" {
			enrichment.ServiceName = serviceName
		}
		enrichmentCache.Set(ip, enrichment)
		return enrichment
	}

	// Enrich with GeoIP data
	if err := enrichIPWithGeoIP(ip, enrichment); err != nil {
		log.Printf("GeoIP enrichment error for %s: %v", ip, err)
	}

	// Enrich with reverse DNS (with timeout)
	if err := enrichIPWithReverseDNS(ip, enrichment); err != nil {
		log.Printf("Reverse DNS enrichment error for %s: %v", ip, err)
	}

	// Cache the result
	enrichmentCache.Set(ip, enrichment)

	return enrichment
}

// getLocalServiceName checks the services table for local IP names
func getLocalServiceName(ip string) string {
	// Check service networks from the database
	networks := getServiceNetworks()
	for _, network := range networks {
		if ipMatchesNetwork(ip, network.CIDR) {
			return network.Name
		}
	}
	return ""
}

// ipMatchesNetwork checks if an IP matches a CIDR network
func ipMatchesNetwork(ip string, cidr string) bool {
	ipAddr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return false
	}

	return prefix.Contains(ipAddr)
}

// EnrichIPs enriches multiple IPs concurrently
func EnrichIPs(ips []string) map[string]*IPEnrichment {
	results := make(map[string]*IPEnrichment)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrent DNS lookups
	semaphore := make(chan struct{}, 10)

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			enrichment := EnrichIP(ip)
			mu.Lock()
			results[ip] = enrichment
			mu.Unlock()
		}(ip)
	}

	wg.Wait()
	return results
}

// getEnrichmentRequest handles HTTP requests for IP enrichment
func getEnrichmentRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get IP from query parameter
	ip := r.URL.Query().Get("ip")
	if ip == "" {
		http.Error(w, `{"error": "ip parameter is required"}`, http.StatusBadRequest)
		return
	}

	// Validate IP
	if net.ParseIP(ip) == nil {
		http.Error(w, `{"error": "invalid IP address"}`, http.StatusBadRequest)
		return
	}

	enrichment := EnrichIP(ip)

	jsonBytes, err := json.Marshal(enrichment)
	if err != nil {
		log.Printf("Error marshaling enrichment: %v", err)
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}

// getBulkEnrichmentRequest handles HTTP requests for bulk IP enrichment
func getBulkEnrichmentRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var request struct {
		IPs []string `json:"ips"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	if len(request.IPs) == 0 {
		http.Error(w, `{"error": "no IPs provided"}`, http.StatusBadRequest)
		return
	}

	// Limit to prevent abuse
	if len(request.IPs) > 1000 {
		http.Error(w, `{"error": "too many IPs (max 1000)"}`, http.StatusBadRequest)
		return
	}

	// Enrich all IPs
	enrichments := EnrichIPs(request.IPs)

	jsonBytes, err := json.Marshal(enrichments)
	if err != nil {
		log.Printf("Error marshaling enrichments: %v", err)
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}

// getProtocolNameFromNumber returns the protocol name for a given protocol number
func getProtocolNameFromNumber(protocol int) string {
	protocols := map[int]string{
		1:   "ICMP",
		2:   "IGMP",
		6:   "TCP",
		17:  "UDP",
		41:  "IPv6",
		47:  "GRE",
		50:  "ESP",
		51:  "AH",
		58:  "ICMPv6",
		88:  "EIGRP",
		89:  "OSPF",
		103: "PIM",
		115: "L2TP",
		132: "SCTP",
	}

	if name, ok := protocols[protocol]; ok {
		return name
	}
	return fmt.Sprintf("Protocol %d", protocol)
}
