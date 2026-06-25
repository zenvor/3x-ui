import { lazy, useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ApiOutlined,
  CheckCircleOutlined,
  ContainerOutlined,
  FileTextOutlined,
  PlusOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Layout,
  Modal,
  Row,
  Space,
  Spin,
  Statistic,
  Tag,
  Tooltip,
  message,
} from 'antd';

import { keys } from '@/api/queryKeys';
import {
  fetchSubconverterSettings,
  useSubconverter,
  useSubconverterDetail,
} from '@/api/queries/useSubconverter';
import { LazyMount } from '@/components/utility';
import AppSidebar from '@/layouts/AppSidebar';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useTheme } from '@/hooks/useTheme';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { setMessageInstance } from '@/utils/messageBus';
import SubconverterSettingsModal from './SubconverterSettingsModal';
import SubconverterSubscriptionList from './SubconverterSubscriptionList';
import SubconverterSubscriptionModal from './SubconverterSubscriptionModal';
import type {
  FormValues,
  InboundOption,
  IpBindingRecord,
  SettingsValues,
  SubscriptionRecord,
} from './types';
import {
  canConfigureCdnTls,
  fallbackCopy,
  getCommonClientEmails,
  INBOUND_TAG_COLOR,
  isSupportedInbound,
  normalizeUAKeywords,
  requiresCdnTls,
} from './utils';
import './SubconverterPage.css';

const SubconverterAccessLogModal = lazy(() => import('./SubconverterAccessLogModal'));
const SubconverterInfoModal = lazy(() => import('./SubconverterInfoModal'));

