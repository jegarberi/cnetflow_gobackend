// Highcharts Sankey Template
// Inputs from template data:
// - Container: DOM id for the chart container
// - Exporter: exporter identifier (ID or IP depending on backend schema)
// - Interface: SNMP index or interface identifier
// - StartInputValue, EndInputValue: time range strings (YYYY-MM-DDTHH:mm)
// - PostgrestUrl: base URL for PostgREST
// Optional via querystring (filters): srcaddr, dstaddr, srcport, dstport, protocol, etc.

(function(){
  const container = '{{.Container}}';

  if (window.Highcharts) {
    Highcharts.setOptions({ time: { useUTC: false }, lang: { numericSymbols: null, thousandsSep: ',' } });
  }

  // Build PostgREST URL dynamically with basic filters
  // Expect a materialized or view providing aggregated flows at a coarse bucket
  // e.g., flows_agg_5min with columns: srcaddr, dstaddr, total_octets
  const base = '{{.PostgrestUrl}}flows_agg_5min';
  const params = new URLSearchParams();
  params.set('select', 'srcaddr,dstaddr,total_octets.sum()');
  params.set('exporter', 'eq.{{.Exporter}}');
  params.set('{{.InputOrOutput}}', 'eq.{{.Interface}}');
  params.set('bucket_5min', 'gt.{{.StartInputValue}}');
  params.set('bucket_5min', 'lt.{{.EndInputValue}}');

  // If the current page URL has additional filter query params, pass them through
  try{
    const pageQS = new URLSearchParams(location.search);
    ['srcaddr','dstaddr','srcport','dstport','protocol'].forEach(k=>{
      if (pageQS.has(k)) params.set(k, pageQS.get(k));
    });
  }catch(_){ }

  const url = base + '?' + params.toString();

  const chart = Highcharts.chart(container, {
    chart: { inverted: false },
    title: { text: 'Top Conversations (Sankey)', align: 'left' },
    tooltip: { pointFormat: '<b>{point.from} → {point.to}</b>: {point.weight}' },
    exporting: { enabled: true },
    series: [{
      keys: ['from', 'to', 'weight'],
      type: 'sankey',
      name: 'Traffic by conversations',
      data: []
    }],
    responsive: { rules: [{ condition: { maxWidth: 600 }, chartOptions: { legend: { enabled: false } } }] }
  });

  $.getJSON(url, function(rows){
    if (!rows) rows = [];

    // Aggregate by pair srcaddr→dstaddr summing total_octets
    const map = new Map();
    for (const r of rows){
      const from = r.srcaddr || 'n/a';
      const to   = r.dstaddr || 'n/a';
      const key = from + '|' + to;
      const val = (r.sum || r.total_octets || r.total_octets_sum || 0);
      map.set(key, (map.get(key) || 0) + (typeof val==='number'?val:(Number(val)||0)));
    }
    // Convert to array of tuples, limit to top N to keep chart readable
    const tuples = Array.from(map.entries()).map(([k, w])=>{ const [f,t]=k.split('|'); return [f,t,w]; });
    tuples.sort((a,b)=>b[2]-a[2]);
    const top = tuples.slice(0, 200); // cap to 200 links for performance

    setTimeout(function(){ chart.series[0].setData(top, true); }, 0);
  });
})();
