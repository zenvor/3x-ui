import { QueryClient } from '@tanstack/react-query';
import { describe, expect, it, vi } from 'vitest';

import { keys } from '@/api/queryKeys';
import {
  invalidateSubconverterRecord,
  invalidateSubconverterSettings,
} from '@/api/queries/useSubconverter';
import {
  InboundOptionListSchema,
  SettingsValuesSchema,
  SubscriptionRecordListSchema,
} from '@/schemas/subconverter';
import type { InboundOption, SubscriptionRecord } from '@/schemas/subconverter';
import {
  canConfigureCdnTls,
  filterSubscriptions,
  getCommonClientDetails,
  formatIpLimitUsage,
  getSubscriptionProtocolOptions,
  getCommonClientEmails,
  isClientDepleted,
  isSupportedInbound,
  ipLimitSortValue,
  ipLimitTagColor,
  normalizeUAKeywords,
  requiresCdnTls,
} from '@/pages/subconverter/utils';

function sub(overrides: Partial<SubscriptionRecord>): SubscriptionRecord {
  return {
    id: 1,
    token: 'token-a',
    remark: 'alpha',
    limitIp: 1,
    enable: true,
    trafficStats: false,
    inbounds: [],
    ...overrides,
  };
}

function inbound(overrides: Partial<InboundOption>): InboundOption {
  return {
    id: 1,
    remark: '',
    tag: '',
    protocol: 'vless',
    port: 443,
    clients: [],
    ...overrides,
  };
}

describe('subconverter utilities', () => {
  it('normalizes UA keywords from mixed separators', () => {
    expect(normalizeUAKeywords([' Clash, mihomo ', 'MIHOMO shadowrocket', ''])).toEqual([
      'clash',
      'mihomo',
      'shadowrocket',
    ]);
  });

  it('formats and sorts IP limit state', () => {
    const unlimited = sub({ limitIp: 0, boundIpCount: 12 });
    const warning = sub({ limitIp: 10, boundIpCount: 8 });
    const full = sub({ limitIp: 2, boundIpCount: 2 });

    expect(formatIpLimitUsage(unlimited)).toBe('12 / ∞');
    expect(ipLimitTagColor(unlimited)).toBe('purple');
    expect(ipLimitTagColor(warning)).toBe('orange');
    expect(ipLimitTagColor(full)).toBe('red');
    expect(ipLimitSortValue(unlimited)).toBe(Number.POSITIVE_INFINITY);
  });

  it('filters subscriptions by status, search, protocol and inbound', () => {
    const inbounds = [
      inbound({ id: 11, protocol: 'vless' }),
      inbound({ id: 22, protocol: 'vmess' }),
    ];
    const inboundById = new Map(inbounds.map((item) => [item.id, item]));
    const rows = [
      sub({ id: 1, remark: 'Alpha', token: 'aaa', enable: true, inbounds: [{ id: 1, subscriptionId: 1, inboundId: 11 }] }),
      sub({ id: 2, remark: 'Beta', token: 'bbb', enable: false, inbounds: [{ id: 2, subscriptionId: 2, inboundId: 22 }] }),
      sub({ id: 3, remark: 'Gamma', token: 'target-token', enable: true, inbounds: [] }),
    ];

    expect(filterSubscriptions(rows, inboundById, { filterMode: true, filterBy: 'disabled' }).map((item) => item.id)).toEqual([2]);
    expect(filterSubscriptions(rows, inboundById, { searchKey: 'target' }).map((item) => item.id)).toEqual([3]);
    expect(filterSubscriptions(rows, inboundById, { protocolFilter: 'vless' }).map((item) => item.id)).toEqual([1]);
    expect(filterSubscriptions(rows, inboundById, { inboundFilter: 22 }).map((item) => item.id)).toEqual([2]);
  });

  it('builds sorted protocol options from supported inbounds', () => {
    expect(getSubscriptionProtocolOptions([
      inbound({ id: 1, protocol: 'vmess' }),
      inbound({ id: 2, protocol: 'vless' }),
      inbound({ id: 3, protocol: 'vless' }),
    ])).toEqual(['vless', 'vmess']);
  });

  it('intersects common client emails across selected inbounds', () => {
    const inbounds = [
      inbound({ id: 1, clients: [{ email: 'alice@x' }, { email: 'bob@x' }] }),
      inbound({ id: 2, clients: [{ email: 'alice@x' }] }),
      inbound({ id: 3, clients: [{ email: 'carol@x' }] }),
    ];
    const inboundById = new Map(inbounds.map((item) => [item.id, item]));

    expect(getCommonClientEmails([1, 2], inboundById)).toEqual(['alice@x']);
    expect(getCommonClientEmails([1, 3], inboundById)).toEqual([]);
    expect(getCommonClientEmails([], inboundById)).toEqual([]);
  });

  it('keeps disabled common clients out of selectable emails but available for diagnostics', () => {
    const inbounds = [
      inbound({ id: 1, clients: [{ email: 'alice@x', enable: false, totalGB: 100, up: 40, down: 60 }] }),
      inbound({ id: 2, clients: [{ email: 'alice@x', enable: false, totalGB: 100, up: 40, down: 60 }] }),
    ];
    const inboundById = new Map(inbounds.map((item) => [item.id, item]));

    expect(getCommonClientEmails([1, 2], inboundById)).toEqual([]);
    expect(getCommonClientDetails([1, 2], inboundById).map((client) => client.email)).toEqual(['alice@x']);
    expect(isClientDepleted(getCommonClientDetails([1, 2], inboundById)[0])).toBe(true);
  });

  it('keeps the product rule to Mihomo-compatible inbound selection', () => {
    expect(isSupportedInbound(inbound({ protocol: 'vless', subconverterCapable: true }))).toBe(true);
    expect(isSupportedInbound(inbound({ protocol: 'vless', cdnTlsCapable: true }))).toBe(true);
    expect(isSupportedInbound(inbound({ protocol: 'vless' }))).toBe(false);
    expect(isSupportedInbound(inbound({ protocol: 'vmess', subconverterCapable: true }))).toBe(false);
  });

  it('shows CDN TLS controls only for CDN-capable VLESS inbounds', () => {
    expect(canConfigureCdnTls(inbound({ protocol: 'vless', cdnTlsCapable: true }))).toBe(true);
    expect(canConfigureCdnTls(inbound({ protocol: 'vless', cdnTlsCapable: false }))).toBe(false);
    expect(canConfigureCdnTls(inbound({ protocol: 'vmess', cdnTlsCapable: true }))).toBe(false);
    expect(canConfigureCdnTls(undefined)).toBe(false);
  });

  it('keeps CDN TLS optional for CDN-capable inbounds', () => {
    expect(requiresCdnTls(inbound({ cdnTlsCapable: true, subconverterCapable: false }))).toBe(false);
    expect(requiresCdnTls(inbound({ cdnTlsCapable: true, subconverterCapable: true }))).toBe(false);
    expect(requiresCdnTls(inbound({ cdnTlsCapable: false, subconverterCapable: true }))).toBe(false);
    expect(requiresCdnTls(undefined)).toBe(false);
  });
});

