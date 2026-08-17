import type { SearchField } from './search';
import { SEARCH_FIELD_DEFS } from './search';

export const SEARCH_BUILDER_FIELDS: SearchField[] = ['all', 'ip', 'country', 'city', 'rule', 'device'];

export const SEARCH_EXAMPLE_CHIPS = [
  { label: 'country:Россия', query: 'country:Россия' },
  { label: 'rule:block', query: 'rule:block' },
  { label: 'NOT city:Москва', query: 'NOT city:Москва' },
  { label: 'country AND device', query: 'country:Россия AND device:fw1' },
  { label: '(A OR B) AND NOT', query: '(country:Россия OR country:Казахстан) AND NOT rule:allow' },
] as const;

export interface BuilderTerm {
  kind: 'term';
  joinWith: 'AND' | 'OR';
  negate: boolean;
  field: SearchField;
  value: string;
}

interface BuilderGroupChild {
  negate: boolean;
  field: SearchField;
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
    field: 'all',
    value: '',
  };
}

export function createDefaultBuilderGroup(): BuilderGroup {
  return {
    kind: 'group',
    joinWith: 'AND',
    negate: false,
    op: 'AND',
    children: [{ negate: false, field: 'all', value: '' }],
  };
}

function serializeBuilderTerm(term: { negate?: boolean; field?: string; value?: string }): string | null {
  const value = String(term?.value || '').trim();
  if (!value) return null;
  const field = SEARCH_FIELD_DEFS[term.field as SearchField] ? (term.field as SearchField) : 'all';
  const prefix = field === 'all' ? '' : `${field}:`;
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
