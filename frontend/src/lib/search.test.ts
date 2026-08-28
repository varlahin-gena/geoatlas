import { describe, expect, it } from 'vitest';
import { compileSearchQuery, parseSearchQuery, evaluateSearchAst } from './search';

describe('search query', () => {
  it('parses country:Россия', () => {
    const ast = parseSearchQuery('country:Россия');
    expect(ast).toEqual({ type: 'TERM', field: 'country', op: 'contains', value: 'Россия' });
  });

  it('parses AND / NOT', () => {
    const ast = parseSearchQuery('country:Россия AND NOT action:allow');
    expect(ast?.type).toBe('AND');
    if (ast?.type === 'AND') {
      expect(ast.left).toEqual({ type: 'TERM', field: 'country', op: 'contains', value: 'Россия' });
      expect(ast.right).toEqual({
        type: 'NOT',
        expr: { type: 'TERM', field: 'action', op: 'contains', value: 'allow' },
      });
    }
  });

  it('maps legacy rule alias to action', () => {
    expect(parseSearchQuery('rule:block')).toEqual({
      type: 'TERM',
      field: 'action',
      op: 'contains',
      value: 'block',
    });
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
    expect(c.ast).toEqual({ type: 'TERM', field: 'country', op: 'contains', value: 'Россия' });
  });

  it('compileSearchQuery reports unknown field', () => {
    const c = compileSearchQuery('foo:bar');
    expect(c.mode).toBe('error');
    expect(c.error).toMatch(/Неизвестное поле/);
  });

  it('parses attacker/target fields', () => {
    expect(parseSearchQuery('src_ip:10.0.0.1')).toEqual({
      type: 'TERM',
      field: 'src_ip',
      op: 'contains',
      value: '10.0.0.1',
    });
    expect(parseSearchQuery('dst_port=443')).toEqual({
      type: 'TERM',
      field: 'dst_port',
      op: 'eq',
      value: '443',
    });
    expect(parseSearchQuery('attacker:1.2.3.4')).toEqual({
      type: 'TERM',
      field: 'src_ip',
      op: 'contains',
      value: '1.2.3.4',
    });
    expect(parseSearchQuery('target_country!=Germany')).toEqual({
      type: 'TERM',
      field: 'dst_country',
      op: 'ne',
      value: 'Germany',
    });
  });

  it('parses comparison operators', () => {
    expect(parseSearchQuery('src_ip=10.0.0.1')).toEqual({
      type: 'TERM',
      field: 'src_ip',
      op: 'eq',
      value: '10.0.0.1',
    });
    expect(parseSearchQuery('action contains deny')).toEqual({
      type: 'TERM',
      field: 'action',
      op: 'contains',
      value: 'deny',
    });
  });

  it('evaluates eq and ne operators', () => {
    const eqAst = parseSearchQuery('src_ip=10.0.0.1');
    expect(evaluateSearchAst(eqAst, { src_ip: ['10.0.0.1'] })).toBe(true);
    expect(evaluateSearchAst(eqAst, { src_ip: ['10.0.0.2'] })).toBe(false);

    const neAst = parseSearchQuery('src_ip!=10.0.0.1');
    expect(evaluateSearchAst(neAst, { src_ip: ['10.0.0.2'] })).toBe(true);
    expect(evaluateSearchAst(neAst, { src_ip: ['10.0.0.1'] })).toBe(false);

    const ipNeAst = parseSearchQuery('ip!=10.0.0.1');
    expect(
      evaluateSearchAst(ipNeAst, {
        ip: ['10.0.0.2', '10.0.0.3'],
      }),
    ).toBe(true);
    expect(
      evaluateSearchAst(ipNeAst, {
        ip: ['10.0.0.1', '10.0.0.2'],
      }),
    ).toBe(false);
  });

  it('evaluates action against last_action and status', () => {
    const ast = parseSearchQuery('action=blocked');
    expect(
      evaluateSearchAst(ast, {
        action: ['deny', 'blocked'],
      }),
    ).toBe(true);
    expect(
      evaluateSearchAst(ast, {
        action: ['allow', 'allowed'],
      }),
    ).toBe(false);
  });

  it('evaluates side-specific fields', () => {
    const ast = parseSearchQuery('src_ip:10.0.0.1 AND dst_port=443');
    expect(
      evaluateSearchAst(ast, {
        src_ip: ['10.0.0.1'],
        dst_port: [443],
      }),
    ).toBe(true);
    expect(
      evaluateSearchAst(ast, {
        src_ip: ['10.0.0.2'],
        dst_port: [443],
      }),
    ).toBe(false);
  });
});
