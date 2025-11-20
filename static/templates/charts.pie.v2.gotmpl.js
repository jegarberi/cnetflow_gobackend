// Highcharts Pie Template v2
// Inputs from template data:
// - Container: DOM id for the chart container
// - Exporter, ExporterIp: identifiers
// - Interface: interface id
// - SrcOrDst: 'src' | 'dst'
// - PktsOrBytes: 'bytes' | 'pkts' or column names like total_octets/total_packets
// - InputOrOutput: 'input' | 'output'
// - StartInputValue, EndInputValue
// - PostgrestUrl

(function(){
  const container = '{{.Container}}';
  const addrCol = '{{.SrcOrDst}}addr';
  // Map friendly names to actual PostgREST column for aggregation
  const aggCol = (function(v){
    switch(v){
      case 'bytes': return 'total_octets';
      case 'pkts': return 'total_packets';
      default: return v; // already a column name
    }
  })('{{.PktsOrBytes}}');

  const url = '{{.PostgrestUrl}}flows_agg_5min'
    + '?select=' + addrCol + ',' + aggCol + '.sum()'
    + '&exporter=eq.{{.Exporter}}'
    + '&{{.InputOrOutput}}=eq.{{.Interface}}'
    + '&bucket_5min=gt.{{.StartInputValue}}'
    + '&bucket_5min=lt.{{.EndInputValue}}';

  if (window.Highcharts) {
    Highcharts.setOptions({
      lang: { numericSymbols: null, thousandsSep: ',' }
    });
  }

  const chart = Highcharts.chart(container, {
    chart: { type: 'pie', animation: false },
    title: { text: '{{.InputOrOutput | title}} {{.SrcOrDst | title}} distribution ({{.PktsOrBytes}})', align: 'left' },
    tooltip: { pointFormat: '<b>{point.y}</b> ({point.percentage:.1f}%)' },
    legend: {
      layout: 'vertical', align: 'right', verticalAlign: 'middle',
      useHTML: true,
      labelFormatter: function(){
        return `<span style="min-width: 240px; display:inline-block; border-bottom:1px solid #eee;">
                  <span style="float:left">${this.name}</span>
                  <span style="float:right">${Highcharts.numberFormat(this.y,0)}</span>
                </span>`;
      }
    },
    plotOptions: {
      pie: {
        allowPointSelect: true,
        showInLegend: true,
        dataLabels: [{ enabled: true, distance: 16 }, { enabled: true, distance: -40, format: '{point.percentage:.1f}%', style: { textOutline: 'none' }, filter: { operator: '>', property: 'percentage', value: 1 } }]
      }
    },
    exporting: { enabled: true },
    series: [{ name: '{{.SrcOrDst}} {{.PktsOrBytes}}', colorByPoint: true, data: [] }],
    responsive: { rules: [{ condition: { maxWidth: 600 }, chartOptions: { legend: { layout: 'horizontal', align: 'center', verticalAlign: 'bottom' } } }] }
  });

  $.getJSON(url, function(data){
    if (!data) data = [];

    // Web Worker to sort and map for large datasets
    const workerCode = `
      self.onmessage = function(e){
        const { data, nameKey } = e.data;
        data.sort((a,b) => (a.sum<b.sum)?1:(a.sum>b.sum?-1:0));
        const seriesData = data.map(d => ({ name: d[nameKey], y: d.sum }));
        self.postMessage({ seriesData });
      };
    `;
    const blob = new Blob([workerCode], { type: 'application/javascript' });
    const worker = new Worker(URL.createObjectURL(blob));
    worker.onmessage = function(ev){
      const { seriesData } = ev.data || {};
      setTimeout(function(){ chart.series[0].setData(seriesData, true); }, 0);
      worker.terminate();
    };
    worker.onerror = function(){
      data.sort((a,b) => (a.sum<b.sum)?1:(a.sum>b.sum?-1:0));
      const seriesData = data.map(d => ({ name: d[addrCol], y: d.sum }));
      setTimeout(function(){ chart.series[0].setData(seriesData, true); }, 0);
    };
    worker.postMessage({ data, nameKey: addrCol });
  });
})();
