export const SEARCH_FIELD_DEFS = {
  all: { key: 'all', label: 'Все поля', aliases: ['all', 'any', 'text'] },
  ip: { key: 'ip', label: 'IP (любая сторона)', aliases: ['ip', 'addr', 'address'] },
  port: { key: 'port', label: 'Порт (любая сторона)', aliases: ['port'] },
  country: { key: 'country', label: 'Страна (любая сторона)', aliases: ['country', 'страна'] },
  city: { key: 'city', label: 'Город (любая сторона)', aliases: ['city', 'город'] },
  action: {
    key: 'action',
    label: 'Действие',
    aliases: ['action', 'act', 'действие', 'rule', 'policy', 'правило'],
  },
  device: { key: 'device', label: 'МСЭ', aliases: ['device', 'host', 'fw', 'устройство', 'мсэ'] },
  src_ip: {
    key: 'src_ip',
    label: 'IP атакующего',
    aliases: ['src_ip', 'attacker', 'attacker_ip', 'атакующий'],
  },
  dst_ip: {
    key: 'dst_ip',
    label: 'IP цели',
    aliases: ['dst_ip', 'target', 'target_ip', 'цель'],
  },
  src_port: {
    key: 'src_port',
    label: 'Порт атакующего',
    aliases: ['src_port', 'spt', 'attacker_port'],
  },
  dst_port: {
    key: 'dst_port',
    label: 'Порт цели',
    aliases: ['dst_port', 'dpt', 'target_port'],
  },
  src_country: {
    key: 'src_country',
    label: 'Страна атакующего',
    aliases: ['src_country', 'attacker_country'],
  },
  dst_country: {
    key: 'dst_country',
    label: 'Страна цели',
    aliases: ['dst_country', 'target_country'],
  },
  src_city: {
    key: 'src_city',
    label: 'Город атакующего',
    aliases: ['src_city', 'attacker_city'],
  },
  dst_city: {
    key: 'dst_city',
    label: 'Город цели',
    aliases: ['dst_city', 'target_city'],
  },
  src: { key: 'src', label: 'Источник', aliases: ['src', 'source', 'from'] },
  dst: { key: 'dst', label: 'Назначение', aliases: ['dst', 'dest', 'destination', 'to'] },
  proto: { key: 'proto', label: 'Протокол', aliases: ['proto', 'protocol'] },
  zone: { key: 'zone', label: 'Зона', aliases: ['zone'] },
} as const;

export type SearchField = keyof typeof SEARCH_FIELD_DEFS;

export type SearchOp = 'contains' | 'eq' | 'ne';

export const SEARCH_OP_DEFS = {
  contains: { key: 'contains' as const, label: 'содержит' },
  eq: { key: 'eq' as const, label: '=' },
  ne: { key: 'ne' as const, label: '!=' },
} as const;

export type SearchAst =
  | { type: 'TERM'; field: SearchField; op: SearchOp; value: string }
  | { type: 'NOT'; expr: SearchAst }
  | { type: 'AND'; left: SearchAst; right: SearchAst }
  | { type: 'OR'; left: SearchAst; right: SearchAst };

type Token =
  | {
      type: 'WORD' | 'STRING' | 'COLON' | 'EQ' | 'NE' | 'LPAREN' | 'RPAREN' | 'AND' | 'OR' | 'NOT';
      value: string;
    };

function normalizeText(v: unknown): string {
  return String(v ?? '')
    .toLowerCase()
    .normalize('NFKC')
    .trim();
}

let aliasMap: Record<string, SearchField> | null = null;

function getAliasMap(): Record<string, SearchField> {
  if (aliasMap) return aliasMap;
  aliasMap = {};
  for (const def of Object.values(SEARCH_FIELD_DEFS)) {
    for (const alias of def.aliases) {
      aliasMap[normalizeText(alias)] = def.key;
    }
  }
  return aliasMap;
}

