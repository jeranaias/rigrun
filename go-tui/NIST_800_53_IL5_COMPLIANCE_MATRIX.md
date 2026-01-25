# NIST 800-53 Rev 5 IL5 Complete Compliance Matrix

**Classification:** UNCLASSIFIED
**Document Version:** 2.0
**Date:** 2026-01-20
**Purpose:** Complete IL5 compliance tracking with implementation status and remediation plans

---

## Compliance Summary

| Status | Count | Percentage |
|--------|-------|------------|
| **COMPLIANT** | 22 | 11% |
| **PARTIAL** | 31 | 16% |
| **NOT COMPLIANT** | 98 | 50% |
| **NOT APPLICABLE** | 46 | 23% |
| **TOTAL** | 197 | 100% |

---

## Legend

- **C** = COMPLIANT (Fully meets requirement)
- **P** = PARTIAL (Some aspects implemented)
- **N** = NOT COMPLIANT (Gap exists)
- **NA** = NOT APPLICABLE (Control not relevant to software)

---

## AC - Access Control Family (25 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| AC-1 | Policy and Procedures | N | None | No policy documentation framework | Create `/etc/rigrun/policies/` structure with AC policy template |
| AC-2 | Account Management | P | Session tracking exists | Missing account lifecycle, types, review | Add `rigrun account` command with create/modify/delete/review |
| AC-2(1) | Automated System Account Management | N | None | No automated account management | Implement automated account provisioning/deprovisioning |
| AC-2(2) | Automated Temporary/Emergency Account Management | N | None | No temp account handling | Add temp account with auto-expiration |
| AC-2(3) | Disable Accounts | N | None | No account disable capability | Add `rigrun account disable` with audit trail |
| AC-2(4) | Automated Audit Actions | P | Audit logging exists | Account events not all logged | Add ACCOUNT_CREATE, ACCOUNT_MODIFY, ACCOUNT_DELETE events |
| AC-3 | Access Enforcement | P | Tool sandboxing | No RBAC enforcement | Implement role-based access control system |
| AC-3(4) | Discretionary Access Control | N | None | No DAC | Add file/resource permission system |
| AC-4 | Information Flow Enforcement | P | Network isolation (paranoid/offline modes) | No classification-based routing | Add classification-aware data flow controls |
| AC-4(4) | Flow Control of Encrypted Information | N | None | No encrypted flow control | Add TLS inspection for classification |
| AC-5 | Separation of Duties | N | None | No role separation | Add admin/operator/auditor roles |
| AC-6 | Least Privilege | P | Tool permission levels | No privilege audit | Add privilege tracking and enforcement |
| AC-6(1) | Authorize Access to Security Functions | N | None | No security function access control | Restrict security settings to admin role |
| AC-6(2) | Non-privileged Access for Non-security Functions | N | None | All access is privileged | Add guest/user role with limited access |
| AC-6(5) | Privileged Accounts | N | None | No privileged account distinction | Implement privileged account tracking |
| AC-6(9) | Log Use of Privileged Functions | P | Query logging exists | Privilege use not explicitly logged | Add PRIVILEGE_USE event type |
| AC-6(10) | Prohibit Non-privileged Users | N | None | No privilege enforcement | Implement privilege checks |
| AC-7 | Unsuccessful Logon Attempts | **N** | **None** | **No lockout mechanism** | **Add `rigrun lockout` with attempt tracking (CRITICAL)** |
| AC-8 | System Use Notification | **C** | Consent banner implemented | None | Verified in consent.go |
| AC-9 | Previous Logon Notification | N | None | No previous session info | Add last login display to status |
| AC-10 | Concurrent Session Control | N | None | No session limits | Add max_concurrent_sessions config |
| AC-11 | Device Lock | P | Session timeout exists | No explicit screen lock | Add TUI screen lock overlay |
| AC-12 | Session Termination | **C** | session.go implements 15-30min timeout | None | DoD STIG AC-12 compliant |
| AC-14 | Permitted Actions Without Identification | N | None | All actions require no auth | Define unauthenticated action list |
| AC-16 | Security and Privacy Attributes | P | Classification markers exist | No attribute binding | Add attribute enforcement |
| AC-17 | Remote Access | N | None | No remote access controls | Add remote access restrictions |
| AC-17(1) | Monitoring/Control | N | None | No remote monitoring | Add remote session monitoring |
| AC-17(2) | Protection of Confidentiality/Integrity | P | TLS used for cloud | No explicit verification | Add TLS certificate pinning |
| AC-18 | Wireless Access | NA | CLI application | N/A | Infrastructure control |
| AC-19 | Access Control for Mobile Devices | NA | Desktop/server app | N/A | Infrastructure control |
| AC-20 | Use of External Systems | P | Paranoid mode blocks external | No external system inventory | Add allowed_external_systems config |
| AC-21 | Information Sharing | N | None | No sharing controls | Add `rigrun export` with sharing rules |
| AC-22 | Publicly Accessible Content | NA | Not public-facing | N/A | Not applicable |
| AC-23 | Data Mining Protection | N | None | No data mining detection | Add query pattern analysis |
| AC-24 | Access Control Decisions | N | None | No access decision logging | Add ACCESS_DECISION event type |
| AC-25 | Reference Monitor | N | None | No reference monitor | Implement security kernel |

