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

// TrafficFilter holds all possible filter parameters
type TrafficFilter struct {
	Exporter   string
	Interface  string
	Direction  string // "input" or "output"
	StartTime  time.Time
	EndTime    time.Time
	SrcAddr    string
	DstAddr    string
	Protocol   string
	SrcPort    int
	DstPort    int
	SrcAS      int
	DstAS      int
	MinOctets  int64
	MaxOctets  int64
	MinPackets int64
	MaxPackets int64
	Limit      int
	Offset     int
	OrderBy    string
	OrderDir   string // "asc" or "desc"
}

// TrafficRecord represents a single traffic flow record
type TrafficRecord struct {
	ID         uint64    `json:"id"`
	InsertedAt time.Time `json:"inserted_at"`
	Exporter   string    `json:"exporter"`
	SrcAddr    string    `json:"srcaddr"`
	DstAddr    string    `json:"dstaddr"`
	Input      int64     `json:"input"`
	Output     int64     `json:"output"`
	DPkts      int64     `json:"dpkts"`
	DOctets    int64     `json:"doctets"`
	SrcPort    int64     `json:"srcport"`
	DstPort    int64     `json:"dstport"`
	TCPFlags   string    `json:"tcp_flags"`
	Protocol   string    `json:"prot"`
	TOS        string    `json:"tos"`
	SrcAS      int64     `json:"src_as"`
	DstAS      int64     `json:"dst_as"`
	SrcMask    string    `json:"src_mask"`
	DstMask    string    `json:"dst_mask"`
	IPVersion  string    `json:"ip_version"`
	First      time.Time `json:"first"`
	Last       time.Time `json:"last"`
}

// TrafficAggregated represents aggregated traffic data
type TrafficAggregated struct {
	Address      string        `json:"address"`
	SrcPort      int64         `json:"srcport,omitempty"`
	DstPort      int64         `json:"dstport,omitempty"`
	Protocol     string        `json:"protocol,omitempty"`
	ProtocolName string        `json:"protocol_name,omitempty"`
	TotalOctets  int64         `json:"total_octets"`
	TotalPackets int64         `json:"total_packets"`
	FlowCount    int64         `json:"flow_count"`
	Percentage   float64       `json:"percentage"`
	Enrichment   *IPEnrichment `json:"enrichment,omitempty"`
}

// TrafficResponse contains the response with stats
type TrafficResponse struct {
	Records      []TrafficAggregated `json:"records"`
	TotalRecords int                 `json:"total_records"`
	TotalOctets  int64               `json:"total_octets"`
	TotalPackets int64               `json:"total_packets"`
	UniqueAddrs  int                 `json:"unique_addrs"`
}

// parseTrafficFilter extracts filter parameters from request
func parseTrafficFilter(r *http.Request) TrafficFilter {
	filter := TrafficFilter{
		Direction: "input",
		Limit:     100,
		OrderBy:   "total_octets",
		OrderDir:  "desc",
	}

	// Parse query parameters
	query := r.URL.Query()

	filter.Exporter = query.Get("exporter")
	filter.Interface = query.Get("interface")
	filter.Direction = query.Get("direction")
	filter.SrcAddr = query.Get("srcaddr")
	filter.DstAddr = query.Get("dstaddr")
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

	// Port filters
	if srcPort := query.Get("srcport"); srcPort != "" {
		filter.SrcPort, _ = strconv.Atoi(srcPort)
	}
	if dstPort := query.Get("dstport"); dstPort != "" {
		filter.DstPort, _ = strconv.Atoi(dstPort)
	}

	// AS filters
	if srcAS := query.Get("srcas"); srcAS != "" {
		filter.SrcAS, _ = strconv.Atoi(srcAS)
	}
	if dstAS := query.Get("dstas"); dstAS != "" {
		filter.DstAS, _ = strconv.Atoi(dstAS)
	}

	// Volume filters
	if minOctets := query.Get("min_octets"); minOctets != "" {
		filter.MinOctets, _ = strconv.ParseInt(minOctets, 10, 64)
	}
	if maxOctets := query.Get("max_octets"); maxOctets != "" {
		filter.MaxOctets, _ = strconv.ParseInt(maxOctets, 10, 64)
	}
	if minPackets := query.Get("min_packets"); minPackets != "" {
		filter.MinPackets, _ = strconv.ParseInt(minPackets, 10, 64)
	}
	if maxPackets := query.Get("max_packets"); maxPackets != "" {
		filter.MaxPackets, _ = strconv.ParseInt(maxPackets, 10, 64)
	}

	// Pagination
	if limit := query.Get("limit"); limit != "" {
		filter.Limit, _ = strconv.Atoi(limit)
	}
	if offset := query.Get("offset"); offset != "" {
		filter.Offset, _ = strconv.Atoi(offset)
	}

	// Sorting
	if orderBy := query.Get("order_by"); orderBy != "" {
		filter.OrderBy = orderBy
	}
	if orderDir := query.Get("order_dir"); orderDir != "" {
		filter.OrderDir = orderDir
	}

	// Ensure defaults
	if filter.Direction == "" {
		filter.Direction = "input"
	}

	return filter
}

