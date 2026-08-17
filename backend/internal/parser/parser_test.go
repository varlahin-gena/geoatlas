package parser

import (
	"strings"
	"testing"
	"time"

	"network_monitor/internal/model"
)

// ============================================================
// Хелперы
// ============================================================

type tlWant struct {
	vendor, src, dst              string
	sport, dport                  uint32
	action, proto, rule           string
	szone, dzone, scountry, dcty  string
	device                        string
	bytesSent, bytesRecv          uint64
	pktSent, pktRecv              uint64
}

func checkTL(t *testing.T, name string, got model.TrafficLog, w tlWant) {
	t.Helper()
	eqS := func(field, g, want string) {
		if g != want {
			t.Errorf("%s: %s = %q, want %q", name, field, g, want)
		}
	}
	eqU32 := func(field string, g, want uint32) {
		if g != want {
			t.Errorf("%s: %s = %d, want %d", name, field, g, want)
		}
	}
	eqU64 := func(field string, g, want uint64) {
		if g != want {
			t.Errorf("%s: %s = %d, want %d", name, field, g, want)
		}
	}
	eqS("vendor", got.Vendor, w.vendor)
	eqS("src", got.SrcIP, w.src)
	eqS("dst", got.DstIP, w.dst)
	eqU32("sport", got.SrcPort, w.sport)
	eqU32("dport", got.DstPort, w.dport)
	eqS("action", got.Action, w.action)
	eqS("proto", got.Proto, w.proto)
	eqS("rule", got.Rule, w.rule)
	eqS("src_zone", got.SrcZone, w.szone)
	eqS("dst_zone", got.DstZone, w.dzone)
	eqS("src_country", got.SrcCountry, w.scountry)
	eqS("dst_country", got.DstCountry, w.dcty)
	eqS("device", got.Device, w.device)
	eqU64("bytes_sent", got.BytesSent, w.bytesSent)
	eqU64("bytes_recv", got.BytesRecv, w.bytesRecv)
	eqU64("packets_sent", got.PacketsSent, w.pktSent)
	eqU64("packets_recv", got.PacketsRecv, w.pktRecv)
}

// testRegistry — тот же порядок парсеров, что и в cmd/network-monitor/main.go.
func testRegistry() *Registry {
	return NewRegistry(
		&UserGateCEF{},
		&FortigateCEF{},
		&CiscoFTD{},
		&CiscoASA{},
		&CowrieJSON{},
		&GenericKV{},
	)
}

// ============================================================
// Утилиты нормализации
// ============================================================

func TestNormalizeAction(t *testing.T) {
	if got := NormalizeAction("  ALLOW "); string(got) != "allow" {
		t.Errorf("NormalizeAction(ALLOW) = %q, want allow", got)
	}
	if got := NormalizeAction("Deny"); string(got) != "deny" {
		t.Errorf("NormalizeAction(Deny) = %q, want deny", got)
	}
	if got := NormalizeAction(""); got != model.ActionUnknown {
		t.Errorf("NormalizeAction(\"\") = %q, want ActionUnknown", got)
	}
}

