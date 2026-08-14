import { apiFetch, apiFetchRaw } from './client';

export function fetchParseSamples(): Promise<Record<string, string[]>> {
  return apiFetch<Record<string, string[]>>('/api/parse-samples');
}

export function runParseTest(body: string, init?: RequestInit): Promise<Response> {
  return apiFetchRaw('/api/parse-test', {
    method: 'POST',
    headers: { 'Content-Type': 'text/plain' },
    body,
    ...init,
  });
}
