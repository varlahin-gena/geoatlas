# OpenSSF Best Practices — шпаргалка (Passing)

Заполнение формы на [bestpractices.dev](https://www.bestpractices.dev/) для репозитория
[varlahin-gena/geoatlas](https://github.com/varlahin-gena/geoatlas).

Критерии Passing: https://www.bestpractices.dev/en/criteria/0  
После создания проекта подставьте ID в бейдж README.

Ниже — рекомендуемые ответы **Met** / **N/A** и URL обоснований (пути от корня репо на `main`).

## Basics

| ID | Ответ | Обоснование |
|----|-------|-------------|
| description_good | Met | [README.md](../README.md) — что делает продукт |
| interact | Met | README: установка, Issues, [CONTRIBUTING.md](../CONTRIBUTING.md) |
| contribution | Met | https://github.com/varlahin-gena/geoatlas/blob/main/CONTRIBUTING.md |
| contribution_requirements | Met | CONTRIBUTING: тесты, CI, RELEASING, без секретов |
| floss_license | Met | Apache-2.0 |
| floss_license_osi | Met | OSI-approved Apache-2.0 |
| license_location | Met | https://github.com/varlahin-gena/geoatlas/blob/main/LICENSE |
| documentation_basics | Met | [docs/install.md](install.md), [docs/operations.md](operations.md), README «Безопасность» |
| documentation_interface | Met | [openapi.yaml](../openapi.yaml), [docs/ui.md](ui.md) |
| sites_https | Met | GitHub HTTPS |
| discussion | Met | GitHub Issues + PR |
| english | Met | Issues/PR на EN принимаются (CONTRIBUTING) |
| maintained | Met | Активные релизы / SECURITY SLA |

## Change Control

| ID | Ответ | Обоснование |
|----|-------|-------------|
| repo_public | Met | https://github.com/varlahin-gena/geoatlas |
| repo_track | Met | git history |
| repo_interim | Met | ветки/коммиты между релизами |
| repo_distributed | Met | git |
| version_unique | Met | `VERSION`, теги `vX.Y.Z` |
| version_semver | Met | SemVer, [RELEASING.md](../RELEASING.md) |
| version_tags | Met | git tags `v*` |
| release_notes | Met | https://github.com/varlahin-gena/geoatlas/blob/main/CHANGELOG.md |
| release_notes_vulns | N/A или Met | N/A если не было публичных CVE продукта; иначе секция Security в CHANGELOG |

## Reporting

| ID | Ответ | Обоснование |
|----|-------|-------------|
| report_process | Met | https://github.com/varlahin-gena/geoatlas/issues |
| report_tracker | Met | GitHub Issues |
| report_responses | Met | Ответы в Issues (проверяется по факту активности) |
| enhancement_responses | Met | То же |
| report_archive | Met | https://github.com/varlahin-gena/geoatlas/issues |
| vulnerability_report_process | Met | https://github.com/varlahin-gena/geoatlas/blob/main/SECURITY.md |
| vulnerability_report_private | Met | GitHub Security Advisories (ссылка в SECURITY.md) |
| vulnerability_report_response | N/A или Met | N/A если не было отчётов за 6 мес.; иначе ≤14 дней (цель в SECURITY: 5 рабочих дней) |

## Quality

| ID | Ответ | Обоснование |
|----|-------|-------------|
| build | Met | Docker/`./start.sh`, [docs/development.md](development.md) |
| build_common_tools | Met | Go toolchain, npm, Docker Compose |
| build_floss_tools | Met | FLOSS tools |
| test | Met | `go test` / `npm test` в development.md; CI workflow |
| test_invocation | Met | стандартные команды языка |
| test_most | Met (SUGGESTED) | покрытие растёт с фичами |
| test_continuous_integration | Met | `.github/workflows/ci.yml` |
| test_policy | Met | [CONTRIBUTING.md](../CONTRIBUTING.md) «Политика тестов» |
| tests_are_added | Met | тесты рядом с фичами в истории PR |
| tests_documented_added | Met | CONTRIBUTING |
| warnings | Met | golangci-lint, TypeScript/ESLint в CI |
| warnings_fixed | Met | CI падает на lint |
| warnings_strict | Met (SUGGESTED) | строгий lint в CI |

## Security

| ID | Ответ | Обоснование |
|----|-------|-------------|
| know_secure_design | Met | primary developer; fail-safe defaults / least privilege в продукте |
| know_common_errors | Met | OWASP-класс угроз учтён (auth, XSS edge, injection в ingest) |
| crypto_published | Met | TLS на nginx; стандартные алгоритмы |
| crypto_call | Met | bcrypt (`golang.org/x/crypto/bcrypt`), без самописной крипты |
| crypto_floss | Met | FLOSS crypto libs |
| crypto_keylength | Met / N/A | TLS на edge; app keys — session/HMAC секреты достаточной длины |
| crypto_working | Met | без MD5/DES/RC4 по умолчанию |
| crypto_weaknesses | Met (SUGGESTED) | современный стек |
| crypto_pfs | N/A или Met | PFS на TLS termination (nginx); app-level key agreement нет |
| crypto_password_storage | Met | bcrypt для паролей УЗ |
| crypto_random | Met | crypto/rand / CSPRNG платформы |
| delivery_mitm | Met | HTTPS GitHub Releases + опционально cosign |
| delivery_unsigned | Met | `.sha256` по HTTPS; не http-only hash |
| vulnerabilities_fixed_60_days | Met | нет известных незакрытых medium+ продукта >60 дней |
| vulnerabilities_critical_fixed | Met (SUGGESTED) | процесс SECURITY + релизы |
| no_leaked_credentials | Met | gitleaks в CI; секреты только в `.env` локально |

## Analysis

| ID | Ответ | Обоснование |
|----|-------|-------------|
| static_analysis | Met | CodeQL, govulncheck, golangci-lint, Trivy, Scorecard |
| static_analysis_common_vulnerabilities | Met | CodeQL / govulncheck / Trivy |
| static_analysis_fixed | Met | CI блокирует CRITICAL/high по политике workflows |
| static_analysis_often | Met | на PR / push |
| dynamic_analysis | Met (SUGGESTED) | автотесты + CI integration; fuzz jobs в CI где есть |
| dynamic_analysis_unsafe | N/A | нет C/C++ в продукте (Go/TS) |
| dynamic_analysis_enable_assertions | Met (SUGGESTED) | тесты с проверками |
| dynamic_analysis_fixed | N/A или Met | по результатам dynamic analysis |

## После заполнения

1. Сохранить проект → статус **in progress** или **passing**.
2. Вставить бейдж в README:

```markdown
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/<ID>/badge)](https://www.bestpractices.dev/projects/<ID>)
```

3. Перезапустить GitHub Actions workflow **Scorecard** на `main`.
