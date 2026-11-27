export function toSortedEntries(m: Record<string, number>): {
  name: string;
  value: number;
}[] {
  return Object.entries(m)
    .map(([name, value]) => ({ name, value }))
    .sort((a, b) => b.value - a.value || a.name.localeCompare(b.name));
}
