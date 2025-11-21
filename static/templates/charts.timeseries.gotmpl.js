chart = Highcharts.chart('{{.Container}}', {
    title: { text: 'Traffic Rate Exporter', align: 'left' },
    subtitle: { text: 'Interface Metrics', align: 'left' },
    yAxis: { title: { text: 'bits/s' } },
    xAxis: { type: 'datetime' },
    legend: { layout: 'vertical', align: 'right', verticalAlign: 'middle' },
    chart: { zooming: { type: 'x' } },
    series: [],
    responsive: {
        rules: [{
            condition: { maxWidth: 500 },
            chartOptions: { legend: { layout: 'horizontal', align: 'center', verticalAlign: 'bottom' } }
        }]
    }
});

// Fetch interface metrics from Go backend (no PostgREST)
// Uses query params start/end; values are provided by template in RFC3339-like format or epoch.
setTimeout(function(){
    // Use epoch seconds to avoid timezone/format parsing issues on backend
    const params = new URLSearchParams({
        start: '{{.StartUnix}}',
        end: '{{.EndUnix}}'
    });
    const url = `/api/v1/metrics/{{.Exporter}}/{{.Interface}}?${params.toString()}`;

    $.getJSON(url, function(rows) {
        if (!rows || !rows.length) return;
        let seriesData_in = { type: 'line', name: 'bits/s in', data: [] };
        let seriesData_out = { type: 'line', name: 'bits/s out', data: [] };

        for (let i = 1; i < rows.length; i++) {
            const prev = rows[i - 1];
            const cur = rows[i];
            const last = Math.floor(Date.parse(prev.timestamp) / 1000);
            const now = Math.floor(Date.parse(cur.timestamp) / 1000);
            const dt = Math.max(1, now - last);
            let rateIn = Math.floor(8 * (cur.octets_in - prev.octets_in) / dt);
            let rateOut = Math.floor(8 * (cur.octets_out - prev.octets_out) / dt);
            if (!isFinite(rateIn) || rateIn < 0) rateIn = 0;
            if (!isFinite(rateOut) || rateOut < 0) rateOut = 0;
            seriesData_in.data.push([Date.parse(cur.timestamp), rateIn]);
            seriesData_out.data.push([Date.parse(cur.timestamp), rateOut]);
        }

        chart.addSeries(seriesData_in);
        chart.addSeries(seriesData_out);
    });
}, 100);
