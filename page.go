package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"
)

type mainPageData struct {
	Exporter        string
	Interface       string
	Start           time.Time
	End             time.Time
	StartInputValue string
	EndInputValue   string
	Metrics         []Metric
	TrafficImg      string
	InputSrcPie     string
	OutputSrcPie    string
	InputDstPie     string
	OutputDstPie    string
}

func mainPage(w http.ResponseWriter, r *http.Request) {
	var data mainPageData
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
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

	data = mainPageData{
		Exporter:        exporterStr,
		Interface:       interfaceStr,
		Start:           start,
		End:             end,
		StartInputValue: start.Format("2006-01-02T15:04"),
		EndInputValue:   end.Format("2006-01-02T15:04"),
		Metrics:         metrics,
		TrafficImg:      fmt.Sprintf("./api/v1/metrics/%s/%s/%d/%d/png", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		InputSrcPie:     fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/src/input/png", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		OutputSrcPie:    fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/src/output/png", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		InputDstPie:     fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/dst/input/png", exporterStr, interfaceStr, start.Unix(), end.Unix()),
		OutputDstPie:    fmt.Sprintf("./api/v1/flows/%s/%s/%d/%d/dst/output/png", exporterStr, interfaceStr, start.Unix(), end.Unix()),
	}
	tmpl, err := template.ParseFiles("./static/body.gotmpl.go.html")
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}
	log.Println(data.TrafficImg)
	err = tmpl.Execute(w, data)
	if err != nil {
		log.Fatalf("Error executing template: %v", err)
	}
}
