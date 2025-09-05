chart = Highcharts.chart({{.Container}}, {
    title: {
        text: 'Traffic Rate Exporter',
        align: 'left'
    },
    subtitle: {
        text: 'Interface Metrics',
        align: 'left'
    },
    yAxis: {
        title: {
            text: 'bits/s'
        }
    },
    xAxis: {
        type: "category",
        accessibility: {
            rangeDescription: ''
        }
    },
    legend: {
        layout: 'vertical',
        align: 'right',
        verticalAlign: 'middle'
    },
    chart: {
        zooming: {
            type: "x"
        },
    },
    series: [],
    responsive: {
        rules: [{
            condition: {
                maxWidth: 500
            },
            chartOptions: {
                legend: {
                    layout: 'horizontal',
                    align: 'center',
                    verticalAlign: 'bottom'
                }
            }
        }]
    }
});
timeout = setTimeout(function(){
    $.getJSON('{{.PostgrestUrl}}interface_metrics?exporter=eq.{{.Exporter}}&snmp_index=eq.{{.Interface}}&inserted_at=gt.{{.StartInputValue}} {{.TZ}}&inserted_at=lt.{{.EndInputValue}} {{.TZ}}&order=inserted_at.asc', function(data) {
        let seriesData_in = {"type": "line","name" : "bits/s in", "data" : []};
        let seriesData_out = {"type": "line","name" : "bits/s out", "data" : []};
        debugger;
        for (let i in data){
            if (i > 0){
                let data_rate_in = 0
                let data_rate_out = 0
                const last = Math.floor(Date.parse(data[i-1].inserted_at) / 1000);
                const now = Math.floor(Date.parse(data[i].inserted_at) / 1000);
                const interval = now - last;
                data_rate_in = 8*(data[i].octets_in - data[i-1].octets_in)/interval;
                data_rate_out = 8*(data[i].octets_out - data[i-1].octets_out)/interval;
                if (data_rate_in < 0) {
                    data_rate_in = 0
                }
                if (data_rate_out < 0) {
                    data_rate_out = 0
                }
                seriesData_in.data.push([
                    data[i].inserted_at,
                    data_rate_in
                ]);
                seriesData_out.data.push([
                    data[i].inserted_at,
                    data_rate_out
                ]);
            }
        }
        chart.addSeries(seriesData_in);
        chart.addSeries(seriesData_out);
    });

},100);
