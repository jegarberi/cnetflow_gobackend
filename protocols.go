package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ProtocolStats represents aggregated statistics for a protocol
type ProtocolStats struct {
	Protocol      int     `json:"protocol"`
	ProtocolName  string  `json:"protocol_name"`
	TotalOctets   int64   `json:"total_octets"`
	TotalPackets  int64   `json:"total_packets"`
	FlowCount     int64   `json:"flow_count"`
	Percentage    float64 `json:"percentage"`
	AvgPacketSize float64 `json:"avg_packet_size"`
}

// ProtocolPortStats represents statistics for protocol+port combinations
type ProtocolPortStats struct {
	Protocol     int     `json:"protocol"`
	ProtocolName string  `json:"protocol_name"`
	SrcPort      int     `json:"srcport,omitempty"`
	DstPort      int     `json:"dstport,omitempty"`
	ServiceName  string  `json:"service_name,omitempty"`
	TotalOctets  int64   `json:"total_octets"`
	TotalPackets int64   `json:"total_packets"`
	FlowCount    int64   `json:"flow_count"`
	Percentage   float64 `json:"percentage"`
}

// ProtocolTimeSeriesPoint represents protocol data at a time point
type ProtocolTimeSeriesPoint struct {
	Timestamp    time.Time     `json:"timestamp"`
	ProtocolData map[int]int64 `json:"protocol_data"` // protocol -> bytes
}

// ProtocolAnalysisResponse contains the full response
type ProtocolAnalysisResponse struct {
	ProtocolStats []ProtocolStats           `json:"protocol_stats"`
	TopPorts      []ProtocolPortStats       `json:"top_ports,omitempty"`
	TimeSeries    []ProtocolTimeSeriesPoint `json:"time_series,omitempty"`
	TotalOctets   int64                     `json:"total_octets"`
	TotalPackets  int64                     `json:"total_packets"`
	TotalFlows    int64                     `json:"total_flows"`
}

// getProtocolName returns the name of a protocol number
func getProtocolName(protocol int) string {
	protocols := map[int]string{
		1:   "ICMP",
		6:   "TCP",
		17:  "UDP",
		41:  "IPv6",
		47:  "GRE",
		50:  "ESP",
		51:  "AH",
		58:  "ICMPv6",
		89:  "OSPF",
		132: "SCTP",
	}
	if name, ok := protocols[protocol]; ok {
		return name
	}
	return fmt.Sprintf("Protocol %d", protocol)
}

// parseProtocolFilter extracts filter parameters from request
func parseProtocolFilter(r *http.Request) TrafficFilter {
	filter := TrafficFilter{
		Direction: "input",
		Limit:     100,
		OrderBy:   "total_octets",
		OrderDir:  "desc",
	}

	query := r.URL.Query()

	filter.Exporter = query.Get("exporter")
	filter.Interface = query.Get("interface")
	filter.Direction = query.Get("direction")
	filter.Protocol = query.Get("protocol")

	// Time filters
	// Note: Database stores timestamps without timezone in GMT-3
	// Apply -3 hour offset to incoming UTC times
	if startStr := query.Get("start"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartTime = t.Add(-3 * time.Hour)
		}
	}
	if endStr := query.Get("end"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndTime = t.Add(-3 * time.Hour)
		}
	}

	// Default to last hour if not specified
	if filter.StartTime.IsZero() {
		filter.StartTime = time.Now().Add(-1 * time.Hour)
	}
	if filter.EndTime.IsZero() {
		filter.EndTime = time.Now()
	}

	return filter
}