export default function SubconverterPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [form] = Form.useForm<FormValues>();
  const [settingsForm] = Form.useForm<SettingsValues>();
  const subconverter = useSubconverter();
  const queryClient = subconverter.queryClient;

  const [formOpen, setFormOpen] = useState(false);
  const [infoTarget, setInfoTarget] = useState<Pick<SubscriptionRecord, 'id' | 'remark'> | null>(null);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [accessLogsOpen, setAccessLogsOpen] = useState(false);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [togglingId, setTogglingId] = useState<number | null>(null);

  const detailQuery = useSubconverterDetail(infoTarget?.id ?? null);

  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const pageClass = useMemo(() => {
    const classes = ['subconverter-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const subscriptions = useMemo(() => subconverter.listQuery.data ?? [], [subconverter.listQuery.data]);
  const inbounds = useMemo(() => subconverter.inboundsQuery.data ?? [], [subconverter.inboundsQuery.data]);
  const pageSize = useMemo(() => {
    const configured = subconverter.defaultsQuery.data?.pageSize;
    return configured && configured > 0 ? configured : 25;
  }, [subconverter.defaultsQuery.data?.pageSize]);

  const fetched = (
    (subconverter.listQuery.data !== undefined || subconverter.listQuery.isError) &&
    (subconverter.inboundsQuery.data !== undefined || subconverter.inboundsQuery.isError)
  );
  const spinning = (
    subconverter.listQuery.isFetching ||
    subconverter.inboundsQuery.isFetching ||
    subconverter.defaultsQuery.isFetching ||
    settingsLoading
  );

  useEffect(() => {
    const error = subconverter.listQuery.error ||
      subconverter.inboundsQuery.error ||
      subconverter.defaultsQuery.error ||
      detailQuery.error;
    if (!error) return;
    messageApi.error(error instanceof Error ? error.message : t('pages.subconverter.loadFailed'));
  }, [
    detailQuery.error,
    messageApi,
    subconverter.defaultsQuery.error,
    subconverter.inboundsQuery.error,
    subconverter.listQuery.error,
    t,
  ]);

  const inboundById = useMemo(() => {
    const out = new Map<number, InboundOption>();
    for (const inbound of inbounds) out.set(inbound.id, inbound);
    return out;
  }, [inbounds]);

  const supportedInbounds = useMemo(
    () => inbounds.filter(isSupportedInbound),
    [inbounds],
  );

  const canConfigureInboundCdnTls = useCallback((id: number) => (
    canConfigureCdnTls(inboundById.get(id))
  ), [inboundById]);
  const inboundRequiresCdnTls = useCallback((id: number) => (
    requiresCdnTls(inboundById.get(id))
  ), [inboundById]);

  const stats = useMemo(() => {
    let enabled = 0;
    let completedSubscriptions = 0;
    const linkedInboundIds = new Set<number>();
    for (const sub of subscriptions) {
      if (sub.enable) enabled += 1;
      completedSubscriptions += sub.stats?.completedCount || 0;
      for (const item of sub.inbounds || []) {
        if (inboundById.has(item.inboundId)) linkedInboundIds.add(item.inboundId);
      }
    }
    return { enabled, completedSubscriptions, linkedInbounds: linkedInboundIds.size };
  }, [inboundById, subscriptions]);

  const inboundSelectLabel = useCallback((id: number) => {
    const inbound = inboundById.get(id);
    if (!inbound) return `#${id}`;
    return formatInboundLabel(inbound.tag, inbound.remark) || `#${id}`;
  }, [inboundById]);

  const inboundTagLabel = useCallback((id: number) => {
    const inbound = inboundById.get(id);
    if (!inbound) return `#${id}`;
    return formatInboundLabel(inbound.tag, inbound.remark) || `#${id}`;
  }, [inboundById]);

  const renderInboundTags = useCallback((record: SubscriptionRecord, color: string = INBOUND_TAG_COLOR) => {
    const items = (record.inbounds || []).filter((item) => inboundById.has(item.inboundId));
    if (items.length === 0) return <span className="subconverter-muted">-</span>;
    return (
      <Space className="subconverter-inbound-tags" size={[4, 4]} wrap>
        {items.map((item) => (
          <Tag key={item.id || `${record.id}-${item.inboundId}`} color={color}>
            {inboundTagLabel(item.inboundId)}
          </Tag>
        ))}
      </Space>
    );
  }, [inboundById, inboundTagLabel]);

  const copyText = useCallback(async (text: string) => {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
      } else {
        fallbackCopy(text);
      }
      messageApi.success(t('copied'));
    } catch {
      fallbackCopy(text);
      messageApi.success(t('copied'));
    }
  }, [messageApi, t]);

  const openCreate = useCallback(() => {
    setEditingId(null);
    form.resetFields();
    form.setFieldsValue({
      remark: '',
      limitIp: 0,
      enable: true,
      trafficStats: false,
      inboundIds: [],
      clientEmail: undefined,
      cdnTls: {},
    });
    setFormOpen(true);
  }, [form]);

  const openSettings = useCallback(async () => {
    setSettingsLoading(true);
    try {
      const settings = await queryClient.fetchQuery({
        queryKey: keys.subconverter.settings(),
        queryFn: fetchSubconverterSettings,
      });
      settingsForm.setFieldsValue({
        uaFilterEnabled: settings.uaFilterEnabled,
        uaKeywords: settings.uaKeywords || [],
        uaRejectStatus: settings.uaRejectStatus || 403,
      });
      setSettingsOpen(true);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : t('pages.subconverter.loadFailed'));
    } finally {
      setSettingsLoading(false);
    }
  }, [messageApi, queryClient, settingsForm, t]);

  const openEdit = useCallback((record: SubscriptionRecord) => {
    const cdnTls: FormValues['cdnTls'] = {};
    for (const item of record.inbounds || []) {
      if (!canConfigureInboundCdnTls(item.inboundId)) continue;
      cdnTls[String(item.inboundId)] = {
        enabled: inboundRequiresCdnTls(item.inboundId) || !!item.cdnTls,
        server: item.cdnServer || '',
        port: item.cdnPort || 443,
        serverName: item.cdnServerName || '',
      };
    }
    setEditingId(record.id);
    form.resetFields();
    form.setFieldsValue({
      remark: record.remark,
      limitIp: record.limitIp,
      enable: record.enable,
      trafficStats: !!record.trafficStats,
      clientEmail: (record.inbounds || []).find((item) => item.clientEmail)?.clientEmail || undefined,
      cdnTls,
      inboundIds: (record.inbounds || [])
        .map((item) => item.inboundId)
        .filter((id) => inboundById.has(id)),
    });
    setFormOpen(true);
  }, [canConfigureInboundCdnTls, form, inboundById, inboundRequiresCdnTls]);

  const openInfo = useCallback((record: SubscriptionRecord) => {
    setInfoTarget({ id: record.id, remark: record.remark });
  }, []);

  const save = useCallback(async () => {
    const values = await form.validateFields();
    const inboundIds = values.inboundIds || [];
    const trafficStats = !!values.trafficStats;
    let selectedClientEmail = '';
    if (trafficStats) {
      const commonClientEmails = getCommonClientEmails(inboundIds, inboundById);
      selectedClientEmail = String(values.clientEmail || '').trim() || (commonClientEmails.length === 1 ? commonClientEmails[0] : '');
      if (inboundIds.length > 0 && commonClientEmails.length === 0) {
        messageApi.error(t('pages.subconverter.commonClientRequired'));
        return;
      }
      if (!selectedClientEmail || !commonClientEmails.includes(selectedClientEmail)) {
        messageApi.error(t('pages.subconverter.clientRequired'));
        return;
      }
    }
    const inbounds = inboundIds.map((id) => {
      const cdn = values.cdnTls?.[String(id)];
      const cdnEnabled = canConfigureInboundCdnTls(id) && (inboundRequiresCdnTls(id) || !!cdn?.enabled);
      return {
        inboundId: id,
        clientEmail: trafficStats ? selectedClientEmail : '',
        cdnTls: cdnEnabled,
        cdnServer: cdnEnabled ? cdn?.server?.trim() || '' : '',
        cdnPort: cdnEnabled ? Number(cdn?.port) || 443 : 0,
        cdnServerName: cdnEnabled ? cdn?.serverName?.trim() || '' : '',
      };
    });
    const payload: FormValues = {
      remark: values.remark,
      limitIp: Number(values.limitIp) || 0,
      enable: !!values.enable,
      trafficStats,
      inboundIds,
      clientEmail: trafficStats ? selectedClientEmail : undefined,
      cdnTls: values.cdnTls || {},
      inbounds,
    };
    try {
      const msg = await subconverter.save(editingId, payload);
      if (!msg?.success) throw new Error(msg?.msg || t('pages.subconverter.saveFailed'));
      messageApi.success(t('pages.subconverter.saved'));
      setFormOpen(false);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : t('pages.subconverter.saveFailed'));
    }
  }, [canConfigureInboundCdnTls, editingId, form, inboundById, inboundRequiresCdnTls, messageApi, subconverter, t]);

  const saveSettings = useCallback(async () => {
    const values = await settingsForm.validateFields();
    const payload: SettingsValues = {
      uaFilterEnabled: !!values.uaFilterEnabled,
      uaKeywords: normalizeUAKeywords(values.uaKeywords),
      uaRejectStatus: values.uaRejectStatus || 403,
    };
    try {
      const msg = await subconverter.saveSettings(payload);
      if (!msg?.success) throw new Error(msg?.msg || t('pages.subconverter.saveFailed'));
      messageApi.success(t('pages.subconverter.saved'));
      setSettingsOpen(false);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : t('pages.subconverter.saveFailed'));
    }
  }, [messageApi, settingsForm, subconverter, t]);

  const remove = useCallback((record: SubscriptionRecord) => {
    modal.confirm({
      title: t('pages.subconverter.confirmDelete'),
      okText: t('confirm'),
      cancelText: t('cancel'),
      okType: 'danger',
      async onOk() {
        const msg = await subconverter.remove(record.id);
        if (!msg?.success) throw new Error(msg?.msg || t('pages.subconverter.deleteFailed'));
        messageApi.success(t('pages.subconverter.deleted'));
      },
    });
  }, [messageApi, modal, subconverter, t]);

  const deleteBoundIp = useCallback((binding: IpBindingRecord) => {
    modal.confirm({
      title: t('pages.subconverter.confirmDeleteIp'),
      content: (
        <Space orientation="vertical" size={4}>
          <span>{`#${binding.subscriptionId}`}</span>
          <span>{`IP: ${binding.ip}`}</span>
        </Space>
      ),
      okText: t('confirm'),
      cancelText: t('cancel'),
      okType: 'danger',
      async onOk() {
        const msg = await subconverter.deleteIp(binding.subscriptionId, binding.id);
        if (!msg?.success) throw new Error(msg?.msg || t('pages.subconverter.deleteFailed'));
        messageApi.success(t('pages.subconverter.deleted'));
      },
    });
  }, [messageApi, modal, subconverter, t]);

  const infoRecord = detailQuery.data ?? null;
  const boundIps = useMemo(() => infoRecord?.boundIps ?? [], [infoRecord?.boundIps]);

  const clearBoundIps = useCallback((record: SubscriptionRecord) => {
    modal.confirm({
      title: t('pages.subconverter.confirmClearIps'),
      content: (
        <Space orientation="vertical" size={4}>
          <span>{`#${record.id}${record.remark ? ` ${record.remark}` : ''}`}</span>
          <span>{`${t('pages.subconverter.boundIps')}: ${boundIps.length}`}</span>
        </Space>
      ),
      okText: t('confirm'),
      cancelText: t('cancel'),
      okType: 'danger',
      async onOk() {
        const msg = await subconverter.clearIps(record.id);
        if (!msg?.success) throw new Error(msg?.msg || t('pages.subconverter.deleteFailed'));
        messageApi.success(t('pages.subconverter.deleted'));
      },
    });
  }, [boundIps.length, messageApi, modal, subconverter, t]);

  const resetToken = useCallback((record: SubscriptionRecord) => {
    modal.confirm({
      title: t('pages.subconverter.confirmResetToken'),
      content: (
        <Space orientation="vertical" size={4}>
          <span>{`#${record.id}${record.remark ? ` ${record.remark}` : ''}`}</span>
          <span>{`${t('pages.subconverter.boundIps')}: ${record.boundIpCount || 0}`}</span>
        </Space>
      ),
      okText: t('confirm'),
      cancelText: t('cancel'),
      okType: 'danger',
      async onOk() {
        const msg = await subconverter.resetToken(record.id);
        if (!msg?.success || !msg.obj) throw new Error(msg?.msg || t('pages.subconverter.saveFailed'));
        messageApi.success(t('pages.subconverter.updated'));
      },
    });
  }, [messageApi, modal, subconverter, t]);

  const toggleEnabled = useCallback(async (record: SubscriptionRecord, checked: boolean) => {
    setTogglingId(record.id);
    const inboundIds = (record.inbounds || []).map((item) => item.inboundId);
    let clientEmail = (record.inbounds || []).find((item) => item.clientEmail)?.clientEmail || '';
    const trafficStats = !!record.trafficStats;
    if (checked && trafficStats && !clientEmail) {
      const commonClientEmails = getCommonClientEmails(inboundIds, inboundById);
      if (commonClientEmails.length === 1) {
        clientEmail = commonClientEmails[0];
      } else {
        messageApi.error(t('pages.subconverter.clientRequired'));
        setTogglingId(null);
        return;
      }
    }
    const payload: FormValues = {
      remark: record.remark,
      limitIp: record.limitIp,
      enable: checked,
      trafficStats,
      inboundIds,
      clientEmail: checked && trafficStats ? clientEmail || undefined : undefined,
      inbounds: (record.inbounds || []).map((item) => ({
        inboundId: item.inboundId,
        clientEmail: checked && trafficStats ? item.clientEmail || clientEmail : item.clientEmail || '',
        cdnTls: !!item.cdnTls,
        cdnServer: item.cdnServer || '',
        cdnPort: item.cdnPort || 443,
        cdnServerName: item.cdnServerName || '',
      })),
    };
    try {
      const msg = await subconverter.save(record.id, payload);
      if (!msg?.success) throw new Error(msg?.msg || t('pages.subconverter.saveFailed'));
      messageApi.success(t('pages.subconverter.updated'));
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : t('pages.subconverter.saveFailed'));
    } finally {
      setTogglingId(null);
    }
  }, [inboundById, messageApi, subconverter, t]);

  const infoTitle = infoRecord || infoTarget;
  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content className="content-area">
            {messageContextHolder}
            {modalContextHolder}
            <Spin spinning={spinning || !fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : (
                <Space orientation="vertical" size={16} className="subconverter-stack">
                  <Card className="summary-card" size="small" hoverable>
                    <Row gutter={[16, 12]}>
                      <Col xs={12} sm={12} md={8}>
                        <Statistic title={t('pages.subconverter.totalCompletedSubscriptions')} value={stats.completedSubscriptions} prefix={<ContainerOutlined />} />
                      </Col>
                      <Col xs={12} sm={12} md={8}>
                        <Statistic title={t('pages.subconverter.enabledCount')} value={stats.enabled} prefix={<CheckCircleOutlined />} />
                      </Col>
                      <Col xs={24} sm={24} md={8}>
                        <Statistic title={t('pages.subconverter.linkedInbounds')} value={stats.linkedInbounds} prefix={<ApiOutlined />} />
                      </Col>
                    </Row>
                  </Card>

                  <Card
                    hoverable
                    title={(
                      <Space size={8}>
                        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                          {!isMobile && t('pages.subconverter.create')}
                        </Button>
                        {isMobile ? (
                          <Tooltip title={t('pages.subconverter.settings')}>
                            <Button icon={<SettingOutlined />} onClick={openSettings} />
                          </Tooltip>
                        ) : (
                          <Button icon={<SettingOutlined />} onClick={openSettings}>
                            {t('pages.subconverter.settings')}
                          </Button>
                        )}
                        {isMobile ? (
                          <Tooltip title={t('pages.subconverter.accessLogs')}>
                            <Button icon={<FileTextOutlined />} onClick={() => setAccessLogsOpen(true)} />
                          </Tooltip>
                        ) : (
                          <Button icon={<FileTextOutlined />} onClick={() => setAccessLogsOpen(true)}>
                            {t('pages.subconverter.accessLogs')}
                          </Button>
                        )}
                      </Space>
                    )}
                  >
                    <SubconverterSubscriptionList
                      rows={subscriptions}
                      pageSize={pageSize}
                      inboundById={inboundById}
                      supportedInbounds={supportedInbounds}
                      inboundTagLabel={inboundTagLabel}
                      renderInboundTags={renderInboundTags}
                      togglingId={togglingId}
                      onInfo={openInfo}
                      onEdit={openEdit}
                      onCopy={copyText}
                      onResetToken={resetToken}
                      onToggleEnabled={toggleEnabled}
                      onRemove={remove}
                    />
                  </Card>
                </Space>
              )}
            </Spin>

            <SubconverterSubscriptionModal
              open={formOpen}
              editingId={editingId}
              saving={subconverter.savePending}
              form={form}
              supportedInbounds={supportedInbounds}
              inboundSelectLabel={inboundSelectLabel}
              inboundTagLabel={inboundTagLabel}
              canConfigureCdnTls={canConfigureInboundCdnTls}
              isCdnTlsRequired={inboundRequiresCdnTls}
              onOk={save}
              onCancel={() => setFormOpen(false)}
            />

            <SubconverterSettingsModal
              open={settingsOpen}
              saving={subconverter.settingsPending}
              form={settingsForm}
              onOk={saveSettings}
              onCancel={() => setSettingsOpen(false)}
            />

            <LazyMount when={accessLogsOpen}>
              <SubconverterAccessLogModal
                open={accessLogsOpen}
                onClose={() => setAccessLogsOpen(false)}
              />
            </LazyMount>

            <LazyMount when={!!infoTarget}>
              <SubconverterInfoModal
                open={!!infoTarget}
                infoTitle={infoTitle}
                infoRecord={infoRecord}
                boundIps={boundIps}
                loading={detailQuery.isFetching}
                renderInboundTags={renderInboundTags}
                onCopy={copyText}
                onClearBoundIps={clearBoundIps}
                onDeleteBoundIp={deleteBoundIp}
                onCancel={() => {
                  setInfoTarget(null);
                }}
              />
            </LazyMount>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
