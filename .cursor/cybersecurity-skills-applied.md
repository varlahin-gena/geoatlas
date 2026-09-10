# Cybersecurity Skills Applied to GeoAtlas

Tracking which skills from [Anthropic-Cybersecurity-Skills](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills) were used on this project.

## Used

| Skill | Applied | Notes |
|-------|---------|-------|
| [conducting-api-security-testing](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/conducting-api-security-testing) | 2026-09-02 | |
| [testing-api-security-with-owasp-top-10](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/testing-api-security-with-owasp-top-10) | 2026-09-02 | |
| [testing-for-broken-access-control](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/testing-for-broken-access-control) | 2026-09-02 | |
| [testing-jwt-token-security](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/testing-jwt-token-security) | 2026-09-02 | JWT нет; чеклист применён к HMAC-сессиям и opaque Bearer |
| [implementing-api-schema-validation-security](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-api-schema-validation-security) | 2026-09-02 | OpenAPI `additionalProperties: false` + maxLength; runtime `DisallowUnknownFields` + JSON depth/string caps; Spectral + `schema_security_check.py` + CI workflow |
| [detecting-api-enumeration-attacks](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-api-enumeration-attacks) | 2026-09-02 | |
| [performing-csrf-attack-simulation](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-csrf-attack-simulation) | 2026-09-02 | |
| [performing-security-headers-audit](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-security-headers-audit) | 2026-09-03 | |
| [performing-directory-traversal-testing](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-directory-traversal-testing) | 2026-09-03 | Code review: нет LFI/`?file=` download; backup allowlist + `os.OpenRoot`; threatprot path guard; tar-slip в apply_package |
| [implementing-api-threat-protection-with-apigee](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-api-threat-protection-with-apigee) | 2026-09-03 | Go `threatprot/` + `apiThreatMW`: SpikeArrest, JSONThreatProtection (depth/name/entries/array/string), RegEx path/CRLF guard, API security headers, Retry-After; docs `GA_API_RATE_*` |
| [building-detection-rules-with-sigma](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-detection-rules-with-sigma) | 2026-09-03 | Оценка fit: Sigma как portable detection-as-code поверх `traffic_logs` / audit (внешний SIEM или CH SQL), не замена anomaly engine / threatprot; см. chat |
| [implementing-mitre-attack-coverage-mapping](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-mitre-attack-coverage-mapping) | 2026-09-03 | |
| [building-threat-hunt-hypothesis-framework](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-threat-hunt-hypothesis-framework) | 2026-09-03 | Адаптация 7-шагового hunt-workflow под firewall/ClickHouse (не EDR): гипотезы → map query / Saved hunts → anomalies + investigate → отчёт TH-GEO-[DATE]-[SEQ]; см. chat |

## Failed / not applied

| Skill | Date | Reason |
|-------|------|--------|
| [exploiting-server-side-request-forgery](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/exploiting-server-side-request-forgery) | 2026-09-03 | Не применим к GeoAtlas (offensive SSRF exploit skill; не подходит под defensive hardening/`safeurl` и текущий scope) |

## Remaining candidates (not used yet)

Рекомендованные ранее скиллы минус Used / Failed. Блок API security из исходного списка закрыт.

### Близко к текущему продукту

| Skill | Зачем |
|-------|-------|
| [implementing-api-rate-limiting-and-throttling](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-api-rate-limiting-and-throttling) | Rate limit / login throttle |
| [detecting-beaconing-patterns-with-zeek](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-beaconing-patterns-with-zeek) | Anomaly `beaconing` |
| [hunting-for-beaconing-with-frequency-analysis](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/hunting-for-beaconing-with-frequency-analysis) | Улучшение beaconing |
| [detecting-lateral-movement-in-network](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-lateral-movement-in-network) | Anomaly `lateral_fanout` |
| [detecting-network-scanning-with-ids-signatures](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-network-scanning-with-ids-signatures) | `port_scan` / `horizontal_scan` |
| [detecting-port-scanning-with-fail2ban](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-port-scanning-with-fail2ban) | Логика сканов из firewall-логов |
| [performing-ip-reputation-analysis-with-shodan](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-ip-reputation-analysis-with-shodan) | IP reputation |
| [analyzing-indicators-of-compromise](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-indicators-of-compromise) | IOC / alerts / investigate |
| [building-threat-intelligence-feed-integration](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-threat-intelligence-feed-integration) | Reputation URL feeds |
| [analyzing-threat-intelligence-feeds](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-threat-intelligence-feeds) | Качество feeds |
| [analyzing-web-server-logs-for-intrusion](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-web-server-logs-for-intrusion) | Парсинг / корреляция логов |
| [analyzing-api-gateway-access-logs](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-api-gateway-access-logs) | Audit / API access patterns |
| [implementing-network-traffic-baselining](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-network-traffic-baselining) | Baseline для surge / new_country |
| [generating-and-analyzing-sboms](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/generating-and-analyzing-sboms) | SBOM в CI |

### Threat hunting / detection

