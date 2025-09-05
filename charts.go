package main

import (
	"database/sql"
	"errors"
	"fmt"

	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

func renderChartTag(w http.ResponseWriter, r *http.Request) {

	exporterStr := r.PathValue("exporter")
	log.Println(exporterStr)
	interfaceStr := r.PathValue("interface")
	log.Println(interfaceStr)
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	startStr := fmt.Sprintf("%d", start.Unix())
	endStr := fmt.Sprintf("%d", end.Unix())
	body := []byte(`<img src="./api/v1/metrics/` + exporterStr + `/` + interfaceStr + `/` + startStr + `/` + endStr + `/png" /></img>`)
	w.Write(body)
}

func FormatIECRate(v interface{}) string {
	return FormatIEC(v, false)
}

func FormatIEC(v interface{}, bytes bool) string {
	var ret string
	var bits float64
	switch v.(type) {
	case int64:
		bits = float64(v.(int64))
	case float64:
		bits = v.(float64)
	}
	var suffix string
	if bytes {
		suffix = "B"
	} else {
		suffix = "b/s"
	}
	if bits == 0 {
		ret = fmt.Sprintf("0 %s", suffix)
		log.Println(ret)
		return ret
	}

	neg := bits < 0
	if neg {
		bits = -bits
	}

	if bits < 1024 {
		if neg {
			if bytes {
				ret = fmt.Sprintf("-%.*f B", 1, bits*8)
			} else {
				ret = fmt.Sprintf("-%.*f b/s", 1, bits)
			}

			log.Println(ret)
			return ret
		}
		if bytes {
			ret = fmt.Sprintf("%.*f B", 1, bits*8)
		}

		log.Println(ret)
		return ret
	}

	units := []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}
	val := float64(bits)
	idx := 0
	for val >= 1024 && idx < len(units)-1 {
		val /= 1024
		idx++
	}

	prec := 1
	if val >= 10 {
		prec = 0
	}
	var num string

	if neg {
		if bytes {
			num = fmt.Sprintf("%.*f", prec, val*8)
			ret = fmt.Sprintf("-%s %s%s", num, units[idx], suffix)
		} else {
			num = fmt.Sprintf("%.*f", prec, val)
			ret = fmt.Sprintf("-%s %s%s", num, units[idx], suffix)
		}

		log.Println(ret)
		return ret
	}
	num = fmt.Sprintf("%.*f", prec, val)
	if bytes {
		ret = fmt.Sprintf("%s %s%s", num, units[idx], suffix)
	} else {
		ret = fmt.Sprintf("%s %s%s", num, units[idx], suffix)
	}

	log.Println(ret)
	return ret
}

// ParseIEC parses a human-readable IEC size back into bytes.
// Accepts units: B, KiB, MiB, GiB, TiB, PiB, EiB (case-insensitive).
// Examples:
//
//	ParseIEC("1.5 KiB") -> 1536, nil
//	ParseIEC("10 GiB")  -> 10737418240, nil
func ParseIEC(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("empty size string")
	}

	trim := strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	parts := strings.Fields(trim)
	if len(parts) == 0 {
		return 0, errors.New("invalid size string")
	}

	// Join back if user wrote like "1.5GiB" without space
	if len(parts) == 1 {
		// split trailing letters
		i := len(parts[0])
		for i > 0 && ((parts[0][i-1] >= 'A' && parts[0][i-1] <= 'Z') || (parts[0][i-1] >= 'a' && parts[0][i-1] <= 'z')) {
			i--
		}
		if i == 0 {
			return 0, errors.New("invalid number")
		}
		parts = []string{parts[0][:i], parts[0][i:]}
	}
	if len(parts) > 2 {
		return 0, errors.New("invalid size format")
	}

	numStr := parts[0]
	unit := "B"
	if len(parts) == 2 {
		unit = parts[1]
	}

	n, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %w", err)
	}

	powers := map[string]int{
		"B":   0,
		"KIB": 1,
		"MIB": 2,
		"GIB": 3,
		"TIB": 4,
		"PIB": 5,
		"EIB": 6,
	}
	pow, ok := powers[strings.ToUpper(unit)]
	if !ok {
		return 0, fmt.Errorf("unknown IEC unit: %q", unit)
	}

	val := n * math.Pow(1024, float64(pow))
	// Round to nearest int64
	if val > math.MaxInt64 || val < math.MinInt64 {
		return 0, errors.New("value out of range")
	}
	return int64(math.Round(val)), nil
}