---

## AU - Audit and Accountability Family (16 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| AU-1 | Policy and Procedures | N | None | No audit policy doc | Create audit policy template |
| AU-2 | Event Logging | **C** | audit.go logs STARTUP, QUERY, SHUTDOWN, etc. | None | Verified |
| AU-2(3) | Reviews and Updates | N | None | No review process | Add audit review scheduling |
| AU-3 | Content of Audit Records | **C** | Timestamp, event type, session ID, tier, query, tokens, cost, status | None | All required fields present |
| AU-3(1) | Additional Audit Information | P | Basic fields | Missing source IP, user ID | Add source_ip, user_id fields |
| AU-4 | Audit Log Storage Capacity | P | 10MB rotation | No capacity alerts | Add storage capacity monitoring |
| AU-4(1) | Transfer to Alternate Storage | N | None | No alternate storage | Add SIEM export capability |
| AU-5 | Response to Audit Logging Process Failures | **N** | **None** | **No failure handling** | **Add audit failure detection and halt (CRITICAL)** |
| AU-5(1) | Storage Capacity Warning | N | None | No capacity warning | Add storage threshold alerts |
| AU-5(2) | Real-time Alerts | N | None | No real-time alerts | Add alert webhook/email |
| AU-6 | Audit Record Review, Analysis, Reporting | N | None | No review commands | Add `rigrun audit show/analyze` |
| AU-6(1) | Automated Process Integration | N | None | No SIEM integration | Add syslog/SIEM export |
| AU-7 | Audit Record Reduction and Report Generation | N | None | No reporting | Add `rigrun audit report` |
| AU-8 | Time Stamps | **C** | ISO 8601 timestamps with UTC | None | Verified |
| AU-8(1) | Synchronization with Authoritative Time Source | P | Uses system time | No NTP verification | Add NTP sync check |
| AU-9 | Protection of Audit Information | P | 0600 permissions, secret redaction | No integrity verification | Add audit log signing |
| AU-9(2) | Store on Separate Physical Systems | N | None | Local storage only | Add remote audit storage option |
| AU-9(3) | Cryptographic Protection | N | None | Logs not encrypted | Add audit log encryption |
| AU-10 | Non-repudiation | N | None | No digital signatures | Add HMAC/signature verification |
| AU-11 | Audit Record Retention | P | Log rotation preserves old files | No retention policy | Add retention period config |
| AU-12 | Audit Record Generation | **C** | All components log to central audit | None | Verified |
| AU-13 | Monitoring for Information Disclosure | N | None | No disclosure monitoring | Add DLP scanning |
| AU-14 | Session Audit | P | Session events logged | No session replay | Add session audit capture |
| AU-16 | Cross-organizational Audit Logging | N | None | No cross-org capability | Add SIEM export format |

---

## AT - Awareness and Training Family (5 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| AT-1 | Policy and Procedures | NA | Operational control | N/A | Organizational responsibility |
| AT-2 | Literacy Training and Awareness | NA | Operational control | N/A | Organizational responsibility |
| AT-3 | Role-based Training | NA | Operational control | N/A | Organizational responsibility |
| AT-4 | Training Records | NA | Operational control | N/A | Organizational responsibility |
| AT-6 | Training Feedback | NA | Operational control | N/A | Organizational responsibility |

---

## CA - Security Assessment and Authorization Family (9 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| CA-1 | Policy and Procedures | N | None | No CA policy | Create CA policy template |
| CA-2 | Control Assessments | N | None | No self-assessment | Add `rigrun compliance assess` |
| CA-2(1) | Independent Assessors | NA | Operational control | N/A | Organizational responsibility |
| CA-2(2) | Specialized Assessments | N | None | No pen test support | Add security scan capability |
| CA-3 | Information Exchange | P | Network isolation | No ISA verification | Add exchange authorization |
| CA-5 | Plan of Action and Milestones | N | None | No POA&M tracking | Add `rigrun compliance poam` |
| CA-6 | Authorization | NA | Operational control | N/A | Organizational responsibility |
| CA-7 | Continuous Monitoring | P | Audit logging | No monitoring dashboard | Add `rigrun monitor status` |
| CA-7(1) | Independent Assessment | NA | Operational control | N/A | Organizational responsibility |
| CA-8 | Penetration Testing | NA | Operational control | N/A | Organizational responsibility |
| CA-9 | Internal System Connections | N | None | No internal tracking | Add connection inventory |

