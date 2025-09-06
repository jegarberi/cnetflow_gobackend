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
timeout_{{.Container}}  = setTimeout(function(){
    $.getJSON('{{.PostgrestUrl}}flows_v9_agg_5min?select={{.SrcOrDst}}addr,{{.PktsOrBytes}}.sum()&exporter=eq.{{.Exporter}}&{{.InputOrOutput}}=eq.{{.Interface}}&bucket_5min=gt.{{.StartInputValue}}&bucket_5min=lt.{{.EndInputValue}}', function(data) {
        function compare( a, b ) {
            if ( a.sum < b.sum ){
                return 1;
            }else if ( a.sum > b.sum ){
                return -1;
            } else {
                return 0;
            }
        }

        data.sort( compare );
        seriesData = {name: "Input {{.SrcOrDst}}Addr {{.PktsOrBytes}}", colorbypoint: true, data: []};

        for (let i in data){
            let newdata = {name: data[i].{{.SrcOrDst}}addr, y: data[i].sum    };
            seriesData.data.push(newdata);
        }
        chart_{{.Container}}.addSeries(seriesData);

    });

},100);
