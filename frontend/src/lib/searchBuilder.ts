import type { SearchField, SearchOp } from './search';
import { SEARCH_FIELD_DEFS, SEARCH_OP_DEFS } from './search';

export interface SearchBuilderFieldGroup {
  label: string;
  fields: SearchField[];
}

export const SEARCH_BUILDER_FIELD_GROUPS: SearchBuilderFieldGroup[] = [
  {
    label: 'Общее',
    fields: ['all', 'ip', 'port', 'country', 'city', 'action', 'device'],
  },
  {
    label: 'Атакующий',
    fields: ['src_ip', 'src_port', 'src_country', 'src_city'],
  },
  {
    label: 'Цель',
    fields: ['dst_ip', 'dst_port', 'dst_country', 'dst_city'],
  },
];

export const SEARCH_BUILDER_FIELDS: SearchField[] = SEARCH_BUILDER_FIELD_GROUPS.flatMap(
  (group) => group.fields,
);

export const SEARCH_EXAMPLE_CHIPS = [
  { label: 'Action = allow', query: 'action=allow' },
  { label: 'Action != deny', query: 'action!=deny' },
  { label: 'МСЭ + страна', query: 'device:fw1 AND src_country:Россия' },
  { label: 'Заблокировано', query: 'action=blocked' },
  {
    label: 'Сложный запрос',
    query: '(src_country=Россия OR src_country=Казахстан) AND action!=allow',
  },
] as const;

export interface BuilderTerm {
  kind: 'term';
  joinWith: 'AND' | 'OR';
  negate: boolean;
  field: SearchField;
  op: SearchOp;
  value: string;
}

interface BuilderGroupChild {
  negate: boolean;
  field: SearchField;
  op: SearchOp;
  value: string;
}

export interface BuilderGroup {
  kind: 'group';
  joinWith: 'AND' | 'OR';
  negate: boolean;
  op: 'AND' | 'OR';
  children: BuilderGroupChild[];
}

export type BuilderRow = BuilderTerm | BuilderGroup;

function quoteSearchValue(raw: string): string {
  const value = String(raw || '').trim();
  if (!value) return '';
  if (!/[\s()"']/u.test(value)) return value;
  return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`;
}

export function createDefaultBuilderRow(): BuilderTerm {
  return {
    kind: 'term',
    joinWith: 'AND',
    negate: false,
    field: 'src_ip',
    op: 'eq',
    value: '',
  };
}

export function createDefaultBuilderGroup(): BuilderGroup {
  return {
    kind: 'group',
    joinWith: 'AND',
    negate: false,
    op: 'AND',
    children: [{ negate: false, field: 'src_ip', op: 'eq', value: '' }],
  };
}

function serializeOp(op: SearchOp): string {
  if (op === 'eq') return '=';
  if (op === 'ne') return '!=';
  return ':';
}

function serializeBuilderTerm(
  term: { negate?: boolean; field?: string; op?: SearchOp; value?: string },
): string | null {
  const value = String(term?.value || '').trim();
  if (!value) return null;
  const field = SEARCH_FIELD_DEFS[term.field as SearchField] ? (term.field as SearchField) : 'all';
  const op = term.op && SEARCH_OP_DEFS[term.op] ? term.op : 'contains';
  const prefix = field === 'all' ? '' : `${field}${serializeOp(op)}`;
  return `${term.negate ? 'NOT ' : ''}${prefix}${quoteSearchValue(value)}`;
}

export function serializeSearchBuilderRows(rows: BuilderRow[]): string {
  return (rows || [])
    .map((row, idx) => {
      const joinWith = idx === 0 ? '' : row.joinWith === 'OR' ? 'OR ' : 'AND ';
      if (row.kind === 'group') {
        const inner = (row.children || [])
          .map(serializeBuilderTerm)
          .filter(Boolean)
          .join(row.op === 'OR' ? ' OR ' : ' AND ');
        if (!inner) return null;
        return `${joinWith}${row.negate ? 'NOT ' : ''}(${inner})`;
      }
      const expr = serializeBuilderTerm(row);
      if (!expr) return null;
      return joinWith + expr;
    })
    .filter(Boolean)
    .join(' ');
}

export function fieldLabel(field: SearchField): string {
  return SEARCH_FIELD_DEFS[field]?.label || field;
}

export function opLabel(op: SearchOp): string {
  return SEARCH_OP_DEFS[op]?.label || op;
}