---

## CM - Configuration Management Family (14 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| CM-1 | Policy and Procedures | N | None | No CM policy | Create CM policy template |
| CM-2 | Baseline Configuration | P | config.go Default() | No baseline diff | Add `rigrun config baseline` |
| CM-2(1) | Reviews and Updates | N | None | No baseline review | Add baseline review scheduling |
| CM-2(2) | Automation Support | P | Config validation | No automated baseline | Add baseline enforcement |
| CM-3 | Configuration Change Control | P | Config changes logged | No approval workflow | Add change approval for critical |
| CM-3(1) | Automated Documentation | P | Config saved to file | No change history | Add config version history |
| CM-4 | Impact Analyses | N | None | No impact analysis | Add `--analyze-impact` flag |
| CM-5 | Access Restrictions for Change | N | None | No role-based config | Restrict config changes to admin |
| CM-6 | Configuration Settings | **C** | Full config system | None | config.go fully implemented |
| CM-7 | Least Functionality | P | Paranoid mode | No service inventory | Add `rigrun services list` |
| CM-7(1) | Periodic Review | N | None | No periodic review | Add scheduled config review |
| CM-8 | System Component Inventory | N | None | No component inventory | Add `rigrun inventory` |
| CM-9 | Configuration Management Plan | N | None | No CMP | Create CMP template |
| CM-10 | Software Usage Restrictions | N | None | No license tracking | Add `rigrun license` |
| CM-11 | User-installed Software | P | Tool sandboxing | No plugin verification | Add plugin signature check |
| CM-12 | Information Location | N | None | No data location tracking | Add data location inventory |
| CM-13 | Data Action Mapping | N | None | No data action tracking | Add data action audit |
| CM-14 | Signed Components | N | None | No binary signing | Add binary signature verification |

---

## CP - Contingency Planning Family (13 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| CP-1 | Policy and Procedures | N | None | No CP policy | Create CP policy template |
| CP-2 | Contingency Plan | N | None | No contingency plan | Create contingency plan template |
| CP-3 | Contingency Training | NA | Operational control | N/A | Organizational responsibility |
| CP-4 | Contingency Plan Testing | N | None | No contingency test | Add `rigrun test contingency` |
| CP-6 | Alternate Storage Site | NA | Infrastructure control | N/A | Infrastructure responsibility |
| CP-7 | Alternate Processing Site | NA | Infrastructure control | N/A | Infrastructure responsibility |
| CP-8 | Telecommunications Services | NA | Infrastructure control | N/A | Infrastructure responsibility |
| CP-9 | System Backup | P | Cache export exists | No full backup | Add `rigrun backup create/restore` |
| CP-9(1) | Testing for Reliability/Integrity | N | None | No backup testing | Add backup verification |
| CP-10 | System Recovery and Reconstitution | N | None | No recovery procedure | Add `rigrun recovery` |
| CP-11 | Alternate Communications Protocols | NA | Infrastructure control | N/A | Infrastructure responsibility |
| CP-12 | Safe Mode | P | Paranoid mode | Not explicitly safe mode | Add `--safe-mode` flag |
| CP-13 | Alternative Security Mechanisms | P | Offline mode available | No mechanism switching | Add failover controls |

---

## IA - Identification and Authentication Family (12 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| IA-1 | Policy and Procedures | N | None | No IA policy | Create IA policy template |
| IA-2 | Identification and Authentication (Org Users) | **N** | **None** | **No user authentication** | **Add `rigrun auth login` (CRITICAL)** |
| IA-2(1) | Multi-factor to Privileged Accounts | N | None | No MFA | Add MFA support |
| IA-2(2) | Multi-factor to Non-privileged Accounts | N | None | No MFA | Add MFA support |
| IA-2(8) | Access to Accounts - Replay Resistant | N | None | No replay protection | Add nonce/timestamp validation |
| IA-3 | Device Identification and Authentication | N | None | No device auth | Add device fingerprinting |
| IA-4 | Identifier Management | P | Session IDs generated | No identifier lifecycle | Add ID management |
| IA-4(4) | Identify User Status | N | None | No user status tracking | Add user status flags |
| IA-5 | Authenticator Management | P | API key storage | No key rotation | Add `rigrun keys rotate` |
| IA-5(1) | Password-based Authentication | N | None | No passwords | Add password support if needed |
| IA-5(2) | PKI-based Authentication | N | None | No PKI | Add certificate auth |
| IA-6 | Authentication Feedback | **C** | API keys masked as **** | None | Verified |
| IA-7 | Cryptographic Module Authentication | **N** | **None** | **No FIPS validation** | **Add `rigrun crypto status` (CRITICAL)** |
| IA-8 | Identification and Authentication (Non-org Users) | N | None | No external user auth | Add external user handling |
| IA-9 | Service Identification and Authentication | N | None | No service auth | Add service-to-service auth |
| IA-10 | Adaptive Authentication | N | None | No adaptive auth | Add risk-based auth |
| IA-11 | Re-authentication | P | Session timeout forces | No privilege re-auth | Add re-auth for sensitive ops |
| IA-12 | Identity Proofing | N | None | No identity proofing | Add identity verification |

