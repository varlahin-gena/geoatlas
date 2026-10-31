# OpenSSF Best Practices — шпаргалка (Silver)

Форма: https://www.bestpractices.dev/en/projects/15404/silver/edit  
Критерии: https://www.bestpractices.dev/en/criteria/1  
Passing уже получен (проект [15404](https://www.bestpractices.dev/projects/15404)).

Заполнение (сессия bestpractices.dev):

1. Консоль на странице edit — `scripts/openssf-silver-fill-console.js`
2. Или cookie в `~/.badge-cookie.txt` и `python scripts/submit-openssf-silver.py`

Человекочитаемые обоснования — в docs ниже; в JSON для сайта — краткий EN (как у Passing).

## Статус фазы A (docs / процессы)

| ID | Ответ | Куда смотреть |
|----|-------|---------------|
| achieve_passing | Met | Passing badge |
| contribution_requirements | Met | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| dco | Met | CONTRIBUTING + workflow [dco.yml](../.github/workflows/dco.yml) |
| governance | Met | [GOVERNANCE.md](../GOVERNANCE.md) |
| code_of_conduct | Met | [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) |
| roles_responsibilities | Met | GOVERNANCE |
| access_continuity | Met | GOVERNANCE § непрерывность |
| bus_factor | **Unmet** (SHOULD) | один Maintainer — **единственный блокер** Silver-бейджа; план преемника в GOVERNANCE |
| documentation_roadmap | Met | [roadmap.md](roadmap.md) |
| documentation_architecture | Met | [architecture.md](architecture.md) |
| documentation_security | Met | [security-requirements.md](security-requirements.md) |
| documentation_quick_start | Met | README «Быстрый старт» + [install.md](install.md) |
| documentation_current | Met | docs обновляются с релизами |
| documentation_achievements | Met | бейдж в README |
| accessibility_best_practices | **Met** (SHOULD) | [accessibility.md](accessibility.md) |
| internationalization | **Met** (SHOULD) | [i18n.md](i18n.md) — i18next + ru/en |
| sites_password_security | N/A | пароли пользователей сайта проекта не храним (GitHub) |
| maintenance_or_update | Met | [operations.md](operations.md) обновление; SECURITY — последний релиз |
| report_tracker | Met | GitHub Issues |
| vulnerability_report_credit | N/A | не было resolved vuln reports за 12 мес.; политика credit в SECURITY |
| vulnerability_response_process | Met | [SECURITY.md](../SECURITY.md) |
| coding_standards | Met | CONTRIBUTING «Стандарты кода» |
| coding_standards_enforced | Met | golangci-lint / ESLint в CI |
| build_* / installation_* | Met / N/A | Docker/`./start.sh`; Go modules; [development.md](development.md), [install.md](install.md) |
| external_dependencies | Met | go.mod / package-lock |
| dependency_monitoring | Met | Dependabot, govulncheck, Trivy/Grype |
| updateable_reused_components | Met | стандартные менеджеры зависимостей |
| interfaces_current | Met | без опоры на заведомо obsolete API в нашем коде |
| automated_integration_testing | Met | CI на PR/push |
| regression_tests_added50 | Met | политика в CONTRIBUTING |
| **test_statement_coverage80** | **Met** | total Go statement coverage ≥80%; gate в CI (`coverage overall (≥80%)`) |
| test_policy_mandated / tests_documented_added | Met | CONTRIBUTING |
| warnings_strict | Met | lint падает в CI |
| implement_secure_design | Met | [assurance-case.md](assurance-case.md) |
| crypto_* (silver) | Met / N/A | bcrypt, TLS на nginx, секреты в файлах `.env` |
| **signed_releases** | Met | cosign keyless + verify в [install.md](install.md) |
| version_tags_signed | **Met** (SUGGESTED) | cosign keyless на `.gittag` (tag+commit) в release Assets; verify в [install.md](install.md) |
| input_validation | Met | threatprot, лимиты upload, safeurl |
| hardening | Met | proxy gate, rate limit, headers — configuration.md |
| assurance_case | Met | [assurance-case.md](assurance-case.md) |
| static_analysis_common_vulnerabilities | Met | CodeQL / govulncheck |
| dynamic_analysis_unsafe | N/A | нет C/C++ |

## После фазы A / B

1. Coverage gate и `test_statement_coverage80` → Met — сделано.
2. a11y / i18n / `version_tags_signed` → Met в форме после merge (см. `scripts/openssf-silver-fill.json`).
3. `bus_factor` остаётся Unmet при одном Maintainer — Silver-бейдж не выдаётся, пока не будет co-maintainer (≥2).