// getProtocolStats retrieves aggregated protocol statistics
func getProtocolStats(filter TrafficFilter) ([]ProtocolStats, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// Build WHERE clause
	if filter.Exporter != "" {
		conditions = append(conditions, fmt.Sprintf("exporter = $%d::inet", argIndex))
		args = append(args, filter.Exporter)
		argIndex++
	}

	if filter.Interface != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", filter.Direction, argIndex))
		args = append(args, filter.Interface)
		argIndex++
	}

	if !filter.StartTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("bucket_5min AT TIME ZONE 'UTC' >= $%d", argIndex))
		args = append(args, filter.StartTime)
		argIndex++
	}

	if !filter.EndTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("bucket_5min AT TIME ZONE 'UTC' <= $%d", argIndex))
		args = append(args, filter.EndTime)
		argIndex++
	}

	if filter.Protocol != "" {
		conditions = append(conditions, fmt.Sprintf("prot = $%d", argIndex))
		args = append(args, filter.Protocol)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			prot as protocol,
			SUM(total_octets) as total_octets,
			SUM(total_packets) as total_packets,
			COUNT(*) as flow_count
		FROM flows_agg_5min
		%s
		GROUP BY prot
		ORDER BY total_octets DESC
	`, whereClause)

	log.Println("Protocol stats query:", query)
	log.Println("Args:", args)

	rows, err := config.Db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var stats []ProtocolStats
	var totalOctets int64

	for rows.Next() {
		var stat ProtocolStats
		var protocol sql.NullInt64

		err := rows.Scan(
			&protocol,
			&stat.TotalOctets,
			&stat.TotalPackets,
			&stat.FlowCount,
		)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		if protocol.Valid {
			stat.Protocol = int(protocol.Int64)
			stat.ProtocolName = getProtocolName(stat.Protocol)
		}

		if stat.TotalPackets > 0 {
			stat.AvgPacketSize = float64(stat.TotalOctets) / float64(stat.TotalPackets)
		}

		totalOctets += stat.TotalOctets
		stats = append(stats, stat)
	}

	// Calculate percentages
	for i := range stats {
		if totalOctets > 0 {
			stats[i].Percentage = float64(stats[i].TotalOctets) / float64(totalOctets) * 100
		}
	}

	return stats, nil
}

// getProtocolPortStats retrieves protocol+port statistics
func getProtocolPortStats(filter TrafficFilter, limit int) ([]ProtocolPortStats, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// Build WHERE clause
	if filter.Exporter != "" {
		conditions = append(conditions, fmt.Sprintf("exporter = $%d::inet", argIndex))
		args = append(args, filter.Exporter)
		argIndex++
	}

	if filter.Interface != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", filter.Direction, argIndex))
		args = append(args, filter.Interface)
		argIndex++
	}

	if !filter.StartTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("bucket_5min AT TIME ZONE 'UTC' >= $%d", argIndex))
		args = append(args, filter.StartTime)
		argIndex++
	}

	if !filter.EndTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("bucket_5min AT TIME ZONE 'UTC' <= $%d", argIndex))
		args = append(args, filter.EndTime)
		argIndex++
	}

	if filter.Protocol != "" {
		conditions = append(conditions, fmt.Sprintf("prot = $%d", argIndex))
		args = append(args, filter.Protocol)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			prot as protocol,
			srcport,
			dstport,
			SUM(total_octets) as total_octets,
			SUM(total_packets) as total_packets,
			COUNT(*) as flow_count
		FROM flows_agg_5min
		%s
		GROUP BY prot, srcport, dstport
		ORDER BY total_octets DESC
		LIMIT %d
	`, whereClause, limit)

	log.Println("Protocol port stats query:", query)

	rows, err := config.Db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var stats []ProtocolPortStats
	var totalOctets int64

	// Get service names map
	services := getPorts()
	serviceMap := make(map[string]string)
	for _, svc := range services {
		key := fmt.Sprintf("%d:%d", svc.Protocol, svc.Port)
		serviceMap[key] = svc.Name
	}

	for rows.Next() {
		var stat ProtocolPortStats
		var protocol, srcPort, dstPort sql.NullInt64

		err := rows.Scan(
			&protocol,
			&srcPort,
			&dstPort,
			&stat.TotalOctets,
			&stat.TotalPackets,
			&stat.FlowCount,
		)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		if protocol.Valid {
			stat.Protocol = int(protocol.Int64)
			stat.ProtocolName = getProtocolName(stat.Protocol)
		}

		if srcPort.Valid {
			stat.SrcPort = int(srcPort.Int64)
		}
		if dstPort.Valid {
			stat.DstPort = int(dstPort.Int64)
		}

		// Try to find service name from destination port first
		if dstPort.Valid {
			key := fmt.Sprintf("%d:%d", stat.Protocol, stat.DstPort)
			if name, ok := serviceMap[key]; ok {
				stat.ServiceName = name
			}
		}
		// If not found, try source port
		if stat.ServiceName == "" && srcPort.Valid {
			key := fmt.Sprintf("%d:%d", stat.Protocol, stat.SrcPort)
			if name, ok := serviceMap[key]; ok {
				stat.ServiceName = name + " (src)"
			}
		}

		totalOctets += stat.TotalOctets
		stats = append(stats, stat)
	}

	// Calculate percentages
	for i := range stats {
		if totalOctets > 0 {
			stats[i].Percentage = float64(stats[i].TotalOctets) / float64(totalOctets) * 100
		}
	}

	return stats, nil
}