---

## IR - Incident Response Family (10 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| IR-1 | Policy and Procedures | N | None | No IR policy | Create IR policy template |
| IR-2 | Incident Response Training | NA | Operational control | N/A | Organizational responsibility |
| IR-3 | Incident Response Testing | NA | Operational control | N/A | Organizational responsibility |
| IR-4 | Incident Handling | P | Audit logging aids investigation | No incident tracking | Add `rigrun incident` command |
| IR-4(1) | Automated Incident Handling | N | None | No automation | Add automated incident creation |
| IR-5 | Incident Monitoring | P | Audit logs | No active monitoring | Add incident monitoring |
| IR-6 | Incident Reporting | **N** | **None** | **No incident reporting** | **Add `rigrun incident report` (CRITICAL)** |
| IR-6(1) | Automated Reporting | N | None | No auto reporting | Add automated reporting |
| IR-7 | Incident Response Assistance | N | None | No IR assistance | Add `rigrun help incident` |
| IR-8 | Incident Response Plan | N | None | No IRP | Create IRP template |
| IR-9 | Information Spillage Response | **N** | **None** | **No spillage response** | **Add `rigrun data sanitize` (CRITICAL)** |

---

## MA - Maintenance Family (6 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| MA-1 | Policy and Procedures | N | None | No MA policy | Create MA policy template |
| MA-2 | Controlled Maintenance | P | Update mechanism exists | No maintenance logging | Add MAINTENANCE event type |
| MA-3 | Maintenance Tools | NA | Software updates only | N/A | Not applicable |
| MA-4 | Nonlocal Maintenance | NA | CLI tool | N/A | Not applicable |
| MA-5 | Maintenance Personnel | NA | Operational control | N/A | Organizational responsibility |
| MA-6 | Timely Maintenance | N | None | No update scheduling | Add update check scheduling |

---

## MP - Media Protection Family (8 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| MP-1 | Policy and Procedures | N | None | No MP policy | Create MP policy template |
| MP-2 | Media Access | P | File permissions | No explicit media control | Add media access config |
| MP-3 | Media Marking | P | Classification in config | No media labeling | Add file classification tags |
| MP-4 | Media Storage | N | None | No encrypted storage | Add encrypted storage option |
| MP-5 | Media Transport | NA | Software does not transport | N/A | Not applicable |
| MP-6 | Media Sanitization | P | Cache clear exists | No secure erase | Add `--secure-erase` flag |
| MP-6(1) | Review/Approve/Track/Document/Verify | N | None | No sanitization tracking | Add sanitization audit |
| MP-7 | Media Use | NA | No removable media | N/A | Not applicable |
| MP-8 | Media Downgrading | NA | Software control | N/A | Infrastructure responsibility |

---

## PE - Physical and Environmental Protection Family (20 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| PE-1 to PE-20 | All PE Controls | NA | Physical controls | N/A | Infrastructure responsibility |

---

## PL - Planning Family (9 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| PL-1 | Policy and Procedures | N | None | No PL policy | Create PL policy template |
| PL-2 | System Security and Privacy Plans | P | Test plan exists | No formal SSP | Create SSP template |
| PL-4 | Rules of Behavior | P | Consent banner mentions rules | No explicit rules | Add rules display |
| PL-4(1) | Social Media and External Site/Application Usage Restrictions | NA | CLI tool | N/A | Not applicable |
| PL-7 | Concept of Operations | N | None | No CONOPS | Create CONOPS template |
| PL-8 | Security and Privacy Architectures | N | None | No architecture doc | Create architecture documentation |
| PL-9 | Central Management | N | None | No central management | Add centralized config |
| PL-10 | Baseline Selection | P | FedRAMP High baseline | No explicit selection | Document baseline selection |
| PL-11 | Baseline Tailoring | P | IL5 tailoring applied | No formal tailoring | Document tailoring rationale |

---

## PM - Program Management Family (16 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| PM-1 to PM-16 | All PM Controls | NA | Organizational controls | N/A | Organizational responsibility |

---

## PS - Personnel Security Family (8 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| PS-1 to PS-8 | All PS Controls | NA | Personnel controls | N/A | Organizational responsibility |

---

