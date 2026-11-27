export interface Link {
  code: string;
  shortUrl: string;
  url: string;
  custom: boolean;
  createdAt: string;
  expiresAt?: string;
  expired: boolean;
  clicks: number;
}
export interface CreateLinkInput {
  url: string;
  alias?: string;
  expiresAt?: string;
}
export interface DayCount {
  day: string;
  clicks: number;
}
export interface ReferrerCount {
  referrer: string;
  clicks: number;
}
export interface Stats {
  totalClicks: number;
  byDay: DayCount[];
  byDevice: Record<string, number>;
  byCountry: Record<string, number>;
  topReferrers: ReferrerCount[];
}