function canonicalSearchField(field: string): SearchField | null {
  if (!field) return 'all';
  return getAliasMap()[normalizeText(field)] || null;
}

function searchValueTokens(text: string): Token[] {
  const raw = String(text || '').trim();
  if (!raw) return [];
  const tokens: Token[] = [];
  let i = 0;
  while (i < raw.length) {
    const ch = raw[i];
    if (/\s/.test(ch)) {
      i++;
      continue;
    }
    if (ch === '(') {
      tokens.push({ type: 'LPAREN', value: ch });
      i++;
      continue;
    }
    if (ch === ')') {
      tokens.push({ type: 'RPAREN', value: ch });
      i++;
      continue;
    }
    if (ch === ':') {
      tokens.push({ type: 'COLON', value: ch });
      i++;
      continue;
    }
    if (ch === '=') {
      tokens.push({ type: 'EQ', value: ch });
      i++;
      continue;
    }
    if (ch === '!' && i + 1 < raw.length && raw[i + 1] === '=') {
      tokens.push({ type: 'NE', value: '!=' });
      i += 2;
      continue;
    }
    if (ch === '"') {
      i++;
      let value = '';
      let closed = false;
      while (i < raw.length) {
        const cur = raw[i];
        if (cur === '\\' && i + 1 < raw.length) {
          value += raw[i + 1];
          i += 2;
          continue;
        }
        if (cur === '"') {
          closed = true;
          i++;
          break;
        }
        value += cur;
        i++;
      }
      if (!closed) throw new Error('Незакрытая кавычка в поисковом запросе');
      tokens.push({ type: 'STRING', value });
      continue;
    }
    let word = '';
    while (i < raw.length) {
      const cur = raw[i];
      if (/\s/.test(cur) || cur === '(' || cur === ')' || cur === ':' || cur === '"' || cur === '=' || cur === '!') {
        break;
      }
      word += cur;
      i++;
    }
    const upper = word.toUpperCase();
    if (upper === 'AND' || upper === 'OR' || upper === 'NOT') {
      tokens.push({ type: upper as 'AND' | 'OR' | 'NOT', value: upper });
    } else {
      tokens.push({ type: 'WORD', value: word });
    }
  }
  return tokens;
}