## PT - PII Processing and Transparency Family (8 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| PT-1 | Policy and Procedures | N | None | No PII policy | Create PII policy template |
| PT-2 | Authority to Process PII | NA | No PII processed | N/A | Not applicable |
| PT-3 | PII Processing Purposes | NA | No PII processed | N/A | Not applicable |
| PT-4 | Consent | P | Consent banner | Not PII-specific | Add PII consent if needed |
| PT-5 | Privacy Notice | N | None | No privacy notice | Add privacy notice |
| PT-6 | System of Records Notice | NA | No PII processed | N/A | Not applicable |
| PT-7 | Specific Categories of PII | NA | No PII processed | N/A | Not applicable |
| PT-8 | Computer Matching Requirements | NA | No PII processed | N/A | Not applicable |

---

## RA - Risk Assessment Family (6 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| RA-1 | Policy and Procedures | N | None | No RA policy | Create RA policy template |
| RA-2 | Security Categorization | P | Classification exists | No FIPS 199 | Add FIPS 199 categorization |
| RA-3 | Risk Assessment | N | None | No risk assessment | Create risk assessment doc |
| RA-3(1) | Supply Chain Risk Assessment | N | None | No supply chain RA | Add dependency risk check |
| RA-5 | Vulnerability Monitoring and Scanning | **N** | **None** | **No vulnerability scanning** | **Add `rigrun security scan` (HIGH)** |
| RA-5(2) | Update Vulnerabilities to Be Scanned | N | None | No vuln DB updates | Add vuln database |
| RA-5(5) | Privileged Access | N | None | No privileged scan | Add privileged scan mode |
| RA-6 | Technical Surveillance Countermeasures | NA | Physical control | N/A | Infrastructure responsibility |
| RA-7 | Risk Response | N | None | No risk response | Add risk response tracking |
| RA-9 | Criticality Analysis | N | None | No criticality analysis | Add criticality assessment |

---

## SA - System and Services Acquisition Family (22 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| SA-1 | Policy and Procedures | N | None | No SA policy | Create SA policy template |
| SA-2 | Allocation of Resources | NA | Operational control | N/A | Organizational responsibility |
| SA-3 | System Development Life Cycle | NA | Development control | N/A | Development responsibility |
| SA-4 | Acquisition Process | NA | Operational control | N/A | Organizational responsibility |
| SA-5 | System Documentation | P | README exists | No full system doc | Create system documentation |
| SA-8 | Security and Privacy Engineering Principles | P | Security by design | No formal verification | Document security principles |
| SA-9 | External System Services | P | Paranoid mode limits | No service inventory | Add external service inventory |
| SA-9(2) | Identification of Functions/Ports/Protocols/Services | P | Limited to Ollama, OpenRouter | No formal inventory | Add service endpoint inventory |
| SA-10 | Developer Configuration Management | NA | Development control | N/A | Development responsibility |
| SA-11 | Developer Testing and Evaluation | P | Test plan exists | No security testing | Add security test coverage |
| SA-11(1) | Static Code Analysis | N | None | No static analysis | Add go vet, staticcheck |
| SA-11(2) | Threat Modeling | N | None | No threat model | Create threat model |
| SA-15 | Development Process, Standards, Tools | NA | Development control | N/A | Development responsibility |
| SA-16 | Developer-provided Training | NA | Development control | N/A | Development responsibility |
| SA-17 | Developer Security and Privacy Architecture and Design | P | Architecture documented | No formal SADD | Create SADD |
| SA-21 | Developer Screening | NA | Development control | N/A | Development responsibility |
| SA-22 | Unsupported System Components | N | None | No dependency age check | Add `rigrun dependencies audit` |

---

