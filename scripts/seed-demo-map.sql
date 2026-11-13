TRUNCATE TABLE IF EXISTS traffic_logs;
TRUNCATE TABLE IF EXISTS traffic_edges_daily;
TRUNCATE TABLE IF EXISTS traffic_edges_hourly;
TRUNCATE TABLE IF EXISTS traffic_edges_city_daily;
TRUNCATE TABLE IF EXISTS traffic_edges_country_daily;
TRUNCATE TABLE IF EXISTS traffic_edges_continent_daily;

INSERT INTO traffic_logs (
  timestamp, vendor, device, connector_id,
  src_ip, dst_ip, src_port, dst_port, action, rule, proto,
  src_zone, dst_zone, src_country, dst_country, src_city, dst_city,
  src_region, dst_region, src_lat, src_lon, dst_lat, dst_lon,
  bytes_sent, bytes_recv, packets_sent, packets_recv
) VALUES
(now64(3) - INTERVAL 12 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.10'), toIPv4('51.15.1.10'), 49152, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'United Kingdom', 'Moscow', 'London', 'Moscow', 'England', 55.7558, 37.6173, 51.5074, -0.1278, 850000, 2100000, 1200, 3400),
(now64(3) - INTERVAL 18 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.11'), toIPv4('18.196.1.20'), 49153, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'Germany', 'Moscow', 'Frankfurt', 'Moscow', 'Hesse', 55.7558, 37.6173, 50.1109, 8.6821, 420000, 980000, 800, 1500),
(now64(3) - INTERVAL 25 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.12'), toIPv4('37.139.1.30'), 49154, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'Netherlands', 'Moscow', 'Amsterdam', 'Moscow', 'North Holland', 55.7558, 37.6173, 52.3676, 4.9041, 310000, 720000, 600, 1100),
(now64(3) - INTERVAL 9 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.13'), toIPv4('104.16.1.40'), 49155, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'United States', 'Moscow', 'New York', 'Moscow', 'New York', 55.7558, 37.6173, 40.7128, -74.0060, 1200000, 3400000, 2100, 5200),
(now64(3) - INTERVAL 33 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.14'), toIPv4('47.91.1.50'), 49156, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'United Arab Emirates', 'Moscow', 'Dubai', 'Moscow', 'Dubai', 55.7558, 37.6173, 25.2048, 55.2708, 190000, 450000, 400, 900),
(now64(3) - INTERVAL 41 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.15'), toIPv4('13.228.1.60'), 49157, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'Singapore', 'Moscow', 'Singapore', 'Moscow', 'Singapore', 55.7558, 37.6173, 1.3521, 103.8198, 560000, 1400000, 950, 2200),
(now64(3) - INTERVAL 15 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.16'), toIPv4('52.68.1.70'), 49158, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'Japan', 'Moscow', 'Tokyo', 'Moscow', 'Tokyo', 55.7558, 37.6173, 35.6762, 139.6503, 780000, 1900000, 1300, 2800),
(now64(3) - INTERVAL 7 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.17'), toIPv4('177.71.1.80'), 49159, 22, 'deny', 'ssh-block', 'TCP', 'LAN', 'WAN', 'Russia', 'Brazil', 'Moscow', 'Sao Paulo', 'Moscow', 'Sao Paulo', 55.7558, 37.6173, -23.5505, -46.6333, 1200, 0, 4, 0),
(now64(3) - INTERVAL 50 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.18'), toIPv4('13.54.1.90'), 49160, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'Australia', 'Moscow', 'Sydney', 'Moscow', 'New South Wales', 55.7558, 37.6173, -33.8688, 151.2093, 240000, 610000, 500, 1000),
(now64(3) - INTERVAL 20 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('51.15.1.11'), toIPv4('185.10.10.20'), 443, 51443, 'allow', 'inet-in', 'TCP', 'WAN', 'LAN', 'United Kingdom', 'Russia', 'London', 'Moscow', 'England', 'Moscow', 51.5074, -0.1278, 55.7558, 37.6173, 900000, 400000, 1500, 900),
(now64(3) - INTERVAL 28 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('18.196.1.21'), toIPv4('185.10.10.21'), 443, 51443, 'allow', 'inet-in', 'TCP', 'WAN', 'LAN', 'Germany', 'Russia', 'Frankfurt', 'Moscow', 'Hesse', 'Moscow', 50.1109, 8.6821, 55.7558, 37.6173, 500000, 220000, 900, 500),
(now64(3) - INTERVAL 5 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.10'), toIPv4('51.15.1.12'), 49161, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'United Kingdom', 'Moscow', 'London', 'Moscow', 'England', 55.7558, 37.6173, 51.5074, -0.1278, 700000, 1800000, 1000, 3000),
(now64(3) - INTERVAL 3 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.13'), toIPv4('104.16.1.41'), 49162, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'United States', 'Moscow', 'New York', 'Moscow', 'New York', 55.7558, 37.6173, 40.7128, -74.0060, 1100000, 3000000, 2000, 4800),
(now64(3) - INTERVAL 2 MINUTE, 'demo', 'fw-edge-1', 'default', toIPv4('185.10.10.16'), toIPv4('52.68.1.71'), 49163, 443, 'allow', 'inet-out', 'TCP', 'LAN', 'WAN', 'Russia', 'Japan', 'Moscow', 'Tokyo', 'Moscow', 'Tokyo', 55.7558, 37.6173, 35.6762, 139.6503, 640000, 1500000, 1100, 2500);
