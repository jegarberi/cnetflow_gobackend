package main

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"time"

	_ "github.com/lib/pq" // The underscore is intentional - it's a blank import
	"github.com/oschwald/maxminddb-golang/v2"
)

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
func getFlowsDB(exporter int64, last int64) ([]FlowGEO, int64) {
	var flowsGeo map[int64]map[int64]*FlowGEO
	flowsGeo = make(map[int64]map[int64]*FlowGEO)
	var max_last int64
	max_last = 0
	var rows *sql.Rows
	var err error
	last = 0
	rows, err = config.Db.Query("select * from flows_v9 where exporter = $2 and last >= $1 UNION select * from flows_v5 where exporter = $2 and last >= $1 ", last, exporter)
	if err != nil {
		return nil, 0
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
				err = config.Mmdb.Lookup(addr).Decode(&record)
				err = config.Mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Latitude, "location", "latitude")
				err = config.Mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].SrcCoord.Longitude, "location", "longitude")
				//log.Println(record)
				addr, err = netip.ParseAddr(flow_geo.DstAddr.String())
				if err != nil {
					log.Println(err.Error())
				}
				//log.Println(addr)
				err = config.Mmdb.Lookup(addr).Decode(&record)
				err = config.Mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Latitude, "location", "latitude")
				err = config.Mmdb.Lookup(addr).DecodePath(&flowsGeo[flow.SrcAddr][flow.DstAddr].DstCoord.Longitude, "location", "longitude")
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

func getInterfacesMetricsRequest(w http.ResponseWriter, r *http.Request) {

	exporterStr := r.PathValue("exporter")
	log.Println(exporterStr)
	interfaceStr := r.PathValue("interface")
	log.Println(interfaceStr)
	var start time.Time
	var end time.Time
	startStr := r.PathValue("start")
	endStr := r.PathValue("end")
	if startStr != "" {
		start, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr != "" {
		end, _ = time.Parse(time.RFC3339, endStr)
	}
	if start.IsZero() {
		start = time.Now().Add(-24 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now()
	}
	metrics, err := getInterfacesMetrics(exporterStr, interfaceStr, start, end)

	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metrics_json, err := json.Marshal(metrics)
	fmt.Fprintf(w, string(metrics_json))

}

func getInterfacesMetrics(exporter string, interfac string, start time.Time, end time.Time) ([]Metric, error) {
	var rows *sql.Rows
	var err error
	var metrics []Metric
	log.Println("start: ", start)
	log.Println("end : ", end)
	log.Println("start: ", start.String())
	log.Println("end : ", end.String())

	rows, err = config.Db.Query("select inserted_at,octets_in,octets_out from interface_metrics where exporter = $1 and snmp_index = $2 and (inserted_at >= $3 and inserted_at <= $4 )", exporter, interfac, start.String(), end.String())
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err.Error())
		}
	}(rows)
	for rows.Next() {
		//var row Row
		var metric Metric
		var ts sql.NullTime
		var octets_in sql.NullInt64
		var octets_out sql.NullInt64
		err := rows.Scan(
			&ts,
			&octets_in,
			&octets_out)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		if ts.Valid {
			metric.Timestamp = ts.Time
		}
		if octets_out.Valid {
			metric.OctetsOut = octets_out.Int64
		}
		if octets_in.Valid {
			metric.OctetsIn = octets_in.Int64
		}
		metrics = append(metrics, metric)

	}
	//log.Println(metrics)
	sort.Slice(metrics, func(i, j int) bool {
		//return times[i].Before(times[j]) // ascending
		return metrics[i].Timestamp.Before(metrics[j].Timestamp)
	})

	return metrics, nil
}

func getInterfacesRequest(w http.ResponseWriter, r *http.Request) {

	exporterStr := r.PathValue("exporter")
	log.Println(exporterStr)
	interfaces, err := getInterfacesList(exporterStr)
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Snmp_if < interfaces[j].Snmp_if
	})
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, interfac := range interfaces {
		var line = ""
		//line = fmt.Sprintf("<li hx-swap=\"innerHTML\" hx-target=\"#chart\" hx-post=\"./api/v1/metrics/%s/%d/tag\" id=\"%d\">[%d] %s %s %s %d </li>\n", interfac.Exporter, interfac.Snmp_if, interfac.ID, interfac.Snmp_if, interfac.Name, interfac.Descr, interfac.Alias, interfac.Speed)
		line = fmt.Sprintf("<li hx-swap=\"innerHTML\" hx-target=\"#body\" hx-post=\"./api/v1/body/%s/%d\" id=\"%d\">[%d] %s %s %s %d </li>\n", interfac.Exporter, interfac.Snmp_if, interfac.ID, interfac.Snmp_if, interfac.Name, interfac.Descr, interfac.Alias, interfac.Speed)
		fmt.Fprintf(w, line)

	}

}

