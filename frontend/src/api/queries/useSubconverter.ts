import { useCallback } from 'react';
import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import {
  AccessLogRecordListSchema,
  DefaultsPayloadSchema,
  FormValuesSchema,
  InboundOptionListSchema,
  SettingsValuesSchema,
  SubconverterSettingsSchema,
  SubscriptionDetailRecordSchema,
  SubscriptionRecordListSchema,
  SubscriptionRecordSchema,
  type AccessLogRecord,
  type DefaultsPayload,
  type FormValues,
  type InboundOption,
  type SettingsValues,
  type SubconverterSettings,
  type SubscriptionDetailRecord,
  type SubscriptionRecord,
} from '@/schemas/subconverter';
import { SUBCONVERTER_API } from '@/pages/subconverter/utils';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } } as const;

function assertSuccess<T>(msg: Msg<T>, fallback: string): Msg<T> {
  if (!msg?.success) throw new Error(msg?.msg || fallback);
  return msg;
}

export async function fetchSubconverterList(): Promise<SubscriptionRecord[]> {
  const msg = await HttpUtil.get(`${SUBCONVERTER_API}/list`, undefined, { silent: true });
  const validated = parseMsg(assertSuccess(msg, 'Failed to fetch subscriptions'), SubscriptionRecordListSchema, 'subconverter/list');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export async function fetchSubconverterInbounds(): Promise<InboundOption[]> {
  const msg = await HttpUtil.get(`${SUBCONVERTER_API}/inbounds`, undefined, { silent: true });
  const validated = parseMsg(assertSuccess(msg, 'Failed to fetch inbound options'), InboundOptionListSchema, 'subconverter/inbounds');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export async function fetchSubconverterDefaults(): Promise<DefaultsPayload> {
  const msg = await HttpUtil.post('/panel/api/setting/defaultSettings', undefined, { silent: true });
  const validated = parseMsg(assertSuccess(msg, 'Failed to fetch defaults'), DefaultsPayloadSchema, 'setting/defaultSettings');
  return validated.obj || {};
}

export async function fetchSubconverterSettings(): Promise<SubconverterSettings> {
  const msg = await HttpUtil.get(`${SUBCONVERTER_API}/settings`, undefined, { silent: true });
  const validated = parseMsg(assertSuccess(msg, 'Failed to fetch settings'), SubconverterSettingsSchema, 'subconverter/settings');
  if (!validated.obj) throw new Error(validated.msg || 'Empty settings response');
  return validated.obj;
}

export async function fetchSubconverterDetail(id: number): Promise<SubscriptionDetailRecord> {
  const msg = await HttpUtil.get(`${SUBCONVERTER_API}/get/${id}`, undefined, { silent: true });
  const validated = parseMsg(assertSuccess(msg, 'Failed to fetch subscription detail'), SubscriptionDetailRecordSchema, `subconverter/get/${id}`);
  if (!validated.obj) throw new Error(validated.msg || 'Empty subscription detail response');
  return validated.obj;
}

export async function fetchSubconverterAccessLogs(limit: string | number): Promise<AccessLogRecord[]> {
  const msg = await HttpUtil.get(
    `${SUBCONVERTER_API}/logs?limit=${encodeURIComponent(String(limit))}`,
    undefined,
    { silent: true },
  );
  const validated = parseMsg(assertSuccess(msg, 'Failed to fetch access logs'), AccessLogRecordListSchema, 'subconverter/logs');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function invalidateSubconverterList(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: keys.subconverter.list() });
}

export function invalidateSubconverterSettings(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: keys.subconverter.settings() });
}

export function invalidateSubconverterLogs(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: keys.subconverter.logs() });
}

export function invalidateSubconverterDetail(queryClient: QueryClient, id: number) {
  return queryClient.invalidateQueries({ queryKey: keys.subconverter.detail(id) });
}

export function invalidateSubconverterRecord(queryClient: QueryClient, id?: number) {
  const invalidations = [invalidateSubconverterList(queryClient)];
  if (id) invalidations.push(invalidateSubconverterDetail(queryClient, id));
  return Promise.all(invalidations);
}

export function useSubconverterDetail(id: number | null) {
  return useQuery({
    queryKey: keys.subconverter.detail(id ?? -1),
    queryFn: () => fetchSubconverterDetail(id!),
    enabled: id !== null,
  });
}

