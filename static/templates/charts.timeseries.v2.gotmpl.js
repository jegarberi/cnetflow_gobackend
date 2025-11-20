// Highcharts Time Series Template v2
// Inputs from template data:
// - Container: DOM id for the chart container
// - Exporter: exporter identifier
// - Interface: SNMP index for interface
// - StartInputValue, EndInputValue: RFC3339-like local time strings (YYYY-MM-DDTHH:mm)
// - PostgrestUrl: base URL for PostgREST
// - TZ: timezone (informational)

(function(){
  const container = '{{.Container}}';
  const url = '{{.PostgrestUrl}}interface_metrics'
    + '?exporter=eq.{{.Exporter}}'
    + '&snmp_index=eq.{{.Interface}}'
    + '&inserted_at=gt.{{.StartInputValue}}'
    + '&inserted_at=lt.{{.EndInputValue}}'
    + '&order=inserted_at.asc';

  if (window.Highcharts) {
    Highcharts.setOptions({
      time: { useUTC: false },
      lang: { numericSymbols: null, thousandsSep: ',' }
    });
  }

  const chart = Highcharts.chart(container, {
    chart: {
      zooming: { type: 'x' },
      animation: false
    },
    title: { text: 'Traffic rate (bits/s)', align: 'left' },
    subtitle: { text: 'Exporter {{.Exporter}}, ifIndex {{.Interface}}', align: 'left' },
    yAxis: { title: { text: 'bits/s' } },
    xAxis: { type: 'datetime' },
    legend: { layout: 'horizontal', verticalAlign: 'top', align: 'right' },
    tooltip: {
      shared: true,
      valueDecimals: 0,
      valueSuffix: ' bps'
    },
    exporting: { enabled: true },
    series: [
      { type: 'line', name: 'bits/s in', data: [], turboThreshold: 0 },
      { type: 'line', name: 'bits/s out', data: [], turboThreshold: 0 }
    ],
    responsive: {
      rules: [{
        condition: { maxWidth: 600 },
        chartOptions: {
          legend: { layout: 'horizontal', align: 'center', verticalAlign: 'bottom' }
        }
      }]
    }
  });

  // Fetch and process data, compute deltas → rate
  // Use setTimeout to batch series updates to avoid blocking UI.
  $.getJSON(url, function(rows){
    if (!rows || !rows.length) return;

    // Offload rate calculation to a Web Worker for large datasets
    const workerCode = `
      self.onmessage = function(e){
        const rows = e.data || [];
        const inPts = [];
        const outPts = [];
        for (let i=1; i<rows.length; i++){
          const prev = rows[i-1];
          const cur = rows[i];
          const last = Math.floor(Date.parse(prev.inserted_at)/1000);
          const now  = Math.floor(Date.parse(cur.inserted_at)/1000);
          const dt = Math.max(1, now - last);
          let rateIn  = Math.floor(8 * (cur.octets_in  - prev.octets_in)  / dt);
          let rateOut = Math.floor(8 * (cur.octets_out - prev.octets_out) / dt);
          if (!isFinite(rateIn)  || rateIn  < 0) rateIn  = 0;
          if (!isFinite(rateOut) || rateOut < 0) rateOut = 0;
          inPts.push([Date.parse(cur.inserted_at), rateIn]);
          outPts.push([Date.parse(cur.inserted_at), rateOut]);
        }
        self.postMessage({ inPts, outPts });
      };
    `;
    const blob = new Blob([workerCode], { type: 'application/javascript' });
    const worker = new Worker(URL.createObjectURL(blob));

    worker.onmessage = function(ev){
      const { inPts, outPts } = ev.data || {};
      // Batch updates
      setTimeout(function(){ chart.series[0].setData(inPts, false); }, 0);
      setTimeout(function(){ chart.series[1].setData(outPts, true); }, 0);
      worker.terminate();
    };
    worker.onerror = function(){
      // Fallback on main thread
      const inPts = []; const outPts = [];
      for (let i=1; i<rows.length; i++){
        const prev = rows[i-1];
        const cur = rows[i];
        const last = Math.floor(Date.parse(prev.inserted_at)/1000);
        const now  = Math.floor(Date.parse(cur.inserted_at)/1000);
        const dt = Math.max(1, now - last);
        let rateIn  = Math.floor(8 * (cur.octets_in  - prev.octets_in)  / dt);
        let rateOut = Math.floor(8 * (cur.octets_out - prev.octets_out) / dt);
        if (!isFinite(rateIn)  || rateIn  < 0) rateIn  = 0;
        if (!isFinite(rateOut) || rateOut < 0) rateOut = 0;
        inPts.push([Date.parse(cur.inserted_at), rateIn]);
        outPts.push([Date.parse(cur.inserted_at), rateOut]);
      }
      setTimeout(function(){ chart.series[0].setData(inPts, false); }, 0);
      setTimeout(function(){ chart.series[1].setData(outPts, true); }, 0);
    };

    worker.postMessage(rows);
  });
})();
