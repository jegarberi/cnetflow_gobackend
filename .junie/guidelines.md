# Project Guidelines

#### Backend Support for Filters
- Accept filter parameters via query strings or path parameters
- Build PostgREST queries dynamically: `?srcaddr=eq.X&dstport=eq.Y&protocol=eq.Z`
- Return filtered data in Highcharts-compatible format
- Support aggregation: `select=srcaddr,dstaddr,sum(octets)`

### Chart Interaction and Updates
- Use Web Workers for sorting/processing large datasets (avoid blocking UI)
- Update charts dynamically: `chart.series[0].setData(newData, true)`
- Use `setTimeout` to batch multiple updates
- Enable export buttons (Highcharts exporting module)
- Make charts responsive with `responsive.rules`

### Data Endpoints for Charts
- **Interface Metrics:** `/api/v1/metrics/{exporter}/{interface}/{start}/{end}/js`
    - Returns JavaScript to initialize Highcharts time series
    - Calculate bits/s from octet counters
- **Flow Pie Charts:** `/api/v1/flows/{container}/{exporter}/{interface}/{start}/{end}/{src_or_dst}/{bytes_packets}/{direction}/js`
    - Returns JavaScript for pie chart initialization
    - Pre-aggregated by address/port/protocol
- **Flow Sankey:** Create new endpoint `/api/v1/flows/sankey/{exporter}/{interface}/{start}/{end}[?filters...]`
    - Return JSON with `from`, `to`, `weight` tuples
    - Support port-level drill-down

## Frontend Development with htmx

### When to Use htmx
- **ALWAYS prefer htmx over vanilla JavaScript** for dynamic interactions
- Use htmx for loading content dynamically without full page reloads
- Use htmx for form submissions and user interactions
- Use htmx for polling and real-time updates

### htmx Best Practices
- Use `hx-get`, `hx-post`, `hx-put`, `hx-delete` for HTTP requests
- Use `hx-target` to specify where response HTML should be inserted
- Use `hx-swap` to control how content is swapped (innerHTML, outerHTML, beforeend, etc.)
- Use `hx-trigger` for custom event triggers (click, load, every Xs, change delay:500ms, etc.)
- Use `hx-indicator` to show loading states
- Return HTML fragments from backend endpoints (not JSON) when using htmx
- Use `hx-vals` or `hx-include` to send additional data with requests

### htmx Patterns in This Project
- Navigation lists use `hx-post` to load content into target divs
- Use `hx-target="#body"` or `hx-target="#interfaces_div"` for content areas
- Combo boxes/selects use `hx-get` with `hx-target` for dynamic filtering
- Return HTML list items (`<li>`) or options (`<option>`) from backend for htmx
- **Filter changes trigger chart reloads via htmx**

### Backend Support for htmx
- Create endpoints that return HTML fragments, not just JSON
- Support multiple formats via path parameters: `/api/v1/resource/{format}` where format can be:
    - `json` - for API consumers
    - `list` - for htmx list items
    - `combo` - for htmx select/dropdown options
- Use Go templates when returning complex HTML structures
- Keep HTML generation simple and focused on the data

### Avoid These Frontend Patterns
- Don't use traditional JavaScript AJAX when htmx can do it
- Don't build complex JavaScript SPAs - use htmx for dynamic behavior
- Don't return JSON and then manipulate DOM with JavaScript when you can return HTML
- Avoid jQuery or other heavy frameworks - htmx + vanilla JS is sufficient
- **Exception: Use jQuery for Highcharts data fetching ($.getJSON) as it's already loaded**

## Common Patterns in This Project

### Flow Processing
- Flows are aggregated by source/destination pairs
- Geographic coordinates are looked up using MaxMind GeoIP database
- Default coordinates (-34.5823511, -58.6027697) are used when lookup fails
- Distance calculations use Haversine formula

### Metrics Collection
- Interface metrics track octets in/out over time
- Metrics are stored with timestamps for time-series analysis
- Results are sorted chronologically before returning
- **Calculate rate from delta values, not raw counters**

### API Design
- RESTful endpoints following `/api/v1/{resource}/{id}/{action}` pattern
- Support multiple output formats (json, list, combo) via path parameters
- Use consistent error responses
- **Return HTML fragments for htmx consumers, JSON for API consumers**
- **Return JavaScript snippets for Highcharts initialization when format=js**

## Things to Avoid
- Don't use `panic()` in HTTP handlers
- Don't ignore errors returned by database or I/O operations
- Don't hardcode configuration values - use environment variables
- Avoid SQL string concatenation - use parameterized queries
- Don't leave database connections open without `defer`ing cleanup
- **Don't use vanilla JavaScript AJAX when htmx is available**
- **Don't build heavy client-side logic - keep it server-side and use htmx**
- **Don't use other charting libraries - always use Highcharts**
- **Don't forget to calculate deltas for rate-based metrics**

## Testing & Debugging
- Use `log.SetFlags(log.LstdFlags | log.Lshortfile)` for debugging
- Log intermediate values when troubleshooting
- Test edge cases (nil values, empty strings, zero timestamps)
- Validate data from external sources (database, HTTP requests)
- Test htmx interactions in browser developer tools (network tab)
- **Use browser console to debug Highcharts rendering issues**
- **Verify data format matches Highcharts expectations**

## Security Considerations
- Always use parameterized SQL queries
- Validate and sanitize user input
- Use appropriate error messages (don't expose internal details)
- Set proper CORS headers if needed for frontend integration
- Sanitize HTML output when using user-provided data in htmx responses

## Documentation
- Add comments for exported functions and types
- Document non-obvious logic or algorithms
- Keep comments up-to-date with code changes
- Use meaningful names that reduce need for comments

## When Making Changes
1. Understand the full context of what's being changed
2. Maintain consistency with existing patterns
3. Test changes with representative data
4. Consider error cases and edge conditions
5. Update related documentation if applicable
6. Ensure database connections are properly managed
7. Validate HTTP responses and status codes
8. **When adding interactivity, default to htmx-based solutions**
9. **Return appropriate format (HTML for htmx, JSON for APIs, JS for Highcharts)**
10. **Use Highcharts for all visualizations - line/area for metrics, Sankey for conversations, pie for distribution**
11. **Implement comprehensive filters: IPs, ports, protocols, dates**
12. **Make filters dynamic using htmx to reload charts**