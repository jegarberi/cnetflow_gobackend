(function () {
    const img = document.getElementById('chart-img');
    const overlay = document.getElementById('chart-overlay');
    const out = document.getElementById('selection-output');
    const clearBtn = document.getElementById('clear-selection');

// Optional: set your data x-domain for mapping pixels -> time (ISO strings here)
// If you don't need mapping, leave both null.
    const domainStartISO = null; // e.g., '2025-09-01T00:00:00Z'
    const domainEndISO = null;   // e.g., '2025-09-02T00:00:00Z'

    let startX = null;
    let endX = null;
    let dragging = null; // 'start' | 'end' | 'range' | null
    let dragOffset = 0;

// Scale-aware canvas setup
    function fitOverlayToImage() {
        const rect = img.getBoundingClientRect();
        overlay.style.width = rect.width + 'px';
        overlay.style.height = rect.height + 'px';
        overlay.width = Math.max(1, Math.round(rect.width * devicePixelRatio));
        overlay.height = Math.max(1, Math.round(rect.height * devicePixelRatio));
        overlay.style.left = img.offsetLeft + 'px';
        overlay.style.top = img.offsetTop + 'px';
        draw();
    }

    function pxToDomain(xPx) {
        if (!domainStartISO || !domainEndISO) return null;
        const rect = img.getBoundingClientRect();
        const t0 = new Date(domainStartISO).getTime();
        const t1 = new Date(domainEndISO).getTime();
        const ratio = Math.min(1, Math.max(0, xPx / rect.width));
        const t = t0 + ratio * (t1 - t0);
        return new Date(t);
    }

    function domainToPx(date) {
        if (!domainStartISO || !domainEndISO) return null;
        const rect = img.getBoundingClientRect();
        const t0 = new Date(domainStartISO).getTime();
        const t1 = new Date(domainEndISO).getTime();
        const ratio = (date.getTime() - t0) / (t1 - t0);
        return ratio * rect.width;
    }

    function formatOutput() {
        if (startX == null || endX == null) {
            out.textContent = 'No selection';
            return;
        }
        const a = Math.min(startX, endX);
        const b = Math.max(startX, endX);
        const rect = img.getBoundingClientRect();
        const pctA = (a / rect.width * 100).toFixed(1);
        const pctB = (b / rect.width * 100).toFixed(1);

        const startDate = pxToDomain(a);
        const endDate = pxToDomain(b);
        if (startDate && endDate) {
            out.textContent = `Pixels: [${Math.round(a)}, ${Math.round(b)}] | %: [${pctA}%, ${pctB}%] | Time: [${startDate.toISOString()} .. ${endDate.toISOString()}]`;
        } else {
            out.textContent = `Pixels: [${Math.round(a)}, ${Math.round(b)}] | %: [${pctA}%, ${pctB}%]`;
        }
    }

    function draw() {
        const ctx = overlay.getContext('2d');
        const dpr = devicePixelRatio || 1;
        const rect = img.getBoundingClientRect();

        ctx.setTransform(dpr, 0, 0, dpr, 0, 0); // normalize for CSS pixels
        ctx.clearRect(0, 0, overlay.width, overlay.height);

        if (startX == null && endX == null) return;

        const a = Math.min(startX ?? endX, endX ?? startX);
        const b = Math.max(startX ?? endX, endX ?? startX);

// Shaded selection
        ctx.fillStyle = 'rgba(33, 150, 243, 0.18)';
        ctx.fillRect(a, 0, Math.max(1, b - a), rect.height);

// Guide lines
        ctx.strokeStyle = '#2196f3';
        ctx.lineWidth = 2;
        ctx.beginPath();
        if (startX != null) { ctx.moveTo(startX + 0.5, 0); ctx.lineTo(startX + 0.5, rect.height); }
        if (endX != null) { ctx.moveTo(endX + 0.5, 0); ctx.lineTo(endX + 0.5, rect.height); }
        ctx.stroke();

// Drag handles
        ctx.fillStyle = '#1976d2';
        const handleW = 6, handleH = 18;
        if (startX != null) ctx.fillRect(startX - handleW/2, 6, handleW, handleH);
        if (endX != null) ctx.fillRect(endX - handleW/2, 6, handleW, handleH);
    }

    function getLocalX(evt) {
        const r = img.getBoundingClientRect();
        const x = (evt.clientX - r.left);
        return Math.min(r.width, Math.max(0, x));
    }

    function hitTest(x, target, tol = 6) {
        return Math.abs(x - target) <= tol;
    }

    function onMouseDown(evt) {
// Enable pointer capture on the wrapper to support dragging outside
        const x = getLocalX(evt);
        if (startX != null && hitTest(x, startX)) {
            dragging = 'start';
        } else if (endX != null && hitTest(x, endX)) {
            dragging = 'end';
        } else if (startX != null && endX != null) {
            const a = Math.min(startX, endX);
            const b = Math.max(startX, endX);
            if (x >= a && x <= b) {
                dragging = 'range';
                dragOffset = x - a; // distance from left edge of selection
            } else {
// start a new selection
                startX = x;
                endX = null;
                dragging = 'end';
            }
        } else if (startX == null && endX == null) {
            startX = x;
            endX = null;
            dragging = 'end';
        } else {
// one handle exists; set/drag the other
            if (startX == null) { startX = x; dragging = 'start'; }
            else { endX = x; dragging = 'end'; }
        }
        draw();
        formatOutput();
        window.addEventListener('mousemove', onMouseMove);
        window.addEventListener('mouseup', onMouseUp, { once: true });
    }

    function onMouseMove(evt) {
        const x = getLocalX(evt);
        const rect = img.getBoundingClientRect();
        if (dragging === 'start') {
            startX = Math.min(Math.max(0, x), rect.width);
        } else if (dragging === 'end') {
            endX = Math.min(Math.max(0, x), rect.width);
        } else if (dragging === 'range') {
// move both while preserving width
            const width = Math.abs(endX - startX);
            let a = Math.min(Math.max(0, x - dragOffset), rect.width - width);
            let b = a + width;
            if (startX <= endX) { startX = a; endX = b; } else { endX = a; startX = b; }
        }
        draw();
        formatOutput();
    }

    function onMouseUp() {
        dragging = null;
        window.removeEventListener('mousemove', onMouseMove);
        formatOutput();
    }

    function clearSelection() {
        startX = null; endX = null; dragging = null;
        draw(); formatOutput();
    }

// Wire up events
    img.addEventListener('load', fitOverlayToImage);
    window.addEventListener('resize', fitOverlayToImage);
    clearBtn.addEventListener('click', clearSelection);

// Allow initiating drag on overlay area (even though pointer-events is none on canvas)
// We attach mousedown to the wrapper instead.
    const wrapper = document.querySelector('.chart-wrap');
    wrapper.addEventListener('mousedown', onMouseDown);

// If image is already loaded from cache
    if (img.complete) fitOverlayToImage();
})();
