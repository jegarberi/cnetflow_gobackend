package main

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq" // The underscore is intentional - it's a blank import
	"github.com/oschwald/maxminddb-golang/v2"
	"log"
	"math"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
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

type Config struct {
	bind_address     string
	conn_string      string
	maxmind_database string
	db               *sql.DB
	mmdb             *maxminddb.Reader
}

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

func (p Coordinates) Haversine(p2 Coordinates) float64 {
	lat1 := p.Latitude * RadiansPerDegree
	lon1 := p.Longitude * RadiansPerDegree
	lat2 := p2.Latitude * RadiansPerDegree
	lon2 := p2.Longitude * RadiansPerDegree

	diffLat := lat2 - lat1
	diffLon := lon2 - lon1

	a := math.Pow(math.Sin(diffLat/2), 2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Pow(math.Sin(diffLon/2), 2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKM * c
}

// HaversineMeters returns the distance in meters
func (p Coordinates) HaversineMeters(p2 Coordinates) float64 {
	return p.Haversine(p2) * 1000
}

type FlowGEO struct {
	SrcAddr  net.IP
	DstAddr  net.IP
	SrcCoord Coordinates
	DstCoord Coordinates
	Distance float64
	Octets   int64
	Packets  int64
}

type ReturnStruct struct {
	Fdb  []FlowGEO
	Last int64
}

type FlowDB struct {
	ID       uint64 `json:"id" db:"id"`
	SrcAddr  int64  `json:"srcaddr" db:"srcaddr"`
	DstAddr  int64  `json:"dstaddr" db:"dstaddr"`
	Input    int64  `json:"input" db:"input"`
	Output   int64  `json:"output" db:"output"`
	DPkts    int64  `json:"dpkts" db:"dpkts"`
	DOctets  int64  `json:"doctets" db:"doctets"`
	SrcPort  int64  `json:"srcport" db:"srcport"`
	DstPort  int64  `json:"dstport" db:"dstport"`
	TCPFlags string `json:"tcp_flags" db:"tcp_flags"`
	Protocol string `json:"prot" db:"prot"`
	TOS      string `json:"tos" db:"tos"`
	SrcAS    int64  `json:"src_as" db:"src_as"`
	DstAS    int64  `json:"dst_as" db:"dst_as"`
	SrcMask  string `json:"src_mask" db:"src_mask"`
	DstMask  string `json:"dst_mask" db:"dst_mask"`
	Exporter int64  `json:"exporter" db:"exporter"`
	First    int64  `json:"first" db:"first"`
	Last     int64  `json:"last" db:"last"`
}

var config Config

func int64ToIPv4(intIP int64) net.IP {
	// Create a 4-byte buffer
	ip := make(net.IP, 4)

	// Use binary.BigEndian to encode the least significant 32 bits
	binary.BigEndian.PutUint32(ip, uint32(intIP))

	return ip
}

// TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
func getFlowsDB(last int64) ([]FlowGEO, int64) {
	var flowsGeo map[int64]map[int64]*FlowGEO
	flowsGeo = make(map[int64]map[int64]*FlowGEO)
	var max_last int64
	max_last = 0
	var rows *sql.Rows
	var err error
	last = 0
	rows, err = config.db.Query("select * from flows_v9 where last >= $1", last)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err.Error())
		}
	}(rows)
	for rows.Next() {
		//var row Row
		var flow FlowDB
		err := rows.Scan(
			&flow.ID,
			&flow.SrcAddr,
			&flow.DstAddr,
			&flow.Input,
			&flow.Output,
			&flow.DPkts,
			&flow.DOctets,
			&flow.SrcPort,
			&flow.DstPort,
			&flow.TCPFlags,
			&flow.Protocol,
			&flow.TOS,
			&flow.SrcAS,
			&flow.DstAS,
			&flow.SrcMask,
			&flow.DstMask,
			&flow.Exporter,
			&flow.First,
			&flow.Last,
		)

		if flow.SrcAddr != 0 && flow.DstAddr != 0 {
			flow_geo := FlowGEO{}
			if flow.Last > max_last {
				max_last = flow.Last
			}

			if _, exists := flowsGeo[flow.SrcAddr]; !exists {
				flowsGeo[flow.SrcAddr] = make(map[int64]*FlowGEO)
			}
			if _, exists := flowsGeo[flow.SrcAddr][flow.DstAddr]; !exists {
				flowsGeo[flow.SrcAddr][flow.DstAddr] = &FlowGEO{}
				flow_geo.SrcAddr = int64ToIPv4(flow.SrcAddr)
				flow_geo.DstAddr = int64ToIPv4(flow.DstAddr)
				flowsGeo[flow.SrcAddr][flow.DstAddr].DstAddr = flow_geo.DstAddr
				flowsGeo[flow.SrcAddr][flow.DstAddr].SrcAddr = flow_geo.SrcAddr
				flowsGeo[flow.SrcAddr][flow.DstAddr].Packets = flow.DPkts
				flowsGeo[flow.SrcAddr][flow.DstAddr].Octets = flow.DOctets
				var record any
				// Convert to net.IPAddr

				addr, err := netip.ParseAddr(flow_geo.SrcAddr.String())
				if err != nil {
					log.Println(err.Error())
				}
				//log.Println(addr)
				err = config.mmdb.Lookup(addr).Decode(&record)
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Latitude, "location", "latitude")
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Longitude, "location", "longitude")
				//log.Println(record)
				addr, err = netip.ParseAddr(flow_geo.DstAddr.String())
				if err != nil {
					log.Println(err.Error())
				}
				//log.Println(addr)
				err = config.mmdb.Lookup(addr).Decode(&record)
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Latitude, "location", "latitude")
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Longitude, "location", "longitude")
				//log.Println(record)

			} else {
				flowsGeo[flow.SrcAddr][flow.DstAddr].Packets = flowsGeo[flow.SrcAddr][flow.DstAddr].Packets + flow.DPkts
				flowsGeo[flow.SrcAddr][flow.DstAddr].Octets = flowsGeo[flow.SrcAddr][flow.DstAddr].Octets + flow.DOctets
			}
			if flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Latitude == 0 && flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Longitude == 0 {
				flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Latitude = -34.5823511
				flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Longitude = -58.6027697
			}
			if flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Latitude == 0 && flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Longitude == 0 {
				flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Latitude = -34.5823511
				flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Longitude = -58.6027697
			}
			flowsGeo[flow.SrcAddr][flow.DstAddr].Distance = flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.HaversineMeters(flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord)
			//fmt.Println(flowsGeo[flow.SrcAddr][flow.DstAddr].Distance)
		}
		if err != nil {
			log.Println(err.Error())
		}

	}
	var flowsGEOPruned map[Coordinates]map[Coordinates]*FlowGEO
	flowsGEOPruned = make(map[Coordinates]map[Coordinates]*FlowGEO)
	for _, f1 := range flowsGeo {
		for _, f2 := range f1 {
			if f2.Distance == 0 {
				continue
			}
			if _, exists := flowsGEOPruned[f2.SrcCoord]; !exists {
				flowsGEOPruned[f2.SrcCoord] = make(map[Coordinates]*FlowGEO)
			}
			if _, exists := flowsGEOPruned[f2.SrcCoord][f2.DstCoord]; !exists {
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord] = &FlowGEO{}
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord].SrcCoord = f2.SrcCoord
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord].DstCoord = f2.DstCoord
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord].Distance = f2.Distance
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord].Octets = f2.Octets
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord].Packets = f2.Packets
			} else {
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord].Octets = flowsGEOPruned[f2.SrcCoord][f2.DstCoord].Octets + f2.Octets
				flowsGEOPruned[f2.SrcCoord][f2.DstCoord].Packets = flowsGEOPruned[f2.SrcCoord][f2.DstCoord].Packets + f2.Packets
			}
		}
	}
	var FlowsGEOArray []FlowGEO
	FlowsGEOArray = []FlowGEO{}
	for _, f1 := range flowsGEOPruned {
		for _, f2 := range f1 {
			FlowsGEOArray = append(FlowsGEOArray, *f2)
		}
	}
	return FlowsGEOArray, max_last
}
func getFlows(w http.ResponseWriter, r *http.Request) {
	rs := ReturnStruct{}
	w.Header().Set("Content-Type", "application/json")
	lastStr := r.PathValue("last")
	last, err := strconv.ParseInt(lastStr, 10, 64)
	if err != nil {
		//log.Println(err.Error())
		last = 0
	}
	rs.Fdb, last = getFlowsDB(last)

	//rs.Last = time.Now().Unix()
	rs.Last = last
	jsonBytes, err := json.Marshal(rs)
	if err != nil {
		log.Fatal(err.Error())
	}
	n, err := fmt.Fprintf(w, string(jsonBytes))
	if err != nil {
		log.Println(err.Error())
	} else {
		log.Printf("Wrote %d bytes...\n", n)
	}
}
func main() {
	var err error
	config = Config{}
	//TIP <p>Press <shortcut actionId="ShowIntentionActions"/> when your caret is at the underlined text
	// to see how GoLand suggests fixing the warning.</p><p>Alternatively, if available, click the lightbulb to view possible fixes.</p>
	config.bind_address = os.Getenv("CNETFLOW_GOBACKEND_BIND")
	config.conn_string = os.Getenv("PG_CONN_STRING")
	config.maxmind_database = os.Getenv("MAXMIND_DATABASE")
	if config.maxmind_database == "" {
		log.Println("MAXMIND_DATABASE NOT SET. USING ./GeoLite2-City.mmdb")
		config.maxmind_database = "./GeoLite2-City.mmdb"
	}
	config.mmdb, err = maxminddb.Open(config.maxmind_database)
	if err != nil {
		log.Fatal(err)
	}
	defer func(mmdb *maxminddb.Reader) {
		err := mmdb.Close()
		if err != nil {

		}
	}(config.mmdb)
	config.conn_string += "?sslmode=disable"
	if config.conn_string == "" {
		panic("PG_CONN_STRING not defined...")
	}

	config.db, err = sql.Open("postgres", config.conn_string)

	if err != nil {
		log.Fatal(err.Error())
	}
	if config.bind_address == "" {
		config.bind_address = ":3002"
	}
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir("./static"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/flows", getFlows)
	mux.HandleFunc("/flows/{last}", getFlows)
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	err = http.ListenAndServe(config.bind_address, mux)
	if err != nil {
		log.Fatal(err.Error())
	}
}