export function parseSearchQuery(raw: string): SearchAst | null {
  const tokens = searchValueTokens(raw);
  let idx = 0;

  const peek = () => tokens[idx] || null;
  const next = () => tokens[idx++] || null;
  const startsPrimary = (token: Token | null) =>
    !!token &&
    (token.type === 'WORD' || token.type === 'STRING' || token.type === 'LPAREN' || token.type === 'NOT');

  function parseValueToken(): string {
    const token = next();
    if (!token || (token.type !== 'WORD' && token.type !== 'STRING')) {
      throw new Error('Ожидалось значение после поля поиска');
    }
    return token.value;
  }

  function parseFieldTerm(field: SearchField): SearchAst {
    const nextToken = peek();
    if (!nextToken) throw new Error('Ожидалось значение после поля поиска');
    if (nextToken.type === 'COLON') {
      next();
      return { type: 'TERM', field, op: 'contains', value: parseValueToken() };
    }
    if (nextToken.type === 'EQ') {
      next();
      return { type: 'TERM', field, op: 'eq', value: parseValueToken() };
    }
    if (nextToken.type === 'NE') {
      next();
      return { type: 'TERM', field, op: 'ne', value: parseValueToken() };
    }
    if (nextToken.type === 'WORD' && normalizeText(nextToken.value) === 'contains') {
      next();
      return { type: 'TERM', field, op: 'contains', value: parseValueToken() };
    }
    throw new Error('Ожидался оператор сравнения после поля: ' + field);
  }

  function parsePrimary(): SearchAst {
    const token = peek();
    if (!token) throw new Error('Ожидалось условие поиска');
    if (token.type === 'LPAREN') {
      next();
      const expr = parseOr();
      const closing = next();
      if (!closing || closing.type !== 'RPAREN') throw new Error('Пропущена закрывающая скобка');
      return expr;
    }
    if (token.type !== 'WORD' && token.type !== 'STRING') {
      throw new Error('Ожидалось условие поиска');
    }
    const head = next()!;
    if (head.type === 'WORD') {
      const field = canonicalSearchField(head.value);
      if (field && field !== 'all' && peek()) {
        const opToken = peek()!;
        if (
          opToken.type === 'COLON' ||
          opToken.type === 'EQ' ||
          opToken.type === 'NE' ||
          (opToken.type === 'WORD' && normalizeText(opToken.value) === 'contains')
        ) {
          return parseFieldTerm(field);
        }
      }
      if (field === null) throw new Error('Неизвестное поле: ' + head.value);
    }
    return { type: 'TERM', field: 'all', op: 'contains', value: head.value };
  }

  function parseUnary(): SearchAst {
    if (peek()?.type === 'NOT') {
      next();
      return { type: 'NOT', expr: parseUnary() };
    }
    return parsePrimary();
  }

  function parseAnd(): SearchAst {
    let left = parseUnary();
    while (true) {
      const token = peek();
      if (token?.type === 'AND') {
        next();
        left = { type: 'AND', left, right: parseUnary() };
        continue;
      }
      if (startsPrimary(token)) {
        left = { type: 'AND', left, right: parseUnary() };
        continue;
      }
      break;
    }
    return left;
  }

  function parseOr(): SearchAst {
    let left = parseAnd();
    while (peek()?.type === 'OR') {
      next();
      left = { type: 'OR', left, right: parseAnd() };
    }
    return left;
  }

  if (!tokens.length) return null;
  const ast = parseOr();
  if (idx < tokens.length) throw new Error('Лишние токены в конце запроса');
  return ast;
}

export function evaluateSearchAst(
  ast: SearchAst | null,
  fieldValues: Record<string, unknown[]>,
): boolean {
  if (!ast) return true;
  if (ast.type === 'TERM') {
    const fields = fieldValues[ast.field] || [];
    const needle = normalizeText(ast.value);
    if (!needle) return true;
    const normalized = fields.map((value) => normalizeText(value));
    if (ast.op === 'eq') {
      return normalized.some((value) => value === needle);
    }
    if (ast.op === 'ne') {
      return normalized.every((value) => value !== needle);
    }
    return normalized.some((value) => value.includes(needle));
  }
  if (ast.type === 'NOT') return !evaluateSearchAst(ast.expr, fieldValues);
  if (ast.type === 'AND') {
    return evaluateSearchAst(ast.left, fieldValues) && evaluateSearchAst(ast.right, fieldValues);
  }
  if (ast.type === 'OR') {
    return evaluateSearchAst(ast.left, fieldValues) || evaluateSearchAst(ast.right, fieldValues);
  }
  return true;
}

export function compileSearchQuery(raw: string): {
  raw: string;
  mode: 'empty' | 'simple' | 'advanced' | 'error';
  ast: SearchAst | null;
  error: string;
} {
  const text = String(raw || '').trim();
  if (!text) return { raw: '', mode: 'empty', ast: null, error: '' };
  try {
    const advanced =
      /[()"]/u.test(text) ||
      /!=|=/u.test(text) ||
      /\b(AND|OR|NOT|contains)\b/i.test(text) ||
      /\b[\p{L}\p{N}_-]+\s*:/u.test(text);
    if (!advanced) {
      return {
        raw: text,
        mode: 'simple',
        ast: { type: 'TERM', field: 'all', op: 'contains', value: text },
        error: '',
      };
    }
    return { raw: text, mode: 'advanced', ast: parseSearchQuery(text), error: '' };
  } catch (e) {
    return {
      raw: text,
      mode: 'error',
      ast: null,
      error: e instanceof Error ? e.message : String(e),
    };
  }
}
