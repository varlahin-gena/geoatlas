import { describe, expect, it } from 'vitest';
import { compileSearchQuery, parseSearchQuery, evaluateSearchAst } from './search';

describe('search query', () => {
  it('parses country:Россия', () => {
    const ast = parseSearchQuery('country:Россия');
    expect(ast).toEqual({ type: 'TERM', field: 'country', value: 'Россия' });
  });

  it('parses AND / NOT', () => {
    const ast = parseSearchQuery('country:Россия AND NOT rule:allow');
    expect(ast?.type).toBe('AND');
    if (ast?.type === 'AND') {
      expect(ast.left).toEqual({ type: 'TERM', field: 'country', value: 'Россия' });
      expect(ast.right).toEqual({
        type: 'NOT',
        expr: { type: 'TERM', field: 'rule', value: 'allow' },
      });
    }
  });

  it('evaluates TERM against field values', () => {
    const ast = parseSearchQuery('country:Россия');
    expect(evaluateSearchAst(ast, { country: ['Russia', 'Россия'] })).toBe(true);
    expect(evaluateSearchAst(ast, { country: ['Germany'] })).toBe(false);
  });

  it('compileSearchQuery returns advanced mode for field queries', () => {
    const c = compileSearchQuery('country:Россия');
    expect(c.mode).toBe('advanced');
    expect(c.error).toBe('');
    expect(c.ast).toEqual({ type: 'TERM', field: 'country', value: 'Россия' });
  });

  it('compileSearchQuery reports unknown field', () => {
    const c = compileSearchQuery('foo:bar');
    expect(c.mode).toBe('error');
    expect(c.error).toMatch(/Неизвестное поле/);
  });
});
