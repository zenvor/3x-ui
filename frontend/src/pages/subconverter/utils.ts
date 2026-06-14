import type { InboundOption, SubscriptionRecord } from './types';

export const SUBCONVERTER_API = '/panel/api/subconverter';
export const DEFAULT_UA_KEYWORDS = ['clash', 'mihomo', 'shadowrocket'];
export const INBOUND_TAG_COLOR = 'green';

export interface SubscriptionFilterState {
  filterMode?: boolean;
  filterBy?: string;
  searchKey?: string;
  protocolFilter?: string;
  inboundFilter?: number;
}

export function normalizeUAKeywords(values?: string[]): string[] {
  const seen = new Set<string>();
  const keywords: string[] = [];
  for (const raw of values || []) {
    for (const part of raw.split(/[,\s]+/)) {
      const keyword = part.trim().toLowerCase();
      if (!keyword || seen.has(keyword)) continue;
      seen.add(keyword);
      keywords.push(keyword);
    }
  }
  return keywords;
}

export function buildFeedUrl(token: string): string {
  return `${window.location.origin}/feed/${token}`;
}

export function fallbackCopy(text: string): void {
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
}

export function isSupportedInbound(inbound: InboundOption): boolean {
  return inbound.protocol === 'vless' && (inbound.subconverterCapable === true || inbound.cdnTlsCapable === true);
}

export function canConfigureCdnTls(inbound?: InboundOption): boolean {
  return inbound?.protocol === 'vless' && inbound.cdnTlsCapable === true;
}

export function requiresCdnTls(inbound?: InboundOption): boolean {
  return inbound?.cdnTlsCapable === true && inbound.subconverterCapable !== true;
}

export function formatIpLimitUsage(record: SubscriptionRecord): string {
  const used = record.boundIpCount || 0;
  return `${used} / ${record.limitIp === 0 ? '∞' : record.limitIp}`;
}

export function ipLimitTagColor(record: SubscriptionRecord): string {
  if (record.limitIp === 0) return 'purple';
  if (record.limitIp > 0 && (record.boundIpCount || 0) >= record.limitIp) return 'red';
  if (record.limitIp > 0 && (record.boundIpCount || 0) / record.limitIp >= 0.8) return 'orange';
  return 'blue';
}

export function ipLimitSortValue(record: SubscriptionRecord): number {
  return record.limitIp === 0 ? Number.POSITIVE_INFINITY : record.limitIp || 0;
}

export function getSubscriptionProtocolOptions(supportedInbounds: InboundOption[]): string[] {
  const values = new Set<string>();
  for (const inbound of supportedInbounds) {
    if (inbound.protocol) values.add(inbound.protocol);
  }
  return [...values].sort();
}

export function filterSubscriptions(
  rows: SubscriptionRecord[],
  inboundById: Map<number, InboundOption>,
  filters: SubscriptionFilterState,
): SubscriptionRecord[] {
  let filtered = rows;
  const { filterMode, filterBy, searchKey, protocolFilter, inboundFilter } = filters;
  if (filterMode && filterBy) {
    filtered = filtered.filter((sub) => (filterBy === 'enabled' ? sub.enable : !sub.enable));
  }
  if (!filterMode && searchKey?.trim()) {
    const q = searchKey.trim().toLowerCase();
    filtered = filtered.filter((sub) =>
      (sub.remark || '').toLowerCase().includes(q) ||
      (sub.token || '').toLowerCase().includes(q));
  }
  if (protocolFilter) {
    filtered = filtered.filter((sub) => (sub.inbounds || []).some((item) =>
      inboundById.get(item.inboundId)?.protocol === protocolFilter));
  }
  if (inboundFilter) {
    filtered = filtered.filter((sub) => (sub.inbounds || []).some((item) => item.inboundId === inboundFilter));
  }
  return filtered;
}
