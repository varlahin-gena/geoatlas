package mapsearch

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Field string

const (
	FieldAll        Field = "all"
	FieldIP         Field = "ip"
	FieldPort       Field = "port"
	FieldCountry    Field = "country"
	FieldCity       Field = "city"
	FieldAction     Field = "action"
	FieldDevice     Field = "device"
	FieldSrcIP      Field = "src_ip"
	FieldDstIP      Field = "dst_ip"
	FieldSrcPort    Field = "src_port"
	FieldDstPort    Field = "dst_port"
	FieldSrcCountry Field = "src_country"
	FieldDstCountry Field = "dst_country"
	FieldSrcCity    Field = "src_city"
	FieldDstCity    Field = "dst_city"
	FieldSrc        Field = "src"
	FieldDst        Field = "dst"
	FieldProto      Field = "proto"
	FieldZone       Field = "zone"
)

type Kind int

const (
	KindTerm Kind = iota
	KindNot
	KindAnd
	KindOr
)

type Op int

const (
	OpContains Op = iota
	OpEq
	OpNe
)

type Node struct {
	Kind  Kind
	Field Field
	Op    Op
	Value string
	Left  *Node
	Right *Node
	Expr  *Node
}

type Compiled struct {
	Empty  bool
	Simple string
	Root   *Node
}

var fieldAliases = map[string]Field{
	"all": "all", "any": "all", "text": "all",
	"ip": "ip", "addr": "ip", "address": "ip",
	"port": "port",
	"country": "country", "страна": "country",
	"city": "city", "город": "city",
	"action": "action", "act": "action", "действие": "action",
	"rule": "action", "policy": "action", "правило": "action",
	"device": "device", "host": "device", "fw": "device", "устройство": "device", "мсэ": "device",
	"src_ip": "src_ip", "attacker": "src_ip", "attacker_ip": "src_ip", "атакующий": "src_ip",
	"dst_ip": "dst_ip", "target": "dst_ip", "target_ip": "dst_ip", "цель": "dst_ip",
	"src_port": "src_port", "spt": "src_port", "attacker_port": "src_port",
	"dst_port": "dst_port", "dpt": "dst_port", "target_port": "dst_port",
	"src_country": "src_country", "attacker_country": "src_country",
	"dst_country": "dst_country", "target_country": "dst_country",
	"src_city": "src_city", "attacker_city": "src_city",
	"dst_city": "dst_city", "target_city": "dst_city",
	"src": "src", "source": "src", "from": "src",
	"dst": "dst", "dest": "dst", "destination": "dst", "to": "dst",
	"proto": "proto", "protocol": "proto",
	"zone": "zone",
}

type tokenType int

const (
	tokWord tokenType = iota
	tokString
	tokColon
	tokEq
	tokNe
	tokLParen
	tokRParen
	tokAnd
	tokOr
	tokNot
)

type token struct {
	typ   tokenType
	value string
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func looksAdvanced(raw string) bool {
	for _, r := range raw {
		if r == '(' || r == ')' || r == '"' || r == '=' {
			return true
		}
	}
	upper := strings.ToUpper(raw)
	if hasWord(upper, "AND") || hasWord(upper, "OR") || hasWord(upper, "NOT") || hasWord(upper, "CONTAINS") {
		return true
	}
	if strings.Contains(raw, "!=") {
		return true
	}
	return hasFieldColon(raw)
}

func hasWord(upper, word string) bool {
	i := 0
	for i <= len(upper)-len(word) {
		if upper[i:i+len(word)] == word {
			leftOK := i == 0 || !isIdentByte(upper[i-1])
			rightOK := i+len(word) == len(upper) || !isIdentByte(upper[i+len(word)])
			if leftOK && rightOK {
				return true
			}
		}
		i++
	}
	return false
}

func isIdentByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func hasFieldColon(raw string) bool {
	runes := []rune(raw)
	for i, r := range runes {
		if r != ':' {
			continue
		}
		j := i - 1
		for j >= 0 && (unicode.IsLetter(runes[j]) || unicode.IsNumber(runes[j]) || runes[j] == '_' || runes[j] == '-') {
			j--
		}
		if j+1 < i {
			return true
		}
	}
	return false
}

// Compile parses a map search string. Invalid queries are Empty (no extra SQL filter),
// matching the SPA which keeps all lines on parse error.
func Compile(raw string) Compiled {
	text := strings.TrimSpace(raw)
	if text == "" {
		return Compiled{Empty: true}
	}
	if !looksAdvanced(text) {
		return Compiled{Simple: text}
	}
	root, err := parseQuery(text)
	if err != nil || root == nil {
		return Compiled{Empty: true}
	}
	return Compiled{Root: root}
}

func tokenize(raw string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(raw) {
		r, w := utf8.DecodeRuneInString(raw[i:])
		if unicode.IsSpace(r) {
			i += w
			continue
		}
		switch r {
		case '(':
			tokens = append(tokens, token{typ: tokLParen, value: "("})
			i += w
			continue
		case ')':
			tokens = append(tokens, token{typ: tokRParen, value: ")"})
			i += w
			continue
		case ':':
			tokens = append(tokens, token{typ: tokColon, value: ":"})
			i += w
			continue
		case '=':
			tokens = append(tokens, token{typ: tokEq, value: "="})
			i += w
			continue
		case '!':
			if i+w < len(raw) && raw[i+w] == '=' {
				tokens = append(tokens, token{typ: tokNe, value: "!="})
				i += w + 1
				continue
			}
			tokens = append(tokens, token{typ: tokWord, value: "!"})
			i += w
			continue
		case '"':
			i += w
			var b strings.Builder
			closed := false
			for i < len(raw) {
				cr, cw := utf8.DecodeRuneInString(raw[i:])
				if cr == '\\' && i+cw < len(raw) {
					nr, nw := utf8.DecodeRuneInString(raw[i+cw:])
					b.WriteRune(nr)
					i += cw + nw
					continue
				}
				if cr == '"' {
					closed = true
					i += cw
					break
				}
				b.WriteRune(cr)
				i += cw
			}
			if !closed {
				return nil, errUnclosed
			}
			tokens = append(tokens, token{typ: tokString, value: b.String()})
			continue
		}
		start := i
		for i < len(raw) {
			cr, cw := utf8.DecodeRuneInString(raw[i:])
			if unicode.IsSpace(cr) || cr == '(' || cr == ')' || cr == ':' || cr == '"' || cr == '=' || cr == '!' {
				break
			}
			i += cw
		}
		word := raw[start:i]
		switch strings.ToUpper(word) {
		case "AND":
			tokens = append(tokens, token{typ: tokAnd, value: "AND"})
		case "OR":
			tokens = append(tokens, token{typ: tokOr, value: "OR"})
		case "NOT":
			tokens = append(tokens, token{typ: tokNot, value: "NOT"})
		default:
			tokens = append(tokens, token{typ: tokWord, value: word})
		}
	}
	return tokens, nil
}

var errUnclosed = errString("Незакрытая кавычка в поисковом запросе")
var errExpectedValue = errString("Ожидалось значение после поля поиска")
var errExpectedCond = errString("Ожидалось условие поиска")
var errParen = errString("Пропущена закрывающая скобка")
var errExtra = errString("Лишние токены в конце запроса")

type errString string

func (e errString) Error() string { return string(e) }

func parseQuery(raw string) (*Node, error) {
	tokens, err := tokenize(raw)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens}
	if len(tokens) == 0 {
		return nil, nil
	}
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.idx < len(tokens) {
		return nil, errExtra
	}
	return n, nil
}