## SC - System and Communications Protection Family (45 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| SC-1 | Policy and Procedures | N | None | No SC policy | Create SC policy template |
| SC-2 | Separation of System and User Functionality | N | None | No separation | Add function separation |
| SC-3 | Security Function Isolation | P | Tool sandboxing | No security kernel | Add security function isolation |
| SC-4 | Information in Shared System Resources | N | None | No shared resource protection | Add `--sanitize` for session end |
| SC-5 | Denial of Service Protection | N | None | No DoS protection | Add rate limiting |
| SC-5(1) | Restrict Ability to Attack Other Systems | P | Paranoid mode | No explicit restriction | Add outbound filtering |
| SC-5(2) | Capacity, Bandwidth, Redundancy | NA | Infrastructure control | N/A | Infrastructure responsibility |
| SC-7 | Boundary Protection | **C** | Paranoid/offline modes, network isolation | None | Verified in offline.go |
| SC-7(3) | Access Points | P | Limited endpoints | No explicit access point control | Add endpoint inventory |
| SC-7(4) | External Telecommunications Services | NA | Uses standard HTTPS | N/A | Infrastructure responsibility |
| SC-7(5) | Deny by Default/Allow by Exception | P | Paranoid mode denies all | Not default behavior | Make deny-by-default optional |
| SC-7(8) | Route Traffic to Authenticated Proxy Servers | N | None | No proxy support | Add proxy configuration |
| SC-8 | Transmission Confidentiality and Integrity | P | TLS for cloud | No explicit verification | Add TLS verification |
| SC-8(1) | Cryptographic Protection | P | TLS used | No FIPS verification | Add FIPS TLS verification |
| SC-10 | Network Disconnect | P | Session timeout | No network-specific timeout | Add network idle timeout |
| SC-12 | Cryptographic Key Establishment and Management | P | API key storage | No key lifecycle | Add key management |
| SC-12(1) | Availability | N | None | No key backup | Add key backup/recovery |
| SC-13 | Cryptographic Protection | **N** | **Standard Go crypto** | **No FIPS-validated modules** | **Add FIPS-validated crypto (CRITICAL)** |
| SC-15 | Collaborative Computing Devices | NA | CLI tool | N/A | Not applicable |
| SC-17 | Public Key Infrastructure Certificates | **N** | **None** | **No PKI certificate validation** | **Add certificate validation (HIGH)** |
| SC-18 | Mobile Code | NA | No mobile code | N/A | Not applicable |
| SC-20 | Secure Name/Address Resolution Service | NA | Uses system DNS | N/A | Infrastructure responsibility |
| SC-21 | Secure Name/Address Resolution Service (Recursive or Caching Resolver) | NA | Uses system DNS | N/A | Infrastructure responsibility |
| SC-22 | Architecture and Provisioning for Name/Address Resolution Service | NA | Uses system DNS | N/A | Infrastructure responsibility |
| SC-23 | Session Authenticity | P | Session IDs | No session token verification | Add session token signing |
| SC-23(1) | Invalidate Session Identifiers at Logout | P | Session ended | No explicit invalidation | Add explicit session invalidation |
| SC-28 | Protection of Information at Rest | **N** | **None** | **No at-rest encryption** | **Add encrypted config/cache storage (CRITICAL)** |
| SC-28(1) | Cryptographic Protection | **N** | **None** | **No encryption** | **Add AES-256 encryption (CRITICAL)** |
| SC-39 | Process Isolation | P | Tool sandboxing | No process namespace | Add process isolation |
| SC-45 | System Time Synchronization | P | Uses system time | No sync verification | Add NTP check |

---

## SI - System and Information Integrity Family (23 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| SI-1 | Policy and Procedures | N | None | No SI policy | Create SI policy template |
| SI-2 | Flaw Remediation | N | None | No vulnerability management | Add `rigrun update check` |
| SI-2(2) | Automated Flaw Remediation Status | N | None | No auto-update status | Add update status tracking |
| SI-3 | Malicious Code Protection | P | Tool sandboxing | No malware scanning | Add command injection prevention |
| SI-3(1) | Central Management | NA | Single system | N/A | Not applicable |
| SI-3(2) | Automatic Updates | N | None | No auto-updates | Add auto-update option |
| SI-4 | System Monitoring | P | Audit logging | No real-time monitoring | Add `rigrun monitor` |
| SI-4(2) | Automated Tools and Mechanisms | P | Audit logs | No automated analysis | Add anomaly detection |
| SI-4(4) | Inbound and Outbound Communications Traffic | P | Network isolation available | No traffic monitoring | Add traffic logging |
| SI-4(5) | System-generated Alerts | N | None | No alert system | Add alert generation |
| SI-5 | Security Alerts, Advisories, Directives | N | None | No advisory awareness | Add security advisory check |
| SI-6 | Security Function Verification | P | Doctor command | No crypto verification | Add `--security-functions` flag |
| SI-7 | Software, Firmware, Information Integrity | **N** | **None** | **No integrity verification** | **Add `rigrun verify --checksum` (CRITICAL)** |
| SI-7(1) | Integrity Checks | **N** | **None** | **No integrity checks** | **Add startup integrity check (CRITICAL)** |
| SI-7(7) | Integration of Detection and Response | N | None | No integrated response | Add integrity violation handling |
| SI-10 | Information Input Validation | P | Some input validation | No injection prevention | Add input sanitization |
| SI-10(1) | Manual Override Capability | P | Manual input allowed | No override tracking | Add override logging |
| SI-11 | Error Handling | **C** | Graceful error handling | None | Verified |
| SI-12 | Information Management and Retention | P | Cache management | No retention policy | Add retention policy |
| SI-16 | Memory Protection | N | None | No memory protection | Add memory protection |
| SI-17 | Fail-safe Procedures | N | None | No fail-safe | Add `--fail-safe` mode |

---

## SR - Supply Chain Risk Management Family (12 Controls)

