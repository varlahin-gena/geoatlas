export function Icon({ d, paths }: { d?: string; paths?: string[] }) {
  return (
    <svg className="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      {d ? <path d={d} /> : null}
      {(paths || []).map((p) => (
        <path key={p} d={p} />
      ))}
    </svg>
  );
}