// buildTrafficQuery constructs SQL query based on filters
func buildTrafficQuery(filter TrafficFilter, groupBy string) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// Base query - use flows_agg_5min for aggregated data
	selectFields := ""
	if groupBy == "address" {
		addrField := "srcaddr"
		if filter.Direction == "output" {
			addrField = "dstaddr"
		}
		selectFields = fmt.Sprintf(`
			%s as address,
			SUM(total_octets) as total_octets,
			SUM(total_packets) as total_packets,
			COUNT(*) as flow_count
		`, addrField)
	} else if groupBy == "port" {
		addrField := "srcaddr"
		selectFields = fmt.Sprintf(`
			%s as address,
			srcport,
			dstport,
			prot as protocol,
			SUM(total_octets) as total_octets,
			SUM(total_packets) as total_packets,
			COUNT(*) as flow_count
		`, addrField)
	}

	baseQuery := fmt.Sprintf(`
		SELECT %s
		FROM flows_agg_5min
		WHERE 1=1
	`, selectFields)

	// Add filters
	if filter.Exporter != "" {
		conditions = append(conditions, fmt.Sprintf("exporter = $%d", argIndex))
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

	if filter.SrcAddr != "" {
		conditions = append(conditions, fmt.Sprintf("srcaddr = $%d", argIndex))
		args = append(args, filter.SrcAddr)
		argIndex++
	}

	if filter.DstAddr != "" {
		conditions = append(conditions, fmt.Sprintf("dstaddr = $%d", argIndex))
		args = append(args, filter.DstAddr)
		argIndex++
	}

	if filter.Protocol != "" {
		conditions = append(conditions, fmt.Sprintf("prot = $%d", argIndex))
		args = append(args, filter.Protocol)
		argIndex++
	}

	if filter.SrcPort > 0 {
		conditions = append(conditions, fmt.Sprintf("srcport = $%d", argIndex))
		args = append(args, filter.SrcPort)
		argIndex++
	}

	if filter.DstPort > 0 {
		conditions = append(conditions, fmt.Sprintf("dstport = $%d", argIndex))
		args = append(args, filter.DstPort)
		argIndex++
	}

	if filter.SrcAS > 0 {
		conditions = append(conditions, fmt.Sprintf("src_as = $%d", argIndex))
		args = append(args, filter.SrcAS)
		argIndex++
	}

	if filter.DstAS > 0 {
		conditions = append(conditions, fmt.Sprintf("dst_as = $%d", argIndex))
		args = append(args, filter.DstAS)
		argIndex++
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	// GROUP BY clause
	groupByClause := ""
	if groupBy == "address" {
		if filter.Direction == "output" {
			groupByClause = " GROUP BY dstaddr"
		} else {
			groupByClause = " GROUP BY srcaddr"
		}
	} else if groupBy == "port" {
		groupByClause = " GROUP BY srcaddr, srcport, dstport, prot"
	}

	// Having clause for volume filters
	havingClauses := []string{}
	if filter.MinOctets > 0 {
		havingClauses = append(havingClauses, fmt.Sprintf("SUM(total_octets) >= %d", filter.MinOctets))
	}
	if filter.MaxOctets > 0 {
		havingClauses = append(havingClauses, fmt.Sprintf("SUM(total_octets) <= %d", filter.MaxOctets))
	}
	if filter.MinPackets > 0 {
		havingClauses = append(havingClauses, fmt.Sprintf("SUM(total_packets) >= %d", filter.MinPackets))
	}
	if filter.MaxPackets > 0 {
		havingClauses = append(havingClauses, fmt.Sprintf("SUM(total_packets) <= %d", filter.MaxPackets))
	}

	havingClause := ""
	if len(havingClauses) > 0 {
		havingClause = " HAVING " + strings.Join(havingClauses, " AND ")
	}

	// ORDER BY clause
	orderByClause := fmt.Sprintf(" ORDER BY %s %s", filter.OrderBy, filter.OrderDir)

	// LIMIT and OFFSET
	limitClause := ""
	if filter.Limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		limitClause += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	query := baseQuery + whereClause + groupByClause + havingClause + orderByClause + limitClause

	return query, args
}

// getTrafficDataAggregated retrieves aggregated traffic data based on filters
func getTrafficDataAggregated(filter TrafficFilter, groupBy string) (*TrafficResponse, error) {
	query, args := buildTrafficQuery(filter, groupBy)

	log.Println("Query:", query)
	log.Println("Args:", args)

	rows, err := config.Db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var records []TrafficAggregated
	var totalOctets int64
	var totalPackets int64

	for rows.Next() {
		var record TrafficAggregated

		if groupBy == "address" {
			err := rows.Scan(
				&record.Address,
				&record.TotalOctets,
				&record.TotalPackets,
				&record.FlowCount,
			)
			if err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}
		} else if groupBy == "port" {
			var protocol sql.NullString
			err := rows.Scan(
				&record.Address,
				&record.SrcPort,
				&record.DstPort,
				&protocol,
				&record.TotalOctets,
				&record.TotalPackets,
				&record.FlowCount,
			)
			if err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}
			if protocol.Valid {
				record.Protocol = protocol.String
			}
		}

		totalOctets += record.TotalOctets
		totalPackets += record.TotalPackets
		records = append(records, record)
	}

	// Calculate percentages and enrich IPs
	uniqueIPs := make([]string, 0, len(records))
	uniqueAddrs := make(map[string]bool)

	for i := range records {
		if totalOctets > 0 {
			records[i].Percentage = float64(records[i].TotalOctets) / float64(totalOctets) * 100
		}

		// Add protocol name if we have protocol
		if records[i].Protocol != "" {
			if protocolNum, err := strconv.Atoi(records[i].Protocol); err == nil {
				records[i].ProtocolName = getProtocolNameFromNumber(protocolNum)
			}
		}

		// Collect unique IPs for enrichment
		if _, exists := uniqueAddrs[records[i].Address]; !exists {
			uniqueAddrs[records[i].Address] = true
			uniqueIPs = append(uniqueIPs, records[i].Address)
		}
	}

	// Enrich all IPs concurrently
	enrichments := EnrichIPs(uniqueIPs)

	// Add enrichment data to records
	for i := range records {
		if enrichment, exists := enrichments[records[i].Address]; exists {
			records[i].Enrichment = enrichment
		}
	}

	response := &TrafficResponse{
		Records:      records,
		TotalRecords: len(records),
		TotalOctets:  totalOctets,
		TotalPackets: totalPackets,
		UniqueAddrs:  len(uniqueAddrs),
	}

	return response, nil
}

