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
