import { apiFetch } from './client';

export interface ParseErrorRow {
  id: string;
  timestamp?: string;
  vendor?: string;
  reason?: string;
  raw?: string;
}

export function listParseErrors(params: {
  limit: string | number;
  search?: string;
}): Promise<{ errors: ParseErrorRow[] }> {
  const q = new URLSearchParams({
    limit: String(params.limit),
    search: (params.search || '').trim(),
  });
  return apiFetch<{ errors: ParseErrorRow[] }>(`/api/parse-errors?${q}`);
}

export function deleteParseErrors(ids: string[]): Promise<unknown> {
  return apiFetch('/api/parse-errors/delete', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  });
}

export function deleteAllParseErrors(): Promise<unknown> {
  return apiFetch('/api/parse-errors/delete', {
    method: 'POST',
    body: JSON.stringify({ all: true }),
  });
}
