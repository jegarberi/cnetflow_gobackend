chart_{{.Container}} = Highcharts.chart('{{.Container}}', {
    legend: {
        layout: 'vertical',
        verticalAlign: 'middle',
        align: 'right',
        symbolRadius: 2,
        useHTML: true,
        labelFormatter: function() {
            return `<span style="min-width: 250px; display:inline-block; border-bottom: 1px solid #ccc;"><span style="float:left; font-size:14px; font-weight:normal" >${this.name}</span><span style="float:right">${this.y}</span></span>`
        }
    },
    chart: {
        type: 'pie',
        zooming: {
            type: 'xy'
        },
        panning: {
            enabled: true,
            type: 'xy'
        },
        panKey: 'shift'
    },
    title: {
        text: '{{.SrcOrDst}}Addr {{.InputOrOutput}} {{.PktsOrBytes}}'
    },
    tooltip: {
        valueSuffix: ''
    },
    subtitle: {
        text:
            ''
    },
    plotOptions: {
        pie: {
            allowPointSelect: true,
            cursor: 'pointer',
            showInLegend: true,
            dataLabels: [{
                enabled: true,
                distance: 20
            }, {
                enabled: true,
                distance: -40,
                format: '{point.percentage:.1f}%',
                style: {
                    fontSize: '1.2em',
                    textOutline: 'none',
                    opacity: 0.7
                },
                filter: {
                    operator: '>',
                    property: 'percentage',
                    value: 1
                }
            }]
        }
    },
    series: [
    ]
});
timeout_{{.Container}}  = // Main thread: offload sorting and mapping to a Web Worker
    setTimeout(function () {
        $.getJSON('{{.PostgrestUrl}}flows_agg_5min?select={{.SrcOrDst}}addr,{{.PktsOrBytes}}.sum()&exporter=eq.{{.Exporter}}&{{.InputOrOutput}}=eq.{{.Interface}}&bucket_5min=gt.{{.StartInputValue}}&bucket_5min=lt.{{.EndInputValue}}', function (data) {

            // Inline worker script (Blob)
            const workerCode = `
            self.onmessage = function(e){
                const { data, nameKey } = e.data;

                // Sort descending by sum
                data.sort((a, b) => (a.sum < b.sum) ? 1 : (a.sum > b.sum ? -1 : 0));

                // Map to Highcharts series.data format
                const seriesData = data.map(d => ({
                    name: d[nameKey],
                    y: d.sum
                }));

                self.postMessage({ seriesData });
            };
        `;
            const blob = new Blob([workerCode], { type: 'application/javascript' });
            const worker = new Worker(URL.createObjectURL(blob));

            worker.onmessage = function (e) {
                const { seriesData } = e.data;
                const series = {
                    name: "Input {{.SrcOrDst}}Addr {{.PktsOrBytes}}",
                    colorByPoint: true,
                    data: seriesData
                };
                chart_{{.Container}}.addSeries(series);
                worker.terminate();
            };

            worker.onerror = function () {
                // Fallback to main-thread processing on error
                data.sort((a, b) => (a.sum < b.sum) ? 1 : (a.sum > b.sum ? -1 : 0));
                const series = {
                    name: "Input {{.SrcOrDst}}Addr {{.PktsOrBytes}}",
                    colorByPoint: true,
                    data: data.map(d => ({ name: d['{{.SrcOrDst}}addr'], y: d.sum }))
                };
                chart_{{.Container}}.addSeries(series);
            };

            // Post data to worker with the dynamic key for address column
            worker.postMessage({
                data,
                nameKey: '{{.SrcOrDst}}addr'
            });
        });
    }, 100);