import type { InboundOption, SubscriptionRecord } from './types';

export const SUBCONVERTER_API = '/panel/api/subconverter';
export const DEFAULT_UA_KEYWORDS = ['clash', 'mihomo', 'shadowrocket'];
export const INBOUND_TAG_COLOR = 'blue';

export type InboundOptionClient = InboundOption['clients'][number];

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

export function requiresCdnTls(_inbound?: InboundOption): boolean {
  return false;
}

export function getCommonClientEmails(
  inboundIds: number[],
  inboundById: Map<number, InboundOption>,
): string[] {
  if (inboundIds.length === 0) return [];

  let common: string[] | null = null;
  for (const id of inboundIds) {
    const inbound = inboundById.get(id);
    const emails = uniqueEmails((inbound?.clients || [])
      .filter(isExportableInboundClient)
      .map((client) => client.email));
    if (emails.length === 0) return [];
    if (common === null) {
      common = emails;
      continue;
    }
    const emailSet = new Set(emails);
    common = common.filter((email) => emailSet.has(email));
    if (common.length === 0) return [];
  }
  return common || [];
}

export function getCommonClientDetails(
  inboundIds: number[],
  inboundById: Map<number, InboundOption>,
): InboundOptionClient[] {
  if (inboundIds.length === 0) return [];

  let common: Map<string, InboundOptionClient> | null = null;
  for (const id of inboundIds) {
    const inbound = inboundById.get(id);
    const clientsByEmail = clientsByEmailMap(inbound?.clients || []);
    if (clientsByEmail.size === 0) return [];
    if (common === null) {
      common = clientsByEmail;
      continue;
    }
    for (const [email, current] of [...common.entries()]) {
      const next = clientsByEmail.get(email);
      if (!next) {
        common.delete(email);
        continue;
      }
      common.set(email, mergeClientDetails(current, next));
    }
    if (common.size === 0) return [];
  }
  return common ? [...common.values()] : [];
}

export function isExportableInboundClient(client: InboundOptionClient): boolean {
  return client.enable !== false && client.hasId !== false;
}

export function clientTrafficUsed(client?: InboundOptionClient): number {
  return Math.max(0, Number(client?.up || 0) + Number(client?.down || 0));
}

export function clientTrafficTotal(client?: InboundOptionClient): number {
  return Math.max(0, Number(client?.totalGB || 0));
}

export function isClientDepleted(client?: InboundOptionClient, now = Date.now()): boolean {
  if (!client) return false;
  const total = clientTrafficTotal(client);
  const expiryTime = Number(client.expiryTime || 0);
  return (total > 0 && clientTrafficUsed(client) >= total) || (expiryTime > 0 && expiryTime <= now);
}

function uniqueEmails(values: string[]): string[] {
  const seen = new Set<string>();
  const emails: string[] = [];
  for (const value of values) {
    const email = String(value || '').trim();
    if (!email || seen.has(email)) continue;
    seen.add(email);
    emails.push(email);
  }
  return emails;
}

function clientsByEmailMap(clients: InboundOptionClient[]): Map<string, InboundOptionClient> {
  const out = new Map<string, InboundOptionClient>();
  for (const client of clients) {
    const email = String(client.email || '').trim();
    if (!email) continue;
    const normalized = { ...client, email };
    const existing = out.get(email);
    out.set(email, existing ? mergeClientDetails(existing, normalized) : normalized);
  }
  return out;
}

function mergeClientDetails(left: InboundOptionClient, right: InboundOptionClient): InboundOptionClient {
  const leftExpiry = Number(left.expiryTime || 0);
  const rightExpiry = Number(right.expiryTime || 0);
  return {
    ...left,
    enable: left.enable !== false && right.enable !== false,
    hasId: left.hasId !== false && right.hasId !== false,
    totalGB: Math.max(Number(left.totalGB || 0), Number(right.totalGB || 0)),
    expiryTime: leftExpiry > 0 && rightExpiry > 0
      ? Math.min(leftExpiry, rightExpiry)
      : Math.max(leftExpiry, rightExpiry),
    up: Math.max(Number(left.up || 0), Number(right.up || 0)),
    down: Math.max(Number(left.down || 0), Number(right.down || 0)),
  };
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