type parser struct {
	tokens []token
	idx    int
}

func (p *parser) peek() *token {
	if p.idx >= len(p.tokens) {
		return nil
	}
	return &p.tokens[p.idx]
}

func (p *parser) next() *token {
	t := p.peek()
	if t != nil {
		p.idx++
	}
	return t
}

func startsPrimary(t *token) bool {
	return t != nil && (t.typ == tokWord || t.typ == tokString || t.typ == tokLParen || t.typ == tokNot)
}

func (p *parser) parseValue() (string, error) {
	t := p.next()
	if t == nil || (t.typ != tokWord && t.typ != tokString) {
		return "", errExpectedValue
	}
	return t.value, nil
}

func (p *parser) parseFieldTerm(field Field) (*Node, error) {
	t := p.peek()
	if t == nil {
		return nil, errExpectedValue
	}
	switch t.typ {
	case tokColon:
		p.next()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: KindTerm, Field: field, Op: OpContains, Value: val}, nil
	case tokEq:
		p.next()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: KindTerm, Field: field, Op: OpEq, Value: val}, nil
	case tokNe:
		p.next()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: KindTerm, Field: field, Op: OpNe, Value: val}, nil
	case tokWord:
		if strings.EqualFold(t.value, "contains") {
			p.next()
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			return &Node{Kind: KindTerm, Field: field, Op: OpContains, Value: val}, nil
		}
	}
	return nil, errString("Ожидался оператор сравнения после поля: " + string(field))
}

func (p *parser) parsePrimary() (*Node, error) {
	t := p.peek()
	if t == nil {
		return nil, errExpectedCond
	}
	if t.typ == tokLParen {
		p.next()
		n, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		cl := p.next()
		if cl == nil || cl.typ != tokRParen {
			return nil, errParen
		}
		return n, nil
	}
	if t.typ != tokWord && t.typ != tokString {
		return nil, errExpectedCond
	}
	head := p.next()
	if head.typ == tokWord {
		field, ok := fieldAliases[normalize(head.value)]
		if ok && field != FieldAll && p.peek() != nil {
			next := p.peek()
			if next.typ == tokColon || next.typ == tokEq || next.typ == tokNe ||
				(next.typ == tokWord && strings.EqualFold(next.value, "contains")) {
				return p.parseFieldTerm(field)
			}
		}
		if !ok {
			return nil, errString("Неизвестное поле: " + head.value)
		}
	}
	return &Node{Kind: KindTerm, Field: FieldAll, Op: OpContains, Value: head.value}, nil
}

func (p *parser) parseUnary() (*Node, error) {
	if p.peek() != nil && p.peek().typ == tokNot {
		p.next()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: KindNot, Expr: inner}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parseAnd() (*Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t != nil && t.typ == tokAnd {
			p.next()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &Node{Kind: KindAnd, Left: left, Right: right}
			continue
		}
		if startsPrimary(t) {
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = &Node{Kind: KindAnd, Left: left, Right: right}
			continue
		}
		break
	}
	return left, nil
}

func (p *parser) parseOr() (*Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() != nil && p.peek().typ == tokOr {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Node{Kind: KindOr, Left: left, Right: right}
	}
	return left, nil
}
