package main

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq" // The underscore is intentional - it's a blank import
	"github.com/oschwald/maxminddb-golang/v2"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
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
type FlowGEO struct {
	SrcAddr  net.IP
	DstAddr  net.IP
	SrcCoord Coordinates
	DstCoord Coordinates
	Octets   int64
	Packets  int64
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
func getFlowsDB() map[int64]map[int64]*FlowGEO {
	var flowsGeo map[int64]map[int64]*FlowGEO
	flowsGeo = make(map[int64]map[int64]*FlowGEO)
	rows, err := config.db.Query("select * from flows_v9")
	if err != nil {
		log.Fatal(err.Error())
	}
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
				log.Println(addr)
				err = config.mmdb.Lookup(addr).Decode(&record)
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Latitude, "location", "latitude")
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Longitude, "location", "longitude")
				log.Println(record)
				addr, err = netip.ParseAddr(flow_geo.DstAddr.String())
				if err != nil {
					log.Println(err.Error())
				}
				log.Println(addr)
				err = config.mmdb.Lookup(addr).Decode(&record)
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Latitude, "location", "latitude")
				err = config.mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Longitude, "location", "longitude")
				log.Println(record)

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
		}
		if err != nil {
			log.Println(err.Error())
		}
		fmt.Println(flow)

	}
	return flowsGeo
}
func getFlows(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	flows := getFlowsDB()
	jsonBytes, err := json.Marshal(flows)
	if err != nil {
		log.Fatal(err.Error())
	}
	fmt.Fprintf(w, string(jsonBytes))

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
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	err = http.ListenAndServe(config.bind_address, mux)
	if err != nil {
		log.Fatal(err.Error())
	}
}
