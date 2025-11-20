package main

import (
	"database/sql"
	"math"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const (
	// Earth's radius in various units
	EarthRadiusKM = 6371.0088
	//EarthRadiusMiles  = 3958.7613
	//EarthRadiusMeters = 6371008.8

	// Conversion factors
	RadiansPerDegree = math.Pi / 180.0
	//DegreesPerRadian = 180.0 / math.Pi
)

type Metric struct {
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
	OctetsIn  int64     `json:"octets_in" db:"octets_in"`
	OctetsOut int64     `json:"octets_out" db:"octets_out"`
}

type Config struct {
	Bind_address     string
	Conn_string      string
	Maxmind_database string
	Db               *sql.DB
	Dbrest           string
	TZ               string
	Mmdb             *maxminddb.Reader
}

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

type FlowGEO struct {
	SrcAddr  string
	DstAddr  string
	SrcCoord Coordinates
	DstCoord Coordinates
	Distance float64
	Octets   int64
	Packets  int64
}

type ReturnStruct struct {
	Fdb  []FlowGEO
	Last string
}

type FlowDB struct {
	ID         uint64    `json:"id" db:"id"`
	InsertedAt time.Time `json:"inserted_at" db:"inserted_at"`
	Exporter   string    `json:"exporter" db:"exporter"`
	SrcAddr    string    `json:"srcaddr" db:"srcaddr"`
	DstAddr    string    `json:"dstaddr" db:"dstaddr"`
	Input      int64     `json:"input" db:"input"`
	Output     int64     `json:"output" db:"output"`
	DPkts      int64     `json:"dpkts" db:"dpkts"`
	DOctets    int64     `json:"doctets" db:"doctets"`
	SrcPort    int64     `json:"srcport" db:"srcport"`
	DstPort    int64     `json:"dstport" db:"dstport"`
	TCPFlags   string    `json:"tcp_flags" db:"tcp_flags"`
	Protocol   string    `json:"prot" db:"prot"`
	TOS        string    `json:"tos" db:"tos"`
	SrcAS      int64     `json:"src_as" db:"src_as"`
	DstAS      int64     `json:"dst_as" db:"dst_as"`
	SrcMask    string    `json:"src_mask" db:"src_mask"`
	DstMask    string    `json:"dst_mask" db:"dst_mask"`
	IPVersion  string    `json:"ip_version" db:"ip_version"`
	First      time.Time `json:"first" db:"first"`
	Last       time.Time `json:"last" db:"last"`
	FlowHash   string    `json:"flow_hash" db:"flow_hash"`
}

type Exporter struct {
	ID      uint64 `json:"id" db:"id"`
	IP_Bin  int64  `json:"ip_bin" db:"ip_bin"`
	IP_Inet string `json:"ip_inet" db:"ip_inet"`
	Name    string `json:"name" db:"name"`
}

type Interface struct {
	ID       uint64 `json:"id" db:"id"`
	Exporter string `json:"exporter" db:"exporter"`
	Snmp_if  uint64 `json:"snmp_if" db:"snmp_if"`
	Name     string `json:"name" db:"name"`
	Descr    string `json:"description" db:"description"`
	Alias    string `json:"alias" db:"alias"`
	Speed    int64  `json:"speed" db:"speed"`
	Enabled  bool   `json:"enabled" db:"enabled"`
}

// ServiceNetwork represents a network (CIDR) mapped to a name from the services table
// Minimal struct for API/JSON output when needed
type ServiceNetwork struct {
	CIDR string `json:"cidr"`
	Name string `json:"name"`
}

// PostgresMetrics represents database metrics
type PostgresMetrics struct {
	DatabaseStats    []DatabaseStat  `json:"database_stats"`
	TableStats       []TableStat     `json:"table_stats"`
	ActivityStats    ActivityStat    `json:"activity_stats"`
	TransactionStats TransactionStat `json:"transaction_stats"`
	ConnectionStats  ConnectionStat  `json:"connection_stats"`
}

type DatabaseStat struct {
	DatabaseName  string `json:"database_name"`
	Size          int64  `json:"size"`
	SizeFormatted string `json:"size_formatted"`
	NumBackends   int    `json:"num_backends"`
}

type TableStat struct {
	SchemaName    string `json:"schema_name"`
	TableName     string `json:"table_name"`
	TotalSize     int64  `json:"total_size"`
	TableSize     int64  `json:"table_size"`
	IndexesSize   int64  `json:"indexes_size"`
	SizeFormatted string `json:"size_formatted"`
	RowEstimate   int64  `json:"row_estimate"`
}

type ActivityStat struct {
	ActiveConnections int     `json:"active_connections"`
	IdleConnections   int     `json:"idle_connections"`
	TPS               float64 `json:"transactions_per_second"`
	Commits           int64   `json:"commits"`
	Rollbacks         int64   `json:"rollbacks"`
}

type TransactionStat struct {
	Commits   int64   `json:"commits"`
	Rollbacks int64   `json:"rollbacks"`
	TPS       float64 `json:"tps"`
}

type ConnectionStat struct {
	Total          int `json:"total"`
	Active         int `json:"active"`
	Idle           int `json:"idle"`
	MaxConnections int `json:"max_connections"`
}

// ExporterConfig represents exporter configuration for the config page
type ExporterConfig struct {
	ID              uint64                 `json:"id"`
	IPInet          string                 `json:"ip_inet"`
	Name            string                 `json:"name"`
	SnmpVersion     int                    `json:"snmp_version"`
	SnmpCommunity   string                 `json:"snmp_community"`
	Snmpv3Username  string                 `json:"snmpv3_username"`
	Snmpv3Level     string                 `json:"snmpv3_level"`
	Snmpv3AuthProto string                 `json:"snmpv3_auth_proto"`
	Snmpv3AuthPass  string                 `json:"snmpv3_auth_pass"`
	Snmpv3PrivProto string                 `json:"snmpv3_priv_proto"`
	Snmpv3PrivPass  string                 `json:"snmpv3_priv_pass"`
	Data            map[string]interface{} `json:"data"`
}

// InterfaceConfig represents interface configuration for the config page
type InterfaceConfig struct {
	ID          uint64 `json:"id"`
	Exporter    int64  `json:"exporter"`
	SnmpIndex   int64  `json:"snmp_index"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Alias       string `json:"alias"`
	Speed       int64  `json:"speed"`
	Enabled     bool   `json:"enabled"`
	Bandwidth   int64  `json:"bandwidth"`
}
