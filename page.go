package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	t "text/template"
	"time"
)

type MainPageData struct {
	Exporter          string
	Interface         string
	Start             time.Time
	End               time.Time
	StartUnix         int64
	EndUnix           int64
	StartInputValue   string
	EndInputValue     string
	Metrics           []Metric
	TrafficImg        string
	InputSrcBytesPie  string
	OutputSrcBytesPie string
	InputDstBytesPie  string
	OutputDstBytesPie string
	InputSrcPktsPie   string
	OutputSrcPktsPie  string
	InputDstPktsPie   string
	OutputDstPktsPie  string
	PostgrestUrl      string
	Container         string
}
type highchartsData struct {
	Exporter  string
	Interface string
	Start     string
	End       string
}

func renderTimeseriesChartJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	var data = FillDataFromPath(r)
	data.Container = "chart_snmptraffic"

	tmpl, err := t.ParseFiles("./static/charts.timeseries.gotmpl.js")
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func highcharts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/js")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	exporterStr := r.PathValue("exporter")
	interfaceStr := r.PathValue("interface")
	startStr := r.PathValue("start")
	endStr := r.PathValue("end")
	startEpoch, _ := strconv.ParseInt(startStr, 10, 64)
	//endEpoch, _ := strconv.ParseInt(endStr, 10, 64)
	start := time.Unix(startEpoch, 0)
	tmpl, err := template.ParseFiles("./static/charts.gotmpl.js")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}
	data := highchartsData{
		Exporter:  exporterStr,
		Interface: interfaceStr,
		Start:     start.Format("2006-01-02T15:04"),
		End:       endStr,
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}
}

func FillDataFromPath(r *http.Request) MainPageData {

	exporterStr := r.PathValue("exporter")
	log.Println(exporterStr)
	interfaceStr := r.PathValue("interface")
	log.Println(interfaceStr)
	var start time.Time
	var end time.Time
	startStr := r.PathValue("start")
	endStr := r.PathValue("end")
	log.Println(startStr)
	log.Println(endStr)
	startEpoch, _ := strconv.ParseInt(startStr, 10, 64)
	endEpoch, _ := strconv.ParseInt(endStr, 10, 64)
	log.Println("start: ", startStr)
	log.Println("end : ", endStr)
	if startStr != "" {
		start = time.Unix(startEpoch, 0)
	} else {
		start = time.Now().Add(-1 * time.Hour)
	}
	if endStr != "" {
		end = time.Unix(endEpoch, 0)
	} else {
		end = time.Now()
	}
	if start.IsZero() {
		start = time.Now().Add(-1 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now()
	}
	log.Println("start: ", start)
	log.Println("end : ", end)
	metrics, err := getInterfacesMetrics(exporterStr, interfaceStr, start, end)
	if err != nil {
		log.Println(err.Error())
	}
	var data = MainPageData{
		PostgrestUrl:      config.Dbrest,
		Exporter:          exporterStr,
		Interface:         interfaceStr,
		Start:             start,
		End:               end,
		StartUnix:         start.Unix(),
		EndUnix:           end.Unix(),
		StartInputValue:   start.Format("2006-01-02T15:04"),
		EndInputValue:     end.Format("2006-01-02T15:04"),
		Metrics:           metrics,
		TrafficImg:        fmt.Sprintf("./api/v1/metrics/%s/%s/%d/%d/png", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		InputSrcBytesPie:  fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/src/bytes/input/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		OutputSrcBytesPie: fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/src/bytes/output/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		InputDstBytesPie:  fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/dst/bytes/input/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		OutputDstBytesPie: fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/dst/bytes/output/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		InputSrcPktsPie:   fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/src/pkts/input/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		OutputSrcPktsPie:  fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/src/pkts/output/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		InputDstPktsPie:   fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/dst/pkts/input/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		OutputDstPktsPie:  fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/dst/pkts/output/js", exporterStr, interfaceStr, start.Unix(), end.Unix()),
	}
	return data
}

func mainPageHighcharts(w http.ResponseWriter, r *http.Request) {
	var data = FillDataFromPath(r)
	tmpl, err := template.ParseFiles("./static/body.js.gotmpl.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}
}

func mainPagePNG(w http.ResponseWriter, r *http.Request) {
	var data MainPageData = FillDataFromPath(r)
	tmpl, err := template.ParseFiles("./static/body.png.gotmpl.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}
	log.Println(data.TrafficImg)
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}
}