func TestIsIPv4(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"10.0.0.1", true},
		{" 8.8.8.8 ", true},
		{"2001:db8::1", false},
		{"::1", false},
		{"not-an-ip", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsIPv4(tc.in); got != tc.want {
			t.Errorf("IsIPv4(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestContainsIPv4(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"mystery from 10.0.0.1", true},
		{"no addresses here", false},
		{"only IPv6 2001:db8::1", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := ContainsIPv4(tc.in); got != tc.want {
			t.Errorf("ContainsIPv4(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseVerboseSkipsNonIPv4Pair(t *testing.T) {
	r := testRegistry()
	line := `src=2001:db8::1 dst=2001:db8::2 action=allow proto=tcp sport=12345 dport=443`
	res := r.ParseVerbose(line)
	if !res.Skipped {
		t.Fatalf("ожидался Skipped, got OK=%v reason=%q vendor=%q", res.OK, res.Reason, res.Vendor)
	}
	if res.OK {
		t.Fatal("OK = true, want false")
	}
	if !strings.Contains(res.Reason, "IPv4") {
		t.Errorf("reason = %q, want mention of IPv4", res.Reason)
	}

	// Смешанная пара (IPv4 + IPv6) тоже отбрасывается.
	mixed := `src=10.0.0.1 dst=2001:db8::2 action=deny proto=tcp sport=1 dport=80`
	res = r.ParseVerbose(mixed)
	if !res.Skipped {
		t.Fatalf("mixed: ожидался Skipped, got OK=%v reason=%q", res.OK, res.Reason)
	}
}

func TestSampleCorpus(t *testing.T) {
	r := testRegistry()
	for _, s := range Samples() {
		s := s
		t.Run(s.Vendor+"/"+s.Desc, func(t *testing.T) {
			res := r.ParseVerbose(s.Line)
			switch {
			case s.Skip:
				if !res.Skipped {
					t.Errorf("ожидался Skipped, got OK=%v reason=%q", res.OK, res.Reason)
				}
			case !res.OK:
				t.Errorf("не разобралось: %s", res.Reason)
			}
			if res.Vendor != s.Vendor {
				t.Errorf("vendor=%q, want %q", res.Vendor, s.Vendor)
			}
		})
	}
}

func TestMapProtoNumber(t *testing.T) {
	cases := map[string]string{
		"1": "ICMP", "6": "TCP", "17": "UDP", "47": "GRE",
		"50": "ESP", "51": "AH", "": "", "tcp": "TCP",
	}
	for in, want := range cases {
		if got := mapProtoNumber(in); got != want {
			t.Errorf("mapProtoNumber(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseSyslogTime(t *testing.T) {
	// ISO 8601 (детерминированный UTC)
	if got, ok := parseSyslogTime("2023-11-14T22:13:20Z fw01"); !ok {
		t.Error("parseSyslogTime(ISO): ok = false")
	} else if got.UTC().Format(time.RFC3339) != "2023-11-14T22:13:20Z" {
		t.Errorf("parseSyslogTime(ISO) = %s", got.UTC().Format(time.RFC3339))
	}

	// BSD с годом
	if got, ok := parseSyslogTime("Nov 14 2023 22:13:20 fw01"); !ok {
		t.Error("parseSyslogTime(BSD+year): ok = false")
	} else if got.Year() != 2023 || got.Month() != time.November || got.Day() != 14 {
		t.Errorf("parseSyslogTime(BSD+year) = %v", got)
	}

	// BSD без года — год текущий, проверяем месяц/день
	if got, ok := parseSyslogTime("Jun 30 18:50:43 host"); !ok {
		t.Error("parseSyslogTime(BSD): ok = false")
	} else if got.Month() != time.June || got.Day() != 30 {
		t.Errorf("parseSyslogTime(BSD) = %v", got)
	}

	if _, ok := parseSyslogTime("no time here"); ok {
		t.Error("parseSyslogTime(garbage): ok = true, want false")
	}
}

// ============================================================
// CEF: UserGate (формат из docs.usergate.com)
// ============================================================

func TestUserGateCEF(t *testing.T) {
	line := `CEF:0|Usergate|UTM|6|traffic|firewall|1|` +
		`rt=1652344423822 deviceExternalId=utmcore@ersthetatica suser=user_example ` +
		`act=accept cs1Label=Rule cs1=Allow trusted to untrusted ` +
		`src=10.10.10.10 spt=54321 cs2Label=Source Zone cs2=Trusted ` +
		`cs3Label=Source Country cs3=RU proto=TCP ` +
		`dst=194.226.127.130 dpt=443 cs4Label=Destination Zone cs4=Untrusted ` +
		`cs5Label=Destination Country cs5=US in=231 out=40 ` +
		`cn1Label=Packets sent cn1=3 cn2Label=Packets received cn2=1`

	p := &UserGateCEF{}
	if !p.CanParse(line) {
		t.Fatal("UserGate.CanParse = false")
	}
	got, ok := p.Parse(line)
	if !ok {
		t.Fatal("UserGate.Parse = false")
	}
	checkTL(t, "usergate", got, tlWant{
		vendor: "usergate", src: "10.10.10.10", dst: "194.226.127.130",
		sport: 54321, dport: 443, action: "accept", proto: "TCP",
		rule: "Allow trusted to untrusted", szone: "Trusted", dzone: "Untrusted",
		scountry: "RU", dcty: "US", device: "utmcore@ersthetatica",
		// UserGate: in=sent (src→dst), out=recv (dst→src)
		bytesSent: 231, bytesRecv: 40, pktSent: 3, pktRecv: 1,
	})
	if got.Timestamp.UnixMilli() != 1652344423822 {
		t.Errorf("usergate ts = %d, want 1652344423822", got.Timestamp.UnixMilli())
	}
}

// ============================================================
// CEF: Fortigate (официальный пример Fortinet Log Message Reference)
// ============================================================

func TestFortigateCEF(t *testing.T) {
	line := `Dec 27 11:07:55 FGT-A-LOG CEF: 0|Fortinet|Fortigate|v6.0.3|00013|traffic:forward close|3|` +
		`deviceExternalId=FGT5HD3915800610 FTNTFGTlogid=0000000013 cat=traffic:forward ` +
		`FTNTFGTsubtype=forward FTNTFGTlevel=notice FTNTFGTvd=vdom1 FTNTFGTeventtime=1545937675 ` +
		`src=10.1.100.11 spt=54190 deviceInboundInterface=port12 FTNTFGTsrcintfrole=undefined ` +
		`dst=52.53.140.235 dpt=443 deviceOutboundInterface=port11 FTNTFGTdstintfrole=undefined ` +
		`proto=6 act=close FTNTFGTpolicyid=1 app=HTTPS ` +
		`FTNTFGTdstcountry=United States FTNTFGTsrccountry=Reserved ` +
		`out=3652 in=146668 FTNTFGTsentpkt=58 FTNTFGTrcvdpkt=105`

	p := &FortigateCEF{}
	if !p.CanParse(line) {
		t.Fatal("Fortigate.CanParse = false")
	}
	got, ok := p.Parse(line)
	if !ok {
		t.Fatal("Fortigate.Parse = false")
	}
	checkTL(t, "fortigate", got, tlWant{
		vendor: "fortigate", src: "10.1.100.11", dst: "52.53.140.235",
		sport: 54190, dport: 443, action: "close", proto: "TCP",
		rule: "HTTPS (policy 1)", szone: "port12", dzone: "port11",
		scountry: "Reserved", dcty: "United States", device: "FGT5HD3915800610",
		bytesSent: 3652, bytesRecv: 146668, pktSent: 58, pktRecv: 105,
	})
	if got.Timestamp.Unix() != 1545937675 {
		t.Errorf("fortigate ts = %d, want 1545937675", got.Timestamp.Unix())
	}
}

// ============================================================
// Cisco ASA
// ============================================================

func TestCiscoASA(t *testing.T) {
	p := &CiscoASA{}
	cases := []struct {
		name string
		line string
		want tlWant
	}{
		{
			// Cisco FAQ 116149 scenario 2: inside host → outside host
			name: "302013 built outbound tcp",
			line: `%ASA-6-302013: Built outbound TCP connection 9 for outside:10.1.2.1/22 (10.1.2.1/22) to inside:10.1.1.2/53496 (10.1.1.2/53496)`,
			want: tlWant{vendor: "cisco-asa", src: "10.1.1.2", dst: "10.1.2.1",
				sport: 53496, dport: 22, action: "allow", proto: "TCP",
				szone: "inside", dzone: "outside"},
		},
		{
			// Cisco FAQ 116149 scenario 4: outside host → inside host
			name: "302013 built inbound tcp",
			line: `%ASA-6-302013: Built inbound TCP connection 11 for outside:10.1.2.1/21647 (10.1.2.1/21647) to inside:10.1.1.2/22 (10.1.1.2/22)`,
			want: tlWant{vendor: "cisco-asa", src: "10.1.2.1", dst: "10.1.1.2",
				sport: 21647, dport: 22, action: "allow", proto: "TCP",
				szone: "outside", dzone: "inside"},
		},
		{
			name: "302014 teardown tcp",
			line: `%ASA-6-302014: Teardown TCP connection 9 for outside:10.1.2.1/22 to inside:10.1.1.2/53496 duration 0:00:30 bytes 2436 TCP FINs`,
			want: tlWant{vendor: "cisco-asa", src: "10.1.2.1", dst: "10.1.1.2",
				sport: 22, dport: 53496, action: "allow", proto: "TCP",
				szone: "outside", dzone: "inside", bytesSent: 2436},
		},
		{
			name: "302020 built icmp",
			line: `%ASA-6-302020: Built inbound ICMP connection for faddr 203.0.113.9/0 gaddr 10.0.0.1/0 laddr 10.0.0.5/0`,
			want: tlWant{vendor: "cisco-asa", src: "10.0.0.5", dst: "203.0.113.9",
				action: "allow", proto: "ICMP"},
		},
		{
			name: "106023 deny by access-group",
			line: `%ASA-4-106023: Deny tcp src outside:192.168.0.50/21979 dst inside:192.168.0.49/23 by access-group "acl_outside" [0x0, 0x0]`,
			want: tlWant{vendor: "cisco-asa", src: "192.168.0.50", dst: "192.168.0.49",
				sport: 21979, dport: 23, action: "deny", proto: "TCP",
				rule: "acl_outside", szone: "outside", dzone: "inside"},
		},
		{
			name: "106100 acl permitted",
			line: `%ASA-6-106100: access-list outside-acl permitted tcp outside/1.1.1.1(12345) -> inside/192.168.1.1(1357) hit-cnt 1 (first hit)`,
			want: tlWant{vendor: "cisco-asa", src: "1.1.1.1", dst: "192.168.1.1",
				sport: 12345, dport: 1357, action: "allow", proto: "TCP",
				rule: "outside-acl", szone: "outside", dzone: "inside"},
		},
		{
			name: "106015 deny tcp",
			line: `%ASA-6-106015: Deny TCP (no connection) from 203.0.113.77/31337 to 10.0.0.9/443 flags SYN ACK on interface outside`,
			want: tlWant{vendor: "cisco-asa", src: "203.0.113.77", dst: "10.0.0.9",
				sport: 31337, dport: 443, action: "deny", proto: "TCP"},
		},
		{
			name: "106001 inbound tcp denied",
			line: `%ASA-2-106001: Inbound TCP connection denied from 203.0.113.66/12345 to 10.0.0.7/80 flags SYN on interface outside`,
			want: tlWant{vendor: "cisco-asa", src: "203.0.113.66", dst: "10.0.0.7",
				sport: 12345, dport: 80, action: "deny", proto: "TCP"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !p.CanParse(c.line) {
				t.Fatal("CanParse = false")
			}
			got, ok := p.Parse(c.line)
			if !ok {
				t.Fatal("Parse = false")
			}
			checkTL(t, c.name, got, c.want)
		})
	}
}

// ============================================================
// Cisco FTD
// ============================================================

func TestCiscoFTD(t *testing.T) {
	p := &CiscoFTD{}

	t.Run("313009 denied icmp", func(t *testing.T) {
		line := `%FTD-4-313009: Denied ICMP type=8, for inside:10.0.0.5/0 (10.0.0.5/0) to outside:203.0.113.5/0`
		if !p.CanParse(line) {
			t.Fatal("CanParse = false")
		}
		got, ok := p.Parse(line)
		if !ok {
			t.Fatal("Parse = false")
		}
		checkTL(t, "ftd-313009", got, tlWant{
			vendor: "cisco-ftd", src: "10.0.0.5", dst: "203.0.113.5",
			action: "deny", proto: "ICMP", szone: "inside", dzone: "outside",
		})
	})

	t.Run("430002 connection start", func(t *testing.T) {
		line := `%FTD-1-430002: EventPriority: Low, DeviceUUID: b2433c5c-a6a1-11eb-a6e7-be0b9833091f, ` +
			`InstanceID: 2, FirstPacketSecond: 2021-04-30T11:31:19Z, ConnectionID: 4, ` +
			`AccessControlRuleAction: Allow, SrcIP: 172.16.10.10, DstIP: 172.16.20.10, ` +
			`ICMPType: Echo Request, ICMPCode: No Code, Protocol: icmp, ` +
			`IngressInterface: inside, EgressInterface: outside, ` +
			`ACPolicy: Default Allow All Traffic, AccessControlRuleName: test, ` +
			`InitiatorPackets: 1, ResponderPackets: 0, InitiatorBytes: 74, ResponderBytes: 0`
		if !p.CanParse(line) {
			t.Fatal("CanParse = false")
		}
		got, ok := p.Parse(line)
		if !ok {
			t.Fatal("Parse = false")
		}
		checkTL(t, "ftd-430002", got, tlWant{
			vendor: "cisco-ftd", src: "172.16.10.10", dst: "172.16.20.10",
			action: "allow", proto: "ICMP", rule: "test",
			szone: "inside", dzone: "outside",
			bytesSent: 74, bytesRecv: 0, pktSent: 1, pktRecv: 0,
		})
		if got.Timestamp.UTC().Format(time.RFC3339) != "2021-04-30T11:31:19Z" {
			t.Errorf("ftd-430002 ts = %s", got.Timestamp.UTC().Format(time.RFC3339))
		}
	})

	t.Run("430003 without EventPriority", func(t *testing.T) {
		line := `%FTD-6-430003: AccessControlRuleAction: Allow, SrcIP: 10.101.11.21, DstIP: 10.178.219.10, ` +
			`SrcPort: 46915, DstPort: 391, Protocol: udp, IngressZone: Inside, EgressZone: R7_Outside, ` +
			`AccessControlRuleName: Allow_All_Outbound, InitiatorPackets: 1, ResponderPackets: 0, ` +
			`InitiatorBytes: 131, ResponderBytes: 0`
		if !p.CanParse(line) {
			t.Fatal("CanParse = false")
		}
		got, ok := p.Parse(line)
		if !ok {
			t.Fatal("Parse = false")
		}
		checkTL(t, "ftd-430003", got, tlWant{
			vendor: "cisco-ftd", src: "10.101.11.21", dst: "10.178.219.10",
			sport: 46915, dport: 391, action: "allow", proto: "UDP",
			rule: "Allow_All_Outbound", szone: "Inside", dzone: "R7_Outside",
			bytesSent: 131, bytesRecv: 0, pktSent: 1, pktRecv: 0,
		})
	})

	t.Run("lina reuse 302013 outbound", func(t *testing.T) {
		line := `%FTD-6-302013: Built outbound TCP connection 999 for outside:8.8.8.8/53 (8.8.8.8/53) to inside:10.0.0.20/40000 (10.0.0.20/40000)`
		if !p.CanParse(line) {
			t.Fatal("CanParse = false")
		}
		got, ok := p.Parse(line)
		if !ok {
			t.Fatal("Parse = false")
		}
		checkTL(t, "ftd-lina", got, tlWant{
			vendor: "cisco-ftd", src: "10.0.0.20", dst: "8.8.8.8",
			sport: 40000, dport: 53, action: "allow", proto: "TCP",
			szone: "inside", dzone: "outside",
		})
	})
}


// ============================================================
// Cowrie: строгий JSON и Python-repr + skip
// ============================================================

func TestCowrie(t *testing.T) {
	p := &CowrieJSON{}

	t.Run("json connect", func(t *testing.T) {
		line := `{"eventid":"cowrie.session.connect","src_ip":"203.0.113.200","dst_ip":"10.0.0.50","src_port":54321,"dst_port":22,"protocol":"ssh","sensor":"honeypot1","timestamp":"2023-11-14T22:13:20.123456Z"}`
		if !p.CanParse(line) {
			t.Fatal("CanParse = false")
		}
		got, ok := p.Parse(line)
		if !ok {
			t.Fatal("Parse = false")
		}
		checkTL(t, "cowrie-json", got, tlWant{
			vendor: "cowrie", src: "203.0.113.200", dst: "10.0.0.50",
			sport: 54321, dport: 22, action: "accept", proto: "TCP",
			rule: "ssh", device: "honeypot1",
		})
	})

	t.Run("python-repr login.failed", func(t *testing.T) {
		line := `Jun 30 18:50:43 172.18.0.1 {'eventid': 'cowrie.login.failed', 'src_ip': '203.0.113.201', 'dst_ip': '10.0.0.51', 'src_port': 40000, 'dst_port': 22, 'protocol': 'telnet', 'sensor': 'hp2', 'username': 'root'}`
		got, ok := p.Parse(line)
		if !ok {
			t.Fatal("Parse = false")
		}
		checkTL(t, "cowrie-repr", got, tlWant{
			vendor: "cowrie", src: "203.0.113.201", dst: "10.0.0.51",
			sport: 40000, dport: 22, action: "reject", proto: "TCP",
			rule: "telnet", device: "hp2",
		})
	})

	t.Run("skip without net pair", func(t *testing.T) {
		line := `{"eventid":"cowrie.command.input","src_ip":"203.0.113.202","input":"ls -la"}`
		if _, ok := p.Parse(line); ok {
			t.Error("Parse = true, want false (нет сетевой пары)")
		}
		if !p.ShouldSkip(line) {
			t.Error("ShouldSkip = false, want true")
		}
	})

	// direct-tcpip содержит src+dst, но dst — цель прокси, не honeypot.
	t.Run("skip direct-tcpip.request", func(t *testing.T) {
		line := `{"eventid":"cowrie.direct-tcpip.request","dst_ip":"77.88.21.158","dst_port":25,"src_ip":"203.0.113.200","src_port":19453,"protocol":"ssh","sensor":"hp1","timestamp":"2026-07-19T00:00:35.207887Z"}`
		if !p.CanParse(line) {
			t.Fatal("CanParse = false")
		}
		if _, ok := p.Parse(line); ok {
			t.Error("Parse = true, want false (direct-tcpip не в граф)")
		}
		if !p.ShouldSkip(line) {
			t.Error("ShouldSkip = false, want true")
		}
	})

	t.Run("real jsonlog session.connect", func(t *testing.T) {
		line := `{"eventid":"cowrie.session.connect","src_ip":"117.211.15.93","src_port":38540,"dst_ip":"155.212.245.143","dst_port":2222,"session":"190c85024af6","protocol":"ssh","message":"New connection: 117.211.15.93:38540 (155.212.245.143:2222) [session: 190c85024af6]","sensor":"197616tg.com","uuid":"f2768fa8-74b1-11f1-ad9d-00163cba124e","timestamp":"2026-07-19T00:00:32.159611Z"}`
		got, ok := p.Parse(line)
		if !ok {
			t.Fatal("Parse = false")
		}
		checkTL(t, "cowrie-real-json", got, tlWant{
			vendor: "cowrie", src: "117.211.15.93", dst: "155.212.245.143",
			sport: 38540, dport: 2222, action: "accept", proto: "TCP",
			rule: "ssh", device: "197616tg.com",
		})
	})
}

// ============================================================
// GenericKV
// ============================================================

func TestGenericKV(t *testing.T) {
	line := `date=2023-11-14 src=192.0.2.55 dst=198.51.100.66 spt=12345 dpt=8080 act=deny proto=udp rule=BlockRule out=100 in=200 cn1=2 cn2=3`
	p := &GenericKV{}
	if !p.CanParse(line) {
		t.Fatal("CanParse = false")
	}
	got, ok := p.Parse(line)
	if !ok {
		t.Fatal("Parse = false")
	}
	checkTL(t, "generic", got, tlWant{
		vendor: "generic", src: "192.0.2.55", dst: "198.51.100.66",
		sport: 12345, dport: 8080, action: "deny", proto: "UDP",
		rule: "BlockRule", bytesSent: 100, bytesRecv: 200, pktSent: 2, pktRecv: 3,
	})
}

// ============================================================
// ShouldSkip: эвристика "меньше двух IPv4 → не трафик"
// ============================================================

func TestCiscoShouldSkip(t *testing.T) {
	asa := &CiscoASA{}

	// Админ-событие без IP — пропуск.
	if !asa.ShouldSkip(`%ASA-5-111008: User 'admin' executed the 'enable' command`) {
		t.Error("ShouldSkip(no-ip) = false, want true")
	}
	// Неизвестный код, но с парой IP — НЕ пропуск (должно попасть в parse_errors).
	if asa.ShouldSkip(`%ASA-6-999999: mystery event from 1.2.3.4 to 5.6.7.8`) {
		t.Error("ShouldSkip(2 ips) = true, want false")
	}
}