| ID | Control Name | Status | Current Implementation | Gap | Remediation |
|----|--------------|--------|----------------------|-----|-------------|
| SR-1 | Policy and Procedures | N | None | No SR policy | Create SR policy template |
| SR-2 | Supply Chain Risk Management Plan | N | None | No SCRMP | Create SCRMP template |
| SR-3 | Supply Chain Controls and Processes | N | None | No supply chain controls | Add dependency verification |
| SR-4 | Provenance | N | None | No provenance tracking | Add dependency provenance |
| SR-5 | Acquisition Strategies, Tools, and Methods | NA | Operational control | N/A | Organizational responsibility |
| SR-6 | Supplier Assessments and Reviews | NA | Operational control | N/A | Organizational responsibility |
| SR-7 | Supply Chain Operations Security | NA | Operational control | N/A | Organizational responsibility |
| SR-8 | Notification Agreements | NA | Operational control | N/A | Organizational responsibility |
| SR-9 | Tamper Resistance and Detection | N | None | No tamper detection | Add binary tampering check |
| SR-10 | Inspection of Systems or Components | N | None | No inspection | Add component inspection |
| SR-11 | Component Authenticity | N | None | No authenticity check | Add component verification |
| SR-12 | Component Disposal | NA | Software component | N/A | Infrastructure responsibility |

---

## Critical Implementation Priorities

### PHASE 1: Critical (Must have for IL5) - 10 Items

| Priority | Control | Implementation | Effort |
|----------|---------|---------------|--------|
| 1 | AC-7 | Account lockout after 3 failed attempts | 2 days |
| 2 | AU-5 | Halt on audit failure, alert mechanism | 2 days |
| 3 | SC-13 | FIPS 140-2 validated crypto verification | 3 days |
| 4 | SC-28 | AES-256 encryption for config/cache/audit | 5 days |
| 5 | SI-7 | Binary and config integrity verification | 3 days |
| 6 | IR-6 | Incident reporting command | 2 days |
| 7 | IR-9 | Data spillage sanitization | 2 days |
| 8 | IA-2 | User authentication system | 5 days |
| 9 | IA-7 | FIPS crypto module status check | 2 days |
| 10 | SC-17 | PKI certificate validation | 2 days |

**Phase 1 Total: ~28 days**

### PHASE 2: High Priority - 15 Items

| Priority | Control | Implementation | Effort |
|----------|---------|---------------|--------|
| 1 | AC-5 | Role separation (admin/operator/auditor) | 3 days |
| 2 | AC-6 | Least privilege enforcement | 2 days |
| 3 | AU-6 | Audit review and analysis commands | 2 days |
| 4 | AU-9 | Audit log signing (HMAC) | 2 days |
| 5 | AU-10 | Non-repudiation signatures | 2 days |
| 6 | CM-5 | Config change restrictions | 2 days |
| 7 | CM-8 | Component inventory | 1 day |
| 8 | CM-14 | Binary signature verification | 2 days |
| 9 | CP-9 | Full backup and restore | 3 days |
| 10 | CP-10 | Recovery procedures | 2 days |
| 11 | RA-5 | Vulnerability scanning | 3 days |
| 12 | SA-22 | Dependency audit | 2 days |
| 13 | SC-5 | Rate limiting/DoS protection | 2 days |
| 14 | SI-2 | Update check and flaw remediation | 2 days |
| 15 | SI-4 | System monitoring dashboard | 3 days |

**Phase 2 Total: ~35 days**

### PHASE 3: Medium Priority - 20 Items

| Priority | Control | Implementation | Effort |
|----------|---------|---------------|--------|
| 1 | AC-9 | Previous logon notification | 1 day |
| 2 | AC-10 | Concurrent session limits | 1 day |
| 3 | AC-21 | Information sharing rules | 2 days |
| 4 | AU-4 | Storage capacity alerts | 1 day |
| 5 | AU-7 | Audit report generation | 2 days |
| 6 | AU-16 | SIEM/syslog export | 2 days |
| 7 | CA-2 | Self-assessment capability | 3 days |
| 8 | CA-7 | Continuous monitoring | 2 days |
| 9 | CM-2 | Baseline configuration diff | 2 days |
| 10 | CM-4 | Impact analysis | 1 day |
| 11 | IA-5 | Key rotation | 2 days |
| 12 | IA-11 | Re-authentication for sensitive ops | 1 day |
| 13 | MP-6 | Secure erase verification | 1 day |
| 14 | PL-2 | System Security Plan template | 2 days |
| 15 | RA-2 | FIPS 199 categorization | 1 day |
| 16 | SC-4 | Shared resource sanitization | 1 day |
| 17 | SC-23 | Session token signing | 2 days |
| 18 | SI-5 | Security advisory integration | 2 days |
| 19 | SI-10 | Input sanitization/injection prevention | 2 days |
| 20 | All -1 | Policy document templates | 5 days |

