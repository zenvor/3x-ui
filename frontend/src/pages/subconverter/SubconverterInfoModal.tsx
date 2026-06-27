import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CopyOutlined, EyeOutlined, QrcodeOutlined } from '@ant-design/icons';
import { Button, Divider, Modal, Popover, Space, Spin, Tag, Tooltip } from 'antd';

import { useDatepicker } from '@/hooks/useDatepicker';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { QrPanel } from '@/pages/inbounds/qr';
import { IntlUtil, SizeFormatter } from '@/utils';
import type { InboundOption, IpBindingRecord, SubscriptionRecord } from './types';
import {
  buildFeedUrl,
  clientTrafficTotal,
  clientTrafficUsed,
  formatIpLimitUsage,
  INBOUND_TAG_COLOR,
  resolveSubscriptionClient,
} from './utils';

interface SubconverterInfoModalProps {
  open: boolean;
  infoTitle: Pick<SubscriptionRecord, 'id' | 'remark'> | null;
  infoRecord: SubscriptionRecord | null;
  boundIps: IpBindingRecord[];
  loading: boolean;
  inboundById: Map<number, InboundOption>;
  onCopy: (text: string) => void;
  onClearBoundIps: (record: SubscriptionRecord) => void;
  onDeleteBoundIp: (binding: IpBindingRecord) => void;
  onCancel: () => void;
}