// getProtocolTimeSeries retrieves time series data for protocols
func getProtocolTimeSeries(filter TrafficFilter) ([]ProtocolTimeSeriesPoint, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// Build WHERE clause
	if filter.Exporter != "" {
		conditions = append(conditions, fmt.Sprintf("exporter = $%d::inet", argIndex))
		args = append(args, filter.Exporter)
		argIndex++
	}

	if filter.Interface != "" {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", filter.Direction, argIndex))
		args = append(args, filter.Interface)
		argIndex++
	}

	if !filter.StartTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("bucket_5min AT TIME ZONE 'UTC' >= $%d", argIndex))
		args = append(args, filter.StartTime)
		argIndex++
	}

	if !filter.EndTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("bucket_5min AT TIME ZONE 'UTC' <= $%d", argIndex))
		args = append(args, filter.EndTime)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			bucket_5min,
			prot,
			SUM(total_octets) as total_octets
		FROM flows_agg_5min
		%s
		GROUP BY bucket_5min, prot
		ORDER BY bucket_5min ASC, prot ASC
	`, whereClause)

	log.Println("Protocol time series query:", query)

	rows, err := config.Db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	// Build time series data
	timeSeriesMap := make(map[time.Time]map[int]int64)

	for rows.Next() {
		var timestamp time.Time
		var protocol sql.NullInt64
		var octets int64

		err := rows.Scan(&timestamp, &protocol, &octets)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		if !protocol.Valid {
			continue
		}

		if _, exists := timeSeriesMap[timestamp]; !exists {
			timeSeriesMap[timestamp] = make(map[int]int64)
		}

		timeSeriesMap[timestamp][int(protocol.Int64)] = octets
	}

	// Convert map to slice
	var timeSeries []ProtocolTimeSeriesPoint
	for ts, data := range timeSeriesMap {
		timeSeries = append(timeSeries, ProtocolTimeSeriesPoint{
			Timestamp:    ts,
			ProtocolData: data,
		})
	}

	// Sort by timestamp
	// Note: Using a simple bubble sort for small datasets
	for i := 0; i < len(timeSeries)-1; i++ {
		for j := 0; j < len(timeSeries)-i-1; j++ {
			if timeSeries[j].Timestamp.After(timeSeries[j+1].Timestamp) {
				timeSeries[j], timeSeries[j+1] = timeSeries[j+1], timeSeries[j]
			}
		}
	}

	return timeSeries, nil
}

// getProtocolAnalysisRequest handles HTTP request for protocol analysis
func getProtocolAnalysisRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	filter := parseProtocolFilter(r)
	includeTimeSeries := r.URL.Query().Get("include_timeseries") == "true"
	includeTopPorts := r.URL.Query().Get("include_ports") == "true"
	portLimit := 20

	if limitStr := r.URL.Query().Get("port_limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			portLimit = l
		}
	}

	// Log the filter for debugging
	log.Printf("Protocol analysis - Exporter: %s, Interface: %s, Direction: %s, Start: %s, End: %s",
		filter.Exporter, filter.Interface, filter.Direction, filter.StartTime, filter.EndTime)

	// Validate required fields
	if filter.Exporter == "" {
		http.Error(w, `{"error": "exporter parameter is required"}`, http.StatusBadRequest)
		return
	}
	if filter.Interface == "" {
		http.Error(w, `{"error": "interface parameter is required"}`, http.StatusBadRequest)
		return
	}

	// Get protocol statistics
	protocolStats, err := getProtocolStats(filter)
	if err != nil {
		log.Printf("Error getting protocol stats: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	response := &ProtocolAnalysisResponse{
		ProtocolStats: protocolStats,
	}

	// Calculate totals
	for _, stat := range protocolStats {
		response.TotalOctets += stat.TotalOctets
		response.TotalPackets += stat.TotalPackets
		response.TotalFlows += stat.FlowCount
	}

	// Get top ports if requested
	if includeTopPorts {
		topPorts, err := getProtocolPortStats(filter, portLimit)
		if err != nil {
			log.Printf("Error getting protocol port stats: %v", err)
		} else {
			response.TopPorts = topPorts
		}
	}

	// Get time series if requested
	if includeTimeSeries {
		timeSeries, err := getProtocolTimeSeries(filter)
		if err != nil {
			log.Printf("Error getting protocol time series: %v", err)
		} else {
			response.TimeSeries = timeSeries
		}
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}
