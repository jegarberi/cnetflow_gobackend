package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
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
	body := []byte(`<img hx-trigger="every 3s" hx-get="./api/v1/metrics/` + exporterStr + `/` + interfaceStr + `/tag" src="./api/v1/metrics/` + exporterStr + `/` + interfaceStr + `/png" /></img>`)
	w.Write(body)
}

func FormatIEC(v interface{}) string {
	bytes, _ := v.(float64)
	if bytes == 0 {
		return "0 B"
	}

	neg := bytes < 0
	if neg {
		bytes = -bytes
	}

	if bytes < 1024 {
		if neg {
			return fmt.Sprintf("-%d B/s", bytes)
		}
		return fmt.Sprintf("%d B/s", bytes)
	}

	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	val := float64(bytes)
	idx := 0
	for val >= 1024 && idx < len(units)-1 {
		val /= 1024
		idx++
	}

	prec := 1
	if val >= 10 {
		prec = 0
	}

	num := fmt.Sprintf("%.*f", prec, val)
	if neg {
		return fmt.Sprintf("-%s %s/s", num, units[idx])
	}
	return fmt.Sprintf("%s %s/s", num, units[idx])
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

func renderChartPNG(w http.ResponseWriter, r *http.Request) {

	exporterStr := r.PathValue("exporter")
	log.Println(exporterStr)
	interfaceStr := r.PathValue("interface")
	log.Println(interfaceStr)
	metrics, err := getInterfacesMetrics(exporterStr, interfaceStr)

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
		delta_in = delta_in / delta_ts.Seconds() / 8
		delta_out = delta_out / delta_ts.Seconds() / 8
		x_values = append(x_values, metric.Timestamp)
		y_values_in = append(y_values_in, delta_in)
		y_values_out = append(y_values_out, delta_out)
		log.Println(delta_in)
		log.Println(delta_out)
	}
	graph := chart.Chart{
		Width:  600,
		Height: 200,
		XAxis: chart.XAxis{
			ValueFormatter: chart.TimeMinuteValueFormatter,
		},
		YAxis: chart.YAxis{
			ValueFormatter: FormatIEC,
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