func handleQueryRequest(w http.ResponseWriter, r *http.Request) {
	value := r.PathValue("path")
	log.Println(value)
	body := fmt.Sprintf("Not implemented\nPath: %s", value)
	http.Error(w, body, http.StatusNotImplemented)
}

func getInterfacesList(exporter string) ([]Interface, error) {
	var interfaces []Interface

	var rows *sql.Rows
	var err error
	if exporter == "" {
		log.Println("No exporter")
		rows, err = config.Db.Query("select id,exporter,snmp_index,description,alias,speed,enabled from interfaces ;")
	} else {
		log.Println("Exporter: " + exporter)
		rows, err = config.Db.Query("select id,exporter,snmp_index,description,alias,speed,enabled from interfaces where exporter = $1 ;", exporter)
	}

	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err.Error())

		}
	}(rows)

	for rows.Next() {
		interfac := Interface{}
		//var row Row
		var id sql.NullInt64
		var exporter sql.NullString
		var snmp_index sql.NullInt64
		var description sql.NullString
		var alias sql.NullString
		var speed sql.NullInt64
		var enabled sql.NullBool
		err := rows.Scan(
			&id,
			&exporter,
			&snmp_index,
			&description,
			&alias,
			&speed,
			&enabled,
		)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		if !id.Valid {
			log.Println("No ID")
			continue
		}
		if !exporter.Valid {
			log.Println("No exporter")
			continue
		}
		if !snmp_index.Valid {
			log.Println("No snmp_index")
			continue
		}
		if !description.Valid {
			log.Println("No description")
			//continue
		}
		if !alias.Valid {
			log.Println("No alias")
			//continue
		}
		if !speed.Valid {
			log.Println("No speed")
			//continue
		}
		if !enabled.Valid {
			log.Println("No enabled")
			//continue
		}
		interfac.ID = uint64(id.Int64)
		interfac.Exporter = exporter.String
		interfac.Snmp_if = uint64(snmp_index.Int64)
		interfac.Descr = description.String
		interfac.Alias = alias.String
		interfac.Speed = speed.Int64
		interfac.Enabled = enabled.Bool

		interfaces = append(interfaces, interfac)
		log.Println(interfac)
		log.Println(interfaces)
	}
	return interfaces, nil
}

func getExporterList() (map[int]Exporter, error) {
	var exporters = make(map[int]Exporter)

	var rows *sql.Rows
	var err error

	rows, err = config.Db.Query("select id,ip_bin,ip_inet,name from exporters ;")
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println(err.Error())

		}
	}(rows)

	for rows.Next() {
		exporter := Exporter{}
		//var row Row

		err := rows.Scan(
			&exporter.ID,
			&exporter.IP_Bin,
			&exporter.IP_Inet,
			&exporter.Name,
		)
		if err != nil {
			log.Println(err.Error())
			return nil, err
		}
		exporters[int(exporter.ID)] = exporter
		log.Println(exporter)
		log.Println(exporters)
	}
	return exporters, nil
}