**Phase 3 Total: ~37 days**

---

## Implementation Commands Needed

### Critical Commands (Phase 1)

```bash
# AC-7: Account lockout
rigrun lockout status
rigrun lockout reset <user>

# AU-5: Audit failure handling
rigrun audit verify --integrity
rigrun audit alert --configure

# SC-13/IA-7: FIPS crypto
rigrun crypto status
rigrun crypto verify --fips

# SC-28: Encryption
rigrun encrypt config
rigrun encrypt cache
rigrun encrypt audit

# SI-7: Integrity
rigrun verify --checksum
rigrun verify --signature
rigrun verify --all

# IR-6: Incident reporting
rigrun incident report --severity <level> --description "<desc>"
rigrun incident list
rigrun incident status <id>

# IR-9: Spillage
rigrun data sanitize --spillage-response
rigrun data wipe --secure

# IA-2: Authentication
rigrun auth login
rigrun auth logout
rigrun auth status
rigrun auth mfa enable
```

### High Priority Commands (Phase 2)

```bash
# AC-5/AC-6: RBAC
rigrun roles list
rigrun roles assign <user> <role>
rigrun permissions show

# AU-6: Audit review
rigrun audit show --last 24h
rigrun audit analyze --anomalies
rigrun audit search --event <type>

# CM-8: Inventory
rigrun inventory show
rigrun inventory components
rigrun inventory dependencies

# CP-9/CP-10: Backup
rigrun backup create --full
rigrun backup restore <backup-id>
rigrun backup verify <backup-id>
rigrun recovery status

# RA-5: Scanning
rigrun security scan
rigrun security vulnerabilities
rigrun doctor --security

# SA-22: Dependencies
rigrun dependencies audit
rigrun dependencies outdated
rigrun dependencies vulnerabilities
```

---

## Files to Create/Modify

### New Files Required

```
internal/security/
  ├── lockout.go          # AC-7: Account lockout
  ├── auth.go             # IA-2: Authentication
  ├── rbac.go             # AC-5/AC-6: Role-based access
  ├── crypto.go           # SC-13: FIPS crypto
  ├── encrypt.go          # SC-28: At-rest encryption
  ├── integrity.go        # SI-7: Integrity verification
  ├── incident.go         # IR-6: Incident reporting
  ├── spillage.go         # IR-9: Spillage response

internal/compliance/
  ├── scanner.go          # RA-5: Vulnerability scanning
  ├── assessment.go       # CA-2: Self-assessment
  ├── monitor.go          # CA-7/SI-4: Monitoring

internal/backup/
  ├── backup.go           # CP-9: Backup
  ├── restore.go          # CP-10: Recovery

internal/cli/
  ├── lockout_cmd.go      # AC-7 CLI
  ├── auth_cmd.go         # IA-2 CLI
  ├── crypto_cmd.go       # SC-13 CLI
  ├── encrypt_cmd.go      # SC-28 CLI
  ├── verify_cmd.go       # SI-7 CLI
  ├── incident_cmd.go     # IR-6 CLI
  ├── backup_cmd.go       # CP-9/CP-10 CLI
  ├── scan_cmd.go         # RA-5 CLI
  ├── roles_cmd.go        # AC-5/AC-6 CLI
```

### Existing Files to Modify

```
internal/security/audit.go    # Add AU-5 failure handling, AU-9 signing
internal/config/config.go     # Add encryption, RBAC config
internal/cli/cli.go           # Add new command registration
```

---

## Test Coverage Requirements

Each control implementation must include:

1. **Unit tests** - Function-level testing
2. **Integration tests** - End-to-end command testing
3. **Compliance tests** - NIST control verification
4. **Security tests** - Attack scenario testing

### Test File Structure

```
tests/
  ├── compliance/
  │   ├── ac_test.go          # Access Control tests
  │   ├── au_test.go          # Audit tests
  │   ├── ia_test.go          # Authentication tests
  │   ├── ir_test.go          # Incident Response tests
  │   ├── sc_test.go          # System/Comm Protection tests
  │   ├── si_test.go          # System Integrity tests
  │   └── full_compliance_test.go  # Complete IL5 validation
```

---

## Success Criteria

IL5 Authorization requires:

- [ ] All CRITICAL controls implemented and tested
- [ ] All HIGH priority controls implemented and tested
- [ ] Policy documentation for all control families
- [ ] Automated compliance testing suite
- [ ] Security assessment report
- [ ] Plan of Action and Milestones (POA&M) for remaining gaps

---

**Document Prepared By:** Compliance Audit System
**Review Required By:** Security Assessment Team
**Next Review Date:** 2026-02-20

---

**END OF COMPLIANCE MATRIX**