// getTrafficRequest handles HTTP request for traffic data
func getTrafficRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	filter := parseTrafficFilter(r)
	groupBy := r.URL.Query().Get("group_by")
	if groupBy == "" {
		groupBy = "address"
	}

	// Log the filter for debugging
	log.Printf("Traffic filter - Exporter: %s, Interface: %s, Direction: %s, Start: %s, End: %s",
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

	response, err := getTrafficDataAggregated(filter, groupBy)
	if err != nil {
		log.Printf("Error getting traffic data: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}

// getDataRangeRequest returns the time range of available data for an exporter
func getDataRangeRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	exporter := r.URL.Query().Get("exporter")
	if exporter == "" {
		http.Error(w, `{"error": "exporter parameter is required"}`, http.StatusBadRequest)
		return
	}

	var minTime, maxTime time.Time
	query := "SELECT MIN(bucket_5min), MAX(bucket_5min) FROM flows_agg_5min WHERE exporter = $1::inet"
	err := config.Db.QueryRow(query, exporter).Scan(&minTime, &maxTime)
	if err != nil {
		log.Printf("Error querying data range: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"min_time": minTime,
		"max_time": maxTime,
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}

// getRawFlowsRequest returns raw flow records (not aggregated)
func getRawFlowsRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	filter := parseTrafficFilter(r)

	// Build query for raw flows table
	var conditions []string
	var args []interface{}
	argIndex := 1

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
		conditions = append(conditions, fmt.Sprintf("last AT TIME ZONE 'UTC' >= $%d", argIndex))
		args = append(args, filter.StartTime)
		argIndex++
	}

	if !filter.EndTime.IsZero() {
		conditions = append(conditions, fmt.Sprintf("last AT TIME ZONE 'UTC' <= $%d", argIndex))
		args = append(args, filter.EndTime)
		argIndex++
	}

	if filter.SrcAddr != "" {
		conditions = append(conditions, fmt.Sprintf("srcaddr = $%d", argIndex))
		args = append(args, filter.SrcAddr)
		argIndex++
	}

	if filter.DstAddr != "" {
		conditions = append(conditions, fmt.Sprintf("dstaddr = $%d", argIndex))
		args = append(args, filter.DstAddr)
		argIndex++
	}

	if filter.Protocol != "" {
		conditions = append(conditions, fmt.Sprintf("prot = $%d", argIndex))
		args = append(args, filter.Protocol)
		argIndex++
	}

	if filter.SrcPort > 0 {
		conditions = append(conditions, fmt.Sprintf("srcport = $%d", argIndex))
		args = append(args, filter.SrcPort)
		argIndex++
	}

	if filter.DstPort > 0 {
		conditions = append(conditions, fmt.Sprintf("dstport = $%d", argIndex))
		args = append(args, filter.DstPort)
		argIndex++
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")
	if len(conditions) == 0 {
		whereClause = ""
	}

	query := fmt.Sprintf(`
		SELECT
			id, inserted_at, exporter, srcaddr, dstaddr, input, output,
			dpkts, doctets, srcport, dstport, tcp_flags, prot, tos,
			src_as, dst_as, src_mask, dst_mask, ip_version, first, last
		FROM flows
		%s
		ORDER BY last DESC
		LIMIT $%d
	`, whereClause, argIndex)

	args = append(args, filter.Limit)

	log.Println("Raw flows query:", query)
	log.Println("Args:", args)

	rows, err := config.Db.Query(query, args...)
	if err != nil {
		log.Printf("Query error: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var records []TrafficRecord
	for rows.Next() {
		var record TrafficRecord
		err := rows.Scan(
			&record.ID,
			&record.InsertedAt,
			&record.Exporter,
			&record.SrcAddr,
			&record.DstAddr,
			&record.Input,
			&record.Output,
			&record.DPkts,
			&record.DOctets,
			&record.SrcPort,
			&record.DstPort,
			&record.TCPFlags,
			&record.Protocol,
			&record.TOS,
			&record.SrcAS,
			&record.DstAS,
			&record.SrcMask,
			&record.DstMask,
			&record.IPVersion,
			&record.First,
			&record.Last,
		)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		records = append(records, record)
	}

	response := map[string]interface{}{
		"records": records,
		"count":   len(records),
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, `{"error": "failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Write(jsonBytes)
}
