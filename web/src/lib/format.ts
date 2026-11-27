const numberFormat = new Intl.NumberFormat("en-US");

export function formatNumber(n: number): string {
  return numberFormat.format(n);
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

export function formatDay(day: string): string {
  return new Date(`${day}T00:00:00Z`).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    timeZone: "UTC",
  });
}

export function percent(part: number, whole: number): string {
  if (whole <= 0) return "0%";
  return `${Math.round((part / whole) * 100)}%`;
}

let regionNames: Intl.DisplayNames | null | undefined;

export function countryName(code: string): string {
  if (!/^[A-Z]{2}$/.test(code)) return "Unknown";
  if (regionNames === undefined) {
    try {
      regionNames = new Intl.DisplayNames(["en"], { type: "region" });
    } catch {
      regionNames = null;
    }
  }
  return regionNames?.of(code) ?? code;
}

export function splitUrl(raw: string): { host: string; path: string } {
  try {
    const u = new URL(raw);
    const path = `${u.pathname}${u.search}`;
    return {
      host: u.host.replace(/^www\./, ""),
      path: path === "/" ? "" : path,
    };
  } catch {
    return { host: raw, path: "" };
  }
}