func getSNMPMetricsForChart(r *http.Request) (metrics []Metric, err error) {

	exporterStr := r.PathValue("exporter")
	log.Println(exporterStr)
	interfaceStr := r.PathValue("interface")
	log.Println(interfaceStr)
	var start time.Time
	var end time.Time
	startStr := r.PathValue("start")
	endStr := r.PathValue("end")
	startEpoch, _ := strconv.ParseInt(startStr, 10, 64)
	endEpoch, _ := strconv.ParseInt(endStr, 10, 64)
	log.Println("start: ", startStr)
	log.Println("end : ", endStr)
	if startStr != "" {
		start = time.Unix(startEpoch, 0)
	}
	if endStr != "" {
		end = time.Unix(endEpoch, 0)
	}
	if start.IsZero() {
		start = time.Now().Add(-24 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now()
	}
	log.Println("start: ", start)
	log.Println("end : ", end)
	metrics, err = getInterfacesMetrics(exporterStr, interfaceStr, start, end)
	return metrics, err
}

func renderTimeseriesChartPNG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	metrics, err := getSNMPMetricsForChart(r)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var x_values []time.Time
	var y_values_in []float64
	var y_values_out []float64
	var old_in float64 = 0
	var new_in float64 = 0
	var old_out float64 = 0
	var new_out float64 = 0
	var delta_in float64 = 0
	var delta_out float64 = 0
	var new_ts time.Time
	var old_ts time.Time
	var delta_ts time.Duration
	for _, metric := range metrics {
		old_ts = new_ts
		new_ts = metric.Timestamp
		delta_ts = new_ts.Sub(old_ts)

		old_in = new_in
		new_in = float64(metric.OctetsIn)
		delta_in = new_in - old_in
		if delta_in == new_in {
			delta_in = 0
		}
		old_out = new_out
		new_out = float64(metric.OctetsOut)
		delta_out = new_out - old_out
		if delta_out == new_out {
			delta_out = 0
		}
		if delta_out < 0 {
			delta_out = 0
		}
		if delta_in < 0 {
			delta_in = 0
		}

		log.Println("Seconds: ", delta_ts.Seconds())
		delta_in = (delta_in / delta_ts.Seconds()) * 8
		delta_out = (delta_out / delta_ts.Seconds()) * 8
		x_values = append(x_values, metric.Timestamp)
		y_values_in = append(y_values_in, delta_in)
		y_values_out = append(y_values_out, delta_out)
		log.Println(delta_in)
		log.Println(delta_out)
	}
	graph := chart.Chart{
		Title:  "Traffic",
		Width:  600,
		Height: 200,
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeMinuteValueFormatter,
		},
		YAxis: chart.YAxis{
			ValueFormatter: FormatIECRate,
		},
		Series: []chart.Series{
			chart.TimeSeries{
				Name: "Octets In",
				Style: chart.Style{
					Hidden:              false,
					Padding:             chart.Box{},
					ClassName:           "",
					StrokeWidth:         0,
					StrokeColor:         drawing.Color{},
					StrokeDashArray:     nil,
					DotColor:            drawing.Color{},
					DotWidth:            0,
					DotWidthProvider:    nil,
					DotColorProvider:    nil,
					FillColor:           drawing.Color{},
					FontSize:            0,
					FontColor:           drawing.Color{},
					Font:                nil,
					TextHorizontalAlign: 0,
					TextVerticalAlign:   0,
					TextWrap:            0,
					TextLineSpacing:     0,
					TextRotationDegrees: 0,
				},
				YAxis:   0,
				XValues: x_values,
				YValues: y_values_in,
			},
			chart.TimeSeries{
				Name: "Octets Out",
				Style: chart.Style{
					Hidden:              false,
					Padding:             chart.Box{},
					ClassName:           "",
					StrokeWidth:         0,
					StrokeColor:         drawing.Color{},
					StrokeDashArray:     nil,
					DotColor:            drawing.Color{},
					DotWidth:            0,
					DotWidthProvider:    nil,
					DotColorProvider:    nil,
					FillColor:           drawing.Color{},
					FontSize:            0,
					FontColor:           drawing.Color{},
					Font:                nil,
					TextHorizontalAlign: 0,
					TextVerticalAlign:   0,
					TextWrap:            0,
					TextLineSpacing:     0,
					TextRotationDegrees: 0,
				},
				YAxis:   0,
				XValues: x_values,
				YValues: y_values_out,
			},
		},
	}
	graph.Render(chart.PNG, w)

}