describe('subconverter schemas', () => {
  it('parses nullable lists and keeps upstream extra fields', () => {
    expect(SubscriptionRecordListSchema.parse(null)).toEqual([]);
    const parsed = InboundOptionListSchema.parse([
      { id: 1, protocol: 'vless', port: 443, tlsVerifyMode: 'pinned' },
    ]);
    expect(parsed[0]).toMatchObject({ id: 1, protocol: 'vless', port: 443, tlsVerifyMode: 'pinned' });
  });

  it('validates settings payload shape', () => {
    expect(SettingsValuesSchema.safeParse({
      uaFilterEnabled: true,
      uaKeywords: ['clash'],
      uaRejectStatus: 403,
    }).success).toBe(true);
    expect(SettingsValuesSchema.safeParse({
      uaFilterEnabled: true,
      uaKeywords: 'clash',
      uaRejectStatus: 403,
    }).success).toBe(false);
  });
});

describe('subconverter invalidation', () => {
  it('invalidates list and detail after record mutation', async () => {
    const queryClient = new QueryClient();
    const spy = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue(undefined);

    await invalidateSubconverterRecord(queryClient, 7);

    expect(spy).toHaveBeenCalledWith({ queryKey: keys.subconverter.list() });
    expect(spy).toHaveBeenCalledWith({ queryKey: keys.subconverter.detail(7) });
  });

  it('invalidates only settings after settings mutation', async () => {
    const queryClient = new QueryClient();
    const spy = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue(undefined);

    await invalidateSubconverterSettings(queryClient);

    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith({ queryKey: keys.subconverter.settings() });
  });
});