| Skill | Зачем |
|-------|-------|
| [analyzing-network-traffic-for-incidents](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-network-traffic-for-incidents) | Investigate workspace |
| [analyzing-network-flow-data-with-netflow](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-network-flow-data-with-netflow) | NetFlow/IPFIX |
| [performing-network-traffic-analysis-with-zeek](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-network-traffic-analysis-with-zeek) | Доп. детекторы |
| [hunting-for-unusual-network-connections](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/hunting-for-unusual-network-connections) | Saved hunts |
| [hunting-for-data-exfiltration-indicators](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/hunting-for-data-exfiltration-indicators) | Новый anomaly |
| [detecting-dns-exfiltration-with-dns-query-analysis](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-dns-exfiltration-with-dns-query-analysis) | Если есть DNS в логах |
| [detecting-command-and-control-over-dns](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-command-and-control-over-dns) | C2 через DNS |
| [analyzing-dns-logs-for-exfiltration](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-dns-logs-for-exfiltration) | DNS как источник |
| [detecting-ransomware-precursors-in-network](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/detecting-ransomware-precursors-in-network) | SOC use case |
| [analyzing-ransomware-network-indicators](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-ransomware-network-indicators) | Карта + reputation |
| [building-detection-rule-with-splunk-spl](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-detection-rule-with-splunk-spl) | Шаблоны ClickHouse SQL |
| [analyzing-threat-actor-ttps-with-mitre-attack](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-threat-actor-ttps-with-mitre-attack) | Контекст расследований |
| [analyzing-cyber-kill-chain](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-cyber-kill-chain) | Группировка событий |

### Threat intelligence / IOC

| Skill | Зачем |
|-------|-------|
| [automating-ioc-enrichment](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/automating-ioc-enrichment) | Автообогащение IP |
| [building-ioc-enrichment-pipeline-with-opencti](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-ioc-enrichment-pipeline-with-opencti) | OpenCTI/MISP |
| [collecting-indicators-of-compromise](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/collecting-indicators-of-compromise) | Reputation pipeline |
| [processing-stix-taxii-feeds](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/processing-stix-taxii-feeds) | STIX/TAXII feeds |
| [building-threat-feed-aggregation-with-misp](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-threat-feed-aggregation-with-misp) | MISP как источник |
| [performing-ioc-enrichment-automation](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-ioc-enrichment-automation) | Enrichment в investigate |

### IR / SOC

| Skill | Зачем |
|-------|-------|
| [building-incident-response-dashboard](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-incident-response-dashboard) | Investigate + System |
| [building-incident-response-playbook](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-incident-response-playbook) | Playbooks по anomaly |
| [building-incident-timeline-with-timesketch](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-incident-timeline-with-timesketch) | Timeline |
| [triaging-security-incident-with-ir-playbook](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/triaging-security-incident-with-ir-playbook) | Ack/assign |
| [building-soc-metrics-and-kpi-tracking](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-soc-metrics-and-kpi-tracking) | Prometheus / Charts |
| [implementing-alert-fatigue-reduction](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-alert-fatigue-reduction) | Дедуп / приоритеты |

### Инфраструктура / эксплуатация

| Skill | Зачем |
|-------|-------|
| [hardening-docker-containers-for-production](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/hardening-docker-containers-for-production) | Docker appliance |
| [performing-docker-bench-security-assessment](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-docker-bench-security-assessment) | Аудит контейнеров |
| [scanning-docker-images-with-trivy](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/scanning-docker-images-with-trivy) | CI image scan |
| [building-devsecops-pipeline-with-gitlab-ci](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/building-devsecops-pipeline-with-gitlab-ci) | Расширение CI |
| [implementing-secret-scanning-with-gitleaks](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-secret-scanning-with-gitleaks) | Secrets в репо |
| [implementing-secrets-management-with-vault](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-secrets-management-with-vault) | Secrets management |
| [configuring-tls-1-3-for-secure-communications](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/configuring-tls-1-3-for-secure-communications) | TLS module |
| [performing-ssl-certificate-lifecycle-management](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/performing-ssl-certificate-lifecycle-management) | Сертификаты |
| [implementing-immutable-backup-with-restic](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-immutable-backup-with-restic) | ClickHouse backup |
| [implementing-ransomware-backup-strategy](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-ransomware-backup-strategy) | Стратегия бэкапов |
| [validating-backup-integrity-for-recovery](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/validating-backup-integrity-for-recovery) | Тест restore |
| [configuring-suricata-for-network-monitoring](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/configuring-suricata-for-network-monitoring) | Доп. источник логов |
| [configuring-snort-ids-for-intrusion-detection](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/configuring-snort-ids-for-intrusion-detection) | То же |
| [configuring-pfsense-firewall-rules](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/configuring-pfsense-firewall-rules) | Новый parser / docs |
| [implementing-log-forwarding-with-fluentd](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-log-forwarding-with-fluentd) | Альтернатива syslog-ng |
| [analyzing-sbom-for-supply-chain-vulnerabilities](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/analyzing-sbom-for-supply-chain-vulnerabilities) | Supply chain |
| [achieving-cmmc-level-2-compliance](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/achieving-cmmc-level-2-compliance) | Compliance |
| [implementing-iso-27001-information-security-management](https://github.com/mukul975/Anthropic-Cybersecurity-Skills/tree/main/skills/implementing-iso-27001-information-security-management) | Документация / процессы |

### Следующие приоритеты

1. `building-threat-hunt-hypothesis-framework` — saved hunts / гипотезы поверх query builder
2. `automating-ioc-enrichment` — автообогащение IP в alerts/investigate
3. `processing-stix-taxii-feeds` — стандартные TI feeds вместо plain URL lists
4. `analyzing-network-traffic-for-incidents` — workflow расследования
5. `implementing-network-traffic-baselining` — снизить FP у surge / new_country
6. `hunting-for-beaconing-with-frequency-analysis` — улучшить детектор beaconing
7. `detecting-dns-exfiltration-with-dns-query-analysis` — только если в логах есть DNS
8. `hardening-docker-containers-for-production` — hardening appliance
9. `building-incident-response-playbook` — playbooks по типам anomaly
10. `implementing-alert-fatigue-reduction` — дедуп / приоритеты алертов
11. `implementing-api-rate-limiting-and-throttling` — вероятно уже покрыто `threatprot`; можно пропустить или быстрый gap-check