func getExportersRequest(w http.ResponseWriter, r *http.Request) {

	//w.Header().Set("Content-Type", "application/json")

	exporters, err := getExporterList()
	/*sort.Slice(exporters, func(i, j int) bool {
		return exporters[i].ID < exporters[j].ID
	})*/
	if err != nil {
		log.Println(err.Error())
	}
	log.Println(exporters)
	for _, exporter := range exporters {
		var line = ""
		line = fmt.Sprintf("<li id=\"%d\" hx-post=\"./api/v1/interfaces/%d\" hx-target=\"#interfaces\">%s [%s]</li>\n", exporter.ID, exporter.ID, exporter.Name, exporter.IP_Inet)
		fmt.Fprintf(w, line)
	}
	/*jsonBytes, err := json.Marshal(exporters)
	if err != nil {
		log.Fatal(err.Error())
	}
	n, err := fmt.Fprintf(w, string(jsonBytes))
	//rs.Last = time.Now().Unix()
	if err != nil {
		log.Println(err.Error())
	} else {
		log.Printf("Wrote %d bytes...\n", n)
	}*/
}
func getFlowsRequest(w http.ResponseWriter, r *http.Request) {
	rs := ReturnStruct{}
	w.Header().Set("Content-Type", "application/json")
	lastStr := r.PathValue("last")
	last, err := strconv.ParseInt(lastStr, 10, 64)
	if err != nil {
		//log.Println(err.Error())
		last = 0
	}
	exporterStr := r.PathValue("exporter")
	exporter, err := strconv.ParseInt(exporterStr, 10, 64)
	if err != nil {
		//log.Println(err.Error())
		exporter = 0
	}
	rs.Fdb, last = getFlowsDB(exporter, last)

	//rs.Last = time.Now().Unix()
	rs.Last = last
	jsonBytes, err := json.Marshal(rs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	config = Config{}
	//TIP <p>Press <shortcut actionId="ShowIntentionActions"/> when your caret is at the underlined text
	// to see how GoLand suggests fixing the warning.</p><p>Alternatively, if available, click the lightbulb to view possible fixes.</p>
	config.Bind_address = os.Getenv("CNETFLOW_GOBACKEND_BIND")
	config.Conn_string = os.Getenv("PG_CONN_STRING")
	config.TZ = os.Getenv("TZ")
	config.Maxmind_database = os.Getenv("MAXMIND_DATABASE")
	config.Dbrest = os.Getenv("PGREST_URL")
	if config.Maxmind_database == "" {
		log.Println("MAXMIND_DATABASE NOT SET. USING ./GeoLite2-City.mmdb")
		config.Maxmind_database = "./GeoLite2-City.mmdb"
	}
	config.Mmdb, err = maxminddb.Open(config.Maxmind_database)
	if err != nil {
		log.Fatal(err)
	}
	defer func(mmdb *maxminddb.Reader) {
		err := mmdb.Close()
		if err != nil {

		}
	}(config.Mmdb)
	config.Conn_string += "?sslmode=disable"
	if config.Conn_string == "" {
		panic("PG_CONN_STRING not defined...")
	}

	config.Db, err = sql.Open("postgres", config.Conn_string)

	if err != nil {
		log.Fatal(err.Error())
	}
	if config.Bind_address == "" {
		config.Bind_address = ":3002"
	}
	mux := http.NewServeMux()
	fileServer := http.FileServer(http.Dir("./static"))
	//mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/api/v1/body/{exporter}/{interface}", mainPageHighcharts)
	mux.HandleFunc("/api/v1/body/{exporter}/{interface}/{start}", mainPageHighcharts)
	mux.HandleFunc("/api/v1/body/{exporter}/{interface}/{start}/{end}", mainPageHighcharts)
	mux.HandleFunc("/api/v1/interfaces", getInterfacesRequest)
	mux.HandleFunc("/api/v1/metrics/{exporter}/{interface}", getInterfacesMetricsRequest)
	mux.HandleFunc("/api/v1/metrics/{exporter}/{interface}/tag", renderChartTag)
	mux.HandleFunc("/api/v1/metrics/{exporter}/{interface}/{start}/{end}/png", renderTimeseriesChartPNG)
	mux.HandleFunc("/api/v1/metrics/{exporter}/{interface}/png", renderTimeseriesChartPNG)
	mux.HandleFunc("/api/v1/metrics/{exporter}/{interface}/js", highcharts)
	mux.HandleFunc("/api/v1/flows/{exporter}/{interface}/{start}/{end}/{src_or_dst}/{bytes_packets_flow}/{direction}/png", renderPieChartPNG)
	mux.HandleFunc("/api/v1/metrics/{exporter}/{interface}/{start}/{end}/js", renderTimeseriesChartJS)
	mux.HandleFunc("/api/v1/flows/{exporter}/{interface}/{start}/{end}/{src_or_dst}/{bytes_packets_flow}/{direction}/js", renderPieChartJS)
	mux.HandleFunc("/api/v1/flows/{container}/{exporter}/{interface}/{start}/{end}/{src_or_dst}/{bytes_packets_flow}/{direction}/js", renderPieChartJS)

	mux.HandleFunc("/api/v1/interfaces/{exporter}", getInterfacesRequest)
	mux.HandleFunc("/api/v1/exporters/list", getExportersRequest)
	mux.HandleFunc("/api/v1/flows/{exporter}", getFlowsRequest)
	mux.HandleFunc("/api/v1/flows/{exporter}/{last}", getFlowsRequest)
	mux.HandleFunc("/api/v1/query/{path...}", handleQueryRequest)

	mux.Handle("/", http.StripPrefix("", fileServer))
	err = http.ListenAndServe(config.Bind_address, mux)
	if err != nil {
		log.Fatal(err.Error())
	}
}
