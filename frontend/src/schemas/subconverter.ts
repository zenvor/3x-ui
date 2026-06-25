import { z } from 'zod';

export const SubscriptionInboundSchema = z.looseObject({
  id: z.number(),
  subscriptionId: z.number(),
  inboundId: z.number(),
  clientEmail: z.string().optional(),
  sortOrder: z.number().optional(),
  cdnTls: z.boolean().optional(),
  cdnServer: z.string().optional(),
  cdnPort: z.number().optional(),
  cdnServerName: z.string().optional(),
  cdnXhttpHost: z.string().optional(),
  cdnClientFingerprint: z.string().optional(),
});

export const SubscriptionStatsSchema = z.looseObject({
  subscriptionId: z.number().optional(),
  completedCount: z.number().optional(),
  lastCompletedAt: z.string().optional(),
  lastCompletedIp: z.string().optional(),
  lastCompletedUserAgent: z.string().optional(),
  createdAt: z.string().optional(),
  updatedAt: z.string().optional(),
});

export const IpBindingRecordSchema = z.looseObject({
  id: z.number(),
  subscriptionId: z.number(),
  ip: z.string(),
  boundAt: z.string(),
  lastSeenAt: z.string(),
});

export const AccessLogRecordSchema = z.looseObject({
  id: z.number(),
  subscriptionId: z.number(),
  subscriptionRemark: z.string().optional(),
  endpoint: z.string(),
  ip: z.string().optional(),
  userAgent: z.string().optional(),
  statusCode: z.number(),
  result: z.string(),
  accessedAt: z.string(),
});

export const SubscriptionRecordSchema = z.looseObject({
  id: z.number(),
  token: z.string(),
  remark: z.string(),
  limitIp: z.number(),
  enable: z.boolean(),
  trafficStats: z.boolean().optional().default(false),
  createdAt: z.string().optional(),
  updatedAt: z.string().optional(),
  inbounds: z.array(SubscriptionInboundSchema).optional(),
  stats: SubscriptionStatsSchema.optional(),
  boundIpCount: z.number().optional(),
});

export const SubscriptionDetailRecordSchema = SubscriptionRecordSchema.extend({
  boundIps: z.array(IpBindingRecordSchema).optional(),
  accessLogs: z.array(AccessLogRecordSchema).optional(),
});

export const InboundOptionSchema = z.looseObject({
  id: z.number(),
  remark: z.string().optional(),
  tag: z.string().optional(),
  protocol: z.string().optional(),
  port: z.number().optional(),
  tlsFlowCapable: z.boolean().optional(),
  cdnTlsCapable: z.boolean().optional(),
  subconverterCapable: z.boolean().optional(),
  clients: z.array(z.looseObject({
    email: z.string(),
    enable: z.boolean().optional(),
    hasId: z.boolean().optional(),
    totalGB: z.number().optional(),
    expiryTime: z.number().optional(),
    up: z.number().optional(),
    down: z.number().optional(),
  })).nullable().optional().transform((v) => v ?? []),
});

export const DefaultsPayloadSchema = z.looseObject({
  pageSize: z.number().optional(),
});

export const SubconverterSettingsSchema = z.looseObject({
  uaFilterEnabled: z.boolean(),
  uaKeywords: z.array(z.string()).default([]),
  uaRejectStatus: z.number().default(403),
});

export const SubscriptionInboundInputSchema = z.object({
  inboundId: z.number(),
  clientEmail: z.string().optional(),
  cdnTls: z.boolean().optional(),
  cdnServer: z.string().optional(),
  cdnPort: z.number().optional(),
  cdnServerName: z.string().optional(),
});

export const CdnTLSOverrideSchema = z.object({
  enabled: z.boolean().optional(),
  server: z.string().optional(),
  port: z.number().optional(),
  serverName: z.string().optional(),
});

export const FormValuesSchema = z.object({
  remark: z.string(),
  limitIp: z.number(),
  enable: z.boolean(),
  trafficStats: z.boolean(),
  inboundIds: z.array(z.number()),
  clientEmail: z.string().optional(),
  inbounds: z.array(SubscriptionInboundInputSchema).optional(),
  cdnTls: z.record(z.string(), CdnTLSOverrideSchema).optional(),
});

export const SettingsValuesSchema = z.object({
  uaFilterEnabled: z.boolean(),
  uaKeywords: z.array(z.string()),
  uaRejectStatus: z.number(),
});

export const SubscriptionRecordListSchema = z.array(SubscriptionRecordSchema).nullable().transform((v) => v ?? []);
export const InboundOptionListSchema = z.array(InboundOptionSchema).nullable().transform((v) => v ?? []);
export const AccessLogRecordListSchema = z.array(AccessLogRecordSchema).nullable().transform((v) => v ?? []);

export type SubscriptionInbound = z.infer<typeof SubscriptionInboundSchema>;
export type SubscriptionStats = z.infer<typeof SubscriptionStatsSchema>;
export type IpBindingRecord = z.infer<typeof IpBindingRecordSchema>;
export type AccessLogRecord = z.infer<typeof AccessLogRecordSchema>;
export type SubscriptionRecord = z.infer<typeof SubscriptionRecordSchema>;
export type SubscriptionDetailRecord = z.infer<typeof SubscriptionDetailRecordSchema>;
export type InboundOption = z.infer<typeof InboundOptionSchema>;
export type DefaultsPayload = z.infer<typeof DefaultsPayloadSchema>;
export type SubconverterSettings = z.infer<typeof SubconverterSettingsSchema>;
export type FormValues = z.infer<typeof FormValuesSchema>;
export type SettingsValues = z.infer<typeof SettingsValuesSchema>;