export default function SubconverterInfoModal({
  open,
  infoTitle,
  infoRecord,
  boundIps,
  loading,
  inboundById,
  onCopy,
  onClearBoundIps,
  onDeleteBoundIp,
  onCancel,
}: SubconverterInfoModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const [boundIpsModalOpen, setBoundIpsModalOpen] = useState(false);
  const infoFeedUrl = infoRecord ? buildFeedUrl(infoRecord.token) : '';
  const trafficClient = useMemo(
    () => (infoRecord ? resolveSubscriptionClient(infoRecord, inboundById) : undefined),
    [inboundById, infoRecord],
  );
  const trafficUsed = clientTrafficUsed(trafficClient);
  const trafficTotal = clientTrafficTotal(trafficClient);
  const trafficRemaining = trafficTotal > 0 ? Math.max(0, trafficTotal - trafficUsed) : 0;
  const dateLabel = (value?: string) => (value ? IntlUtil.formatDate(value, datepicker) || '-' : '-');
  const boundIpLabel = (binding: IpBindingRecord) => {
    const timestamp = dateLabel(binding.boundAt || binding.lastSeenAt);
    return timestamp === '-' ? binding.ip : `${binding.ip} (${timestamp})`;
  };

  useEffect(() => {
    setBoundIpsModalOpen(false);
  }, [open, infoRecord?.id]);

  const renderInboundChip = (inboundId: number) => {
    const inbound = inboundById.get(inboundId);
    const label = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : `#${inboundId}`;
    return (
      <Tag key={inboundId} color={INBOUND_TAG_COLOR}>
        {label || `#${inboundId}`}
      </Tag>
    );
  };

  const renderInfoInboundTags = (record: SubscriptionRecord) => {
    const inboundIds = (record.inbounds || [])
      .filter((item) => inboundById.has(item.inboundId))
      .sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0))
      .map((item) => item.inboundId);
    if (inboundIds.length === 0) return <span className="subconverter-muted">-</span>;

    const visible = inboundIds.slice(0, 1);
    const overflow = inboundIds.slice(1);
    return (
      <div className="subconverter-info-chips">
        {visible.map((id) => renderInboundChip(id))}
        {overflow.length > 0 && (
          <Popover
            trigger="click"
            placement="bottom"
            content={
              <div className="subconverter-info-chips subconverter-info-chips-stack">
                {overflow.map((id) => renderInboundChip(id))}
              </div>
            }
          >
            <Button
              type="text"
              size="small"
              className="subconverter-info-chip-more subconverter-chip-more-button"
              aria-label={`${t('more')} ${overflow.length}`}
            >
              +{overflow.length} {t('more') !== 'more' ? t('more') : 'more'}
            </Button>
          </Popover>
        )}
      </div>
    );
  };

  const handleClose = () => {
    setBoundIpsModalOpen(false);
    onCancel();
  };

  return (
    <>
      <Modal
        open={open}
        title={infoTitle?.remark || t('info')}
        footer={null}
        width={900}
        onCancel={handleClose}
        destroyOnHidden
      >
        <Spin spinning={loading} delay={200}>
          {infoRecord ? (
            <Space orientation="vertical" size={16} className="subconverter-stack">
              <table className="subconverter-info-table">
                <tbody>
                  <tr>
                    <td>{t('status')}</td>
                    <td>
                      <Tag color={infoRecord.enable ? 'green' : 'red'}>
                        {infoRecord.enable ? t('enabled') : t('disabled')}
                      </Tag>
                    </td>
                  </tr>
                  <tr>
                    <td>{t('pages.subconverter.completedSubscriptions')}</td>
                    <td><Tag color="cyan">{infoRecord.stats?.completedCount || 0}</Tag></td>
                  </tr>
                  <tr>
                    <td>{t('pages.subconverter.maxIps')}</td>
                    <td><Tag>{formatIpLimitUsage(infoRecord)}</Tag></td>
                  </tr>
                  <tr>
                    <td>{t('pages.subconverter.boundIps')}</td>
                    <td>
                      <Button size="small" icon={<EyeOutlined />} onClick={() => setBoundIpsModalOpen(true)}>
                        {boundIps.length > 0 ? boundIps.length : ''}
                      </Button>
                    </td>
                  </tr>
                  <tr>
                    <td>{t('pages.subconverter.trafficStats')}</td>
                    <td>
                      <Tag color={infoRecord.trafficStats ? 'green' : 'default'}>
                        {infoRecord.trafficStats ? t('enabled') : t('disabled')}
                      </Tag>
                    </td>
                  </tr>
                  {infoRecord.trafficStats && (
                    <>
                      <tr>
                        <td>{t('pages.subconverter.client')}</td>
                        <td>
                          {trafficClient ? (
                            <Tag color="green">{trafficClient.email}</Tag>
                          ) : (
                            <span className="subconverter-muted">-</span>
                          )}
                        </td>
                      </tr>
                      <tr>
                        <td>{t('pages.inbounds.traffic')}</td>
                        <td>
                          {trafficClient ? (
                            <>
                              <Tag>
                                ↑ {SizeFormatter.sizeFormat(trafficClient.up || 0)}
                                {' '}/ ↓ {SizeFormatter.sizeFormat(trafficClient.down || 0)}
                              </Tag>
                              <span className="subconverter-info-hint">
                                {SizeFormatter.sizeFormat(trafficUsed)}
                                {' '}/ {trafficTotal > 0 ? SizeFormatter.sizeFormat(trafficTotal) : '∞'}
                              </span>
                            </>
                          ) : (
                            <span className="subconverter-muted">-</span>
                          )}
                        </td>
                      </tr>
                      <tr>
                        <td>{t('pages.clients.remaining')}</td>
                        <td>
                          {trafficClient ? (
                            trafficTotal > 0
                              ? <Tag color={trafficRemaining > 0 ? '' : 'red'}>{SizeFormatter.sizeFormat(trafficRemaining)}</Tag>
                              : <Tag color="purple">∞</Tag>
                          ) : (
                            <span className="subconverter-muted">-</span>
                          )}
                        </td>
                      </tr>
                    </>
                  )}
                  <tr>
                    <td>{t('pages.inbounds.createdAt')}</td>
                    <td><Tag>{dateLabel(infoRecord.createdAt)}</Tag></td>
                  </tr>
                  <tr>
                    <td>{t('pages.inbounds.updatedAt')}</td>
                    <td><Tag>{dateLabel(infoRecord.updatedAt)}</Tag></td>
                  </tr>
                  <tr>
                    <td>{t('pages.subconverter.inbounds')}</td>
                    <td>{renderInfoInboundTags(infoRecord)}</td>
                  </tr>
                </tbody>
              </table>

              <Divider plain>{t('subscription.title')}</Divider>
              <div className="subconverter-link-row">
                <Tooltip title="Clash / Mihomo">
                  <Tag color="gold" className="subconverter-link-row-tag">CLASH</Tag>
                </Tooltip>
                <button
                  type="button"
                  className="subconverter-link-row-title subconverter-link-row-title-copy"
                  title={infoFeedUrl}
                  onClick={() => onCopy(infoFeedUrl)}
                >
                  {infoRecord.token}
                </button>
                <div className="subconverter-link-row-actions">
                  <Tooltip title={t('copy')}>
                    <Button
                      size="small"
                      icon={<CopyOutlined />}
                      aria-label={t('pages.subconverter.copyFeedUrl')}
                      onClick={() => onCopy(infoFeedUrl)}
                    />
                  </Tooltip>
                  <Popover
                    trigger="click"
                    placement="left"
                    destroyOnHidden
                    content={<QrPanel value={infoFeedUrl} remark={`${infoRecord.remark || infoRecord.token} - Clash / Mihomo`} size={220} />}
                  >
                    <Tooltip title={t('pages.clients.qrCode')}>
                      <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
                    </Tooltip>
                  </Popover>
                </div>
              </div>
            </Space>
          ) : (
            <div className="subconverter-info-loading" />
          )}
        </Spin>
      </Modal>

      <Modal
        open={boundIpsModalOpen && !!infoRecord}
        title={`${t('pages.subconverter.boundIps')}${infoRecord?.remark ? ` — ${infoRecord.remark}` : ''}`}
        width={440}
        onCancel={() => setBoundIpsModalOpen(false)}
        footer={[
          <Button
            key="clear"
            danger
            disabled={boundIps.length === 0 || !infoRecord}
            onClick={() => infoRecord && onClearBoundIps(infoRecord)}
          >
            {t('pages.subconverter.clearIps')}
          </Button>,
          <Button key="close" type="primary" onClick={() => setBoundIpsModalOpen(false)}>
            {t('close')}
          </Button>,
        ]}
      >
        {boundIps.length > 0 ? (
          <div className="subconverter-bound-ip-list">
            {boundIps.map((binding) => (
              <div key={binding.id} className="subconverter-bound-ip-row">
                <Tag color="blue" className="subconverter-bound-ip-tag">
                  {boundIpLabel(binding)}
                </Tag>
                <Button size="small" onClick={() => onDeleteBoundIp(binding)}>
                  {t('clear')}
                </Button>
              </div>
            ))}
          </div>
        ) : (
          <Tag>{t('pages.subconverter.noBoundIps')}</Tag>
        )}
      </Modal>
    </>
  );
}
