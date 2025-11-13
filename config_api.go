package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// updateExporterRequest handles POST requests to update exporter configuration
func updateExporterRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var exporter ExporterConfig
	err := json.NewDecoder(r.Body).Decode(&exporter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Marshal data to JSON
	dataJSON, err := json.Marshal(exporter.Data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error marshaling data: %v", err), http.StatusBadRequest)
		return
	}

	query := `
		UPDATE exporters
		SET name = $1,
		    snmp_version = $2,
		    snmp_community = $3,
		    snmpv3_username = $4,
		    snmpv3_level = $5,
		    snmpv3_auth_proto = $6,
		    snmpv3_auth_pass = $7,
		    snmpv3_priv_proto = $8,
		    snmpv3_priv_pass = $9,
		    data = $10
		WHERE id = $11
	`

	_, err = config.Db.Exec(query,
		exporter.Name,
		exporter.SnmpVersion,
		exporter.SnmpCommunity,
		exporter.Snmpv3Username,
		exporter.Snmpv3Level,
		exporter.Snmpv3AuthProto,
		exporter.Snmpv3AuthPass,
		exporter.Snmpv3PrivProto,
		exporter.Snmpv3PrivPass,
		dataJSON,
		exporter.ID,
	)

	if err != nil {
		log.Printf("Error updating exporter: %v", err)
		http.Error(w, fmt.Sprintf("Error updating exporter: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Exporter updated successfully",
	})
}

// getExportersConfigRequest returns detailed exporter configuration
func getExportersConfigRequest(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT
			id, ip_inet, name, snmp_version, snmp_community,
			snmpv3_username, snmpv3_level, snmpv3_auth_proto,
			snmpv3_priv_proto, data
		FROM exporters
		ORDER BY id
	`

	rows, err := config.Db.Query(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error querying exporters: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var exporters []ExporterConfig
	for rows.Next() {
		var exp ExporterConfig
		var dataJSON []byte
		err := rows.Scan(
			&exp.ID,
			&exp.IPInet,
			&exp.Name,
			&exp.SnmpVersion,
			&exp.SnmpCommunity,
			&exp.Snmpv3Username,
			&exp.Snmpv3Level,
			&exp.Snmpv3AuthProto,
			&exp.Snmpv3PrivProto,
			&dataJSON,
		)
		if err != nil {
			log.Printf("Error scanning exporter: %v", err)
			continue
		}

		// Parse JSON data
		if len(dataJSON) > 0 {
			json.Unmarshal(dataJSON, &exp.Data)
		}

		// Don't send passwords to frontend
		exp.Snmpv3AuthPass = ""
		exp.Snmpv3PrivPass = ""
		exporters = append(exporters, exp)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(exporters)
}

// getInterfacesConfigRequest returns interfaces with their enabled status
func getInterfacesConfigRequest(w http.ResponseWriter, r *http.Request) {
	exporterID := r.URL.Query().Get("exporter")

	var query string
	var rows *sql.Rows
	var err error

	if exporterID != "" {
		query = `
			SELECT id, exporter, snmp_index, name, description, alias, speed, enabled, bandwidth
			FROM interfaces
			WHERE exporter = $1
			ORDER BY snmp_index
		`
		rows, err = config.Db.Query(query, exporterID)
	} else {
		query = `
			SELECT id, exporter, snmp_index, name, description, alias, speed, enabled, bandwidth
			FROM interfaces
			ORDER BY exporter, snmp_index
		`
		rows, err = config.Db.Query(query)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Error querying interfaces: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var interfaces []InterfaceConfig
	for rows.Next() {
		var iface InterfaceConfig
		err := rows.Scan(
			&iface.ID,
			&iface.Exporter,
			&iface.SnmpIndex,
			&iface.Name,
			&iface.Description,
			&iface.Alias,
			&iface.Speed,
			&iface.Enabled,
			&iface.Bandwidth,
		)
		if err != nil {
			log.Printf("Error scanning interface: %v", err)
			continue
		}
		interfaces = append(interfaces, iface)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(interfaces)
}

// updateInterfaceRequest handles POST requests to update interface configuration
func updateInterfaceRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var iface InterfaceConfig
	err := json.NewDecoder(r.Body).Decode(&iface)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	query := `
		UPDATE interfaces
		SET enabled = $1,
		    alias = $2,
		    bandwidth = $3
		WHERE id = $4
	`

	_, err = config.Db.Exec(query, iface.Enabled, iface.Alias, iface.Bandwidth, iface.ID)
	if err != nil {
		log.Printf("Error updating interface: %v", err)
		http.Error(w, fmt.Sprintf("Error updating interface: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Interface updated successfully",
	})
}

// bulkUpdateInterfacesRequest handles bulk enable/disable operations
func bulkUpdateInterfacesRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ExporterID string `json:"exporter_id"`
		Enabled    bool   `json:"enabled"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	exporterIDInt, err := strconv.ParseInt(req.ExporterID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid exporter ID", http.StatusBadRequest)
		return
	}

	query := `UPDATE interfaces SET enabled = $1 WHERE exporter = $2`
	result, err := config.Db.Exec(query, req.Enabled, exporterIDInt)
	if err != nil {
		log.Printf("Error bulk updating interfaces: %v", err)
		http.Error(w, fmt.Sprintf("Error updating interfaces: %v", err), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"message":       fmt.Sprintf("Updated %d interfaces", rowsAffected),
		"rows_affected": rowsAffected,
	})
}