type FlowData struct {
	Timestamp    time.Time
	SrcAddr      string
	DstAddr      string
	SrcPort      int64
	DstPort      int64
	SrcAs        int64
	DstAs        int64
	Input        int64
	Output       int64
	TotalPackets int64
	TotalOctets  int64
}

func getFlowMetricsForChart(r *http.Request) ([]FlowData, error) {
	var input_or_output string = r.PathValue("direction")
	if input_or_output == "" {
		input_or_output = "input"
	}
	exporterStr := r.PathValue("exporter")
	log.Println(exporterStr)
	interfaceStr := r.PathValue("interface")
	log.Println(interfaceStr)
	var start time.Time
	var end time.Time
	startStr := r.PathValue("start")
	endStr := r.PathValue("end")
	startEpoch, _ := strconv.ParseInt(startStr, 10, 64)
	endEpoch, _ := strconv.ParseInt(endStr, 10, 64)
	log.Println("start: ", startStr)
	log.Println("end : ", endStr)
	if startStr != "" {
		start = time.Unix(startEpoch, 0)
	}
	if endStr != "" {
		end = time.Unix(endEpoch, 0)
	}
	if start.IsZero() {
		start = time.Now().Add(-24 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now()
	}
	log.Println("start: ", start)
	log.Println("end : ", end)
	var flows []FlowData
	exporters, err := getExporterList()
	if err != nil {
		return nil, err
	}
	var exporterInet string
	for _, exporter := range exporters {
		log.Println(exporter.ID)
		log.Println(exporterStr)
		log.Println(exporter.IP_Inet)
		if fmt.Sprintf("%d", exporter.ID) == exporterStr {
			log.Println("Found")
			exporterInet = exporter.IP_Inet
		}
	}
	if exporterInet == "" {
		return nil, errors.New("Exporter not found")
	}
	log.Println(exporterInet)
	/*
		q := fmt.Sprintf("select bucket_5min,exporter,srcaddr,dstaddr,srcport,dstport,src_as,dst_as,total_packets,total_octets,input,output from flows_v5_agg_5min where exporter=$1 and "+
			input_or_output+
			" = $2 and bucket_5min >= $3 and bucket_5min <= $4 "+
			" UNION "+
			"select bucket_5min,exporter,srcaddr,dstaddr,srcport,dstport,src_as,dst_as,total_packets,total_octets,input,output from flows_v9_agg_5min where exporter=$1 and "+
			input_or_output+" = $2 and bucket_5min >= $3 and bucket_5min <= $4",
			exporterInet, interfaceStr, start, end)

	*/
	rows, err := config.Db.Query("select bucket_5min,exporter,srcaddr,dstaddr,srcport,dstport,src_as,dst_as,total_packets,total_octets,input,output from flows_v5_agg_5min where exporter=$1 and "+
		input_or_output+
		" = $2 and bucket_5min >= $3 and bucket_5min <= $4 "+
		" UNION "+
		"select bucket_5min,exporter,srcaddr,dstaddr,srcport,dstport,src_as,dst_as,total_packets,total_octets,input,output from flows_v9_agg_5min where exporter=$1 and "+
		input_or_output+" = $2 and bucket_5min >= $3 and bucket_5min <= $4",
		exporterInet, interfaceStr, start, end)
	if err != nil {
		return nil, err
	}
	var sqltimestamp sql.NullTime
	var sqlexporter sql.NullString
	var sqlsrcaddr sql.NullString
	var sqldstaddr sql.NullString
	var sqlsrcport sql.NullInt64
	var sqldstport sql.NullInt64
	var sqlsrcas sql.NullInt64
	var sqldstas sql.NullInt64
	var sqlinput sql.NullInt64
	var sqloutput sql.NullInt64
	var sqloctets sql.NullInt64
	var sqlpackets sql.NullInt64
	for rows.Next() {
		var flow FlowData
		err := rows.Scan(&sqltimestamp, &sqlexporter, &sqlsrcaddr, &sqldstaddr, &sqlsrcport, &sqldstport, &sqlsrcas, &sqldstas, &sqlpackets, &sqloctets, &sqlinput, &sqloutput)
		if err != nil {
			return nil, err
		}
		flow.Timestamp = sqltimestamp.Time
		flow.SrcAddr = sqlsrcaddr.String
		flow.DstAddr = sqldstaddr.String
		flow.SrcPort = sqlsrcport.Int64
		flow.DstPort = sqldstport.Int64
		flow.SrcAs = sqlsrcas.Int64
		flow.DstAs = sqldstas.Int64
		flow.Input = sqlinput.Int64
		flow.Output = sqloutput.Int64
		flow.TotalPackets = sqlpackets.Int64
		flow.TotalOctets = sqloctets.Int64
		log.Println(flow)
		flows = append(flows, flow)

	}
	return flows, nil
}

func renderPieChartJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

func renderPieChartPNG(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	metrics, err := getFlowMetricsForChart(r)
	octets := make(map[string]int64)
	octets_with_totals := make(map[string]int64)
	src_or_dst := r.PathValue("src_or_dst")
	direction := r.PathValue("direction")
	interfac := r.PathValue("interface")
	bytes_pkts_flows := r.PathValue("bytes_pkts_flows")
	if src_or_dst == "" {
		src_or_dst = "src"
	}
	if bytes_pkts_flows == "" {
		bytes_pkts_flows = "bytes"
	}
	if bytes_pkts_flows == "bytes" {

		if src_or_dst == "src" {
			for _, metric := range metrics {
				if _, ok := octets[metric.SrcAddr]; !ok {
					octets[metric.SrcAddr] = metric.TotalOctets
				}
				octets[metric.SrcAddr] += metric.TotalOctets
			}
		} else if src_or_dst == "dst" {
			for _, metric := range metrics {
				if _, ok := octets[metric.DstAddr]; !ok {
					octets[metric.DstAddr] = metric.TotalOctets
				}
				octets[metric.DstAddr] += metric.TotalOctets
			}
		}
		sort.Slice(metrics, func(i, j int) bool {
			return metrics[i].TotalOctets > metrics[j].TotalOctets
		})
	} else if bytes_pkts_flows == "pkts" {
		if src_or_dst == "src" {
			for _, metric := range metrics {
				if _, ok := octets[metric.SrcAddr]; !ok {
					octets[metric.SrcAddr] = metric.TotalPackets
				}
				octets[metric.SrcAddr] += metric.TotalPackets
			}
		} else if src_or_dst == "dst" {
			for _, metric := range metrics {
				if _, ok := octets[metric.DstAddr]; !ok {
					octets[metric.DstAddr] = metric.TotalPackets
				}
				octets[metric.DstAddr] += metric.TotalPackets
			}
		}
		sort.Slice(metrics, func(i, j int) bool {
			return metrics[i].TotalOctets > metrics[j].TotalPackets
		})
	}

	for addr, metric := range octets {
		key := fmt.Sprintf("%s  %s", addr, FormatIEC(metric, true))
		if _, ok := octets_with_totals[key]; !ok {
			octets_with_totals[key] = metric
		}
		octets_with_totals[key] += metric
	}
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var values []chart.Value
	for addr, totaloctets := range octets_with_totals {
		value := chart.Value{
			Style: chart.Style{
				FontSize: 10,
			},
			Label: addr,
			Value: float64(totaloctets),
		}
		values = append(values, value)
	}
	var title = ""
	if src_or_dst == "src" {
		title = fmt.Sprintf("Source Address %s on interface %s", direction, interfac)
	} else if src_or_dst == "dst" {
		title = fmt.Sprintf("Destination Address %s on interface %s", direction, interfac)
	}
	log.Println(values)

	graph := chart.PieChart{
		Title: title,
		TitleStyle: chart.Style{
			FontSize: 10,
		},
		ColorPalette: nil,
		Width:        1080,
		Height:       1080,
		DPI:          300,
		Background:   chart.Style{},
		Canvas:       chart.Style{},
		SliceStyle:   chart.Style{},
		Font:         nil,
		Values:       values,
		Elements:     nil,
	}
	//log.Println(values)
	err = graph.Render(chart.PNG, w)
	if err != nil {
		return
	}

}