export function useSubconverter() {
  const queryClient = useQueryClient();

  const listQuery = useQuery({
    queryKey: keys.subconverter.list(),
    queryFn: fetchSubconverterList,
  });

  const inboundsQuery = useQuery({
    queryKey: ['inbounds', 'subconverterOptions'] as const,
    queryFn: fetchSubconverterInbounds,
    staleTime: Infinity,
  });

  const defaultsQuery = useQuery({
    queryKey: keys.settings.defaults(),
    queryFn: fetchSubconverterDefaults,
    staleTime: Infinity,
  });

  const saveMut = useMutation({
    mutationFn: async ({ id, payload }: { id?: number | null; payload: FormValues }): Promise<Msg<SubscriptionRecord>> => {
      const body = FormValuesSchema.safeParse(payload);
      if (!body.success) {
        console.warn('[zod] subconverter/save body failed validation', body.error.issues);
      }
      const url = id == null ? `${SUBCONVERTER_API}/add` : `${SUBCONVERTER_API}/update/${id}`;
      const raw = await HttpUtil.post(url, body.success ? body.data : payload, JSON_HEADERS);
      return parseMsg(raw, SubscriptionRecordSchema, id == null ? 'subconverter/add' : `subconverter/update/${id}`);
    },
    onSuccess: (msg, variables) => {
      if (msg?.success) invalidateSubconverterRecord(queryClient, variables.id ?? msg.obj?.id);
    },
  });

  const saveSettingsMut = useMutation({
    mutationFn: async (payload: SettingsValues): Promise<Msg<SubconverterSettings>> => {
      const body = SettingsValuesSchema.safeParse(payload);
      if (!body.success) {
        console.warn('[zod] subconverter/settings body failed validation', body.error.issues);
      }
      const raw = await HttpUtil.post(`${SUBCONVERTER_API}/settings`, body.success ? body.data : payload, JSON_HEADERS);
      return parseMsg(raw, SubconverterSettingsSchema, 'subconverter/settings/save');
    },
    onSuccess: (msg) => {
      if (msg?.success) invalidateSubconverterSettings(queryClient);
    },
  });

  const removeMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`${SUBCONVERTER_API}/del/${id}`, undefined, { silent: true }),
    onSuccess: (msg) => {
      if (msg?.success) {
        invalidateSubconverterList(queryClient);
        invalidateSubconverterLogs(queryClient);
      }
    },
  });

  const resetTokenMut = useMutation({
    mutationFn: async (id: number): Promise<Msg<SubscriptionRecord>> => {
      const raw = await HttpUtil.post(`${SUBCONVERTER_API}/reset-token/${id}`, undefined, { silent: true });
      return parseMsg(raw, SubscriptionRecordSchema, `subconverter/reset-token/${id}`);
    },
    onSuccess: (msg, id) => {
      if (msg?.success) {
        invalidateSubconverterRecord(queryClient, id);
        invalidateSubconverterLogs(queryClient);
      }
    },
  });

  const deleteIpMut = useMutation({
    mutationFn: ({ subscriptionId, bindingId }: { subscriptionId: number; bindingId: number }) =>
      HttpUtil.post(`${SUBCONVERTER_API}/ips/${subscriptionId}/del/${bindingId}`, undefined, { silent: true }),
    onSuccess: (msg, variables) => {
      if (msg?.success) invalidateSubconverterRecord(queryClient, variables.subscriptionId);
    },
  });

  const clearIpsMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`${SUBCONVERTER_API}/ips/clear/${id}`, undefined, { silent: true }),
    onSuccess: (msg, id) => {
      if (msg?.success) invalidateSubconverterRecord(queryClient, id);
    },
  });

  const save = useCallback((id: number | null, payload: FormValues) => saveMut.mutateAsync({ id, payload }), [saveMut]);
  const saveSettings = useCallback((payload: SettingsValues) => saveSettingsMut.mutateAsync(payload), [saveSettingsMut]);
  const remove = useCallback((id: number) => removeMut.mutateAsync(id), [removeMut]);
  const resetToken = useCallback((id: number) => resetTokenMut.mutateAsync(id), [resetTokenMut]);
  const deleteIp = useCallback(
    (subscriptionId: number, bindingId: number) => deleteIpMut.mutateAsync({ subscriptionId, bindingId }),
    [deleteIpMut],
  );
  const clearIps = useCallback((id: number) => clearIpsMut.mutateAsync(id), [clearIpsMut]);

  return {
    queryClient,
    listQuery,
    inboundsQuery,
    defaultsQuery,
    save,
    saveSettings,
    remove,
    resetToken,
    deleteIp,
    clearIps,
    savePending: saveMut.isPending,
    settingsPending: saveSettingsMut.isPending,
    togglingPending: saveMut.isPending,
  };
}
