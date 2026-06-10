import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ContainerOutlined, CopyOutlined, DeleteOutlined } from '@ant-design/icons';
import { Button, Divider, Modal, Space, Spin, Table, Tabs, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { useDatepicker } from '@/hooks/useDatepicker';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { IntlUtil } from '@/utils';
import type { IpBindingRecord, SubscriptionRecord } from './types';
import { buildFeedUrl, formatIpLimitUsage, ipLimitTagColor } from './utils';

interface SubconverterInfoModalProps {
  open: boolean;
  infoTitle: Pick<SubscriptionRecord, 'id' | 'remark'> | null;
  infoRecord: SubscriptionRecord | null;
  boundIps: IpBindingRecord[];
  loading: boolean;
  renderInboundTags: (record: SubscriptionRecord) => ReactNode;
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
  renderInboundTags,
  onCopy,
  onClearBoundIps,
  onDeleteBoundIp,
  onCancel,
}: SubconverterInfoModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const { isMobile } = useMediaQuery();
  const infoFeedUrl = infoRecord ? buildFeedUrl(infoRecord.token) : '';

  const boundIpColumns = useMemo<ColumnsType<IpBindingRecord>>(() => [
    {
      title: 'IP',
      dataIndex: 'ip',
      width: 150,
      render: (value: string) => <Tag color="blue">{value}</Tag>,
    },
    {
      title: t('pages.subconverter.boundAt'),
      dataIndex: 'boundAt',
      width: 180,
      render: (value: string) => IntlUtil.formatDate(value, datepicker) || '-',
    },
    {
      title: t('pages.subconverter.lastSeenAt'),
      dataIndex: 'lastSeenAt',
      width: 180,
      render: (value: string) => IntlUtil.formatDate(value, datepicker) || '-',
    },
    {
      title: t('pages.inbounds.operate'),
      key: 'actions',
      width: 88,
      align: 'center',
      render: (_, record) => (
        <Tooltip title={t('delete')}>
          <Button
            size="small"
            type="text"
            danger
            icon={<DeleteOutlined />}
            aria-label={t('delete')}
            onClick={() => onDeleteBoundIp(record)}
          />
        </Tooltip>
      ),
    },
  ], [datepicker, onDeleteBoundIp, t]);

  const renderBoundIpCards = () => {
    if (boundIps.length === 0) {
      return (
        <div className="subconverter-card-empty">
          <ContainerOutlined style={{ fontSize: 28, opacity: 0.5 }} />
          <div>{t('noData')}</div>
        </div>
      );
    }
    return (
      <div className="subconverter-cards">
        {boundIps.map((binding) => (
          <div key={binding.id} className="subconverter-card">
            <div className="subconverter-card-head">
              <span className="subconverter-card-id">#{binding.id}</span>
              <span className="subconverter-card-title">{binding.ip}</span>
              <div className="subconverter-card-actions">
                <Tooltip title={t('delete')}>
                  <Button
                    size="small"
                    type="text"
                    danger
                    icon={<DeleteOutlined />}
                    aria-label={t('delete')}
                    onClick={() => onDeleteBoundIp(binding)}
                  />
                </Tooltip>
              </div>
            </div>
            <div className="subconverter-card-meta">
              <div className="subconverter-card-row">
                <span>{t('pages.subconverter.boundAt')}</span>
                <div>{IntlUtil.formatDate(binding.boundAt, datepicker) || '-'}</div>
              </div>
              <div className="subconverter-card-row">
                <span>{t('pages.subconverter.lastSeenAt')}</span>
                <div>{IntlUtil.formatDate(binding.lastSeenAt, datepicker) || '-'}</div>
              </div>
            </div>
          </div>
        ))}
      </div>
    );
  };

  return (
    <Modal
      open={open}
      title={infoTitle ? `#${infoTitle.id}${infoTitle.remark ? ` ${infoTitle.remark}` : ''}` : t('info')}
      footer={null}
      width={900}
      onCancel={onCancel}
      destroyOnHidden
    >
      <Spin spinning={loading} delay={200}>
        {infoRecord ? (
          <Tabs
            items={[
              {
                key: 'overview',
                label: t('pages.subconverter.overview'),
                children: (
                  <Space direction="vertical" size={16} className="subconverter-stack">
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
                          <td><Tag color={ipLimitTagColor(infoRecord)}>{formatIpLimitUsage(infoRecord)}</Tag></td>
                        </tr>
                        <tr>
                          <td>{t('pages.subconverter.inbounds')}</td>
                          <td>{renderInboundTags(infoRecord)}</td>
                        </tr>
                      </tbody>
                    </table>

                    <Divider plain>{t('pages.subconverter.feedUrl')}</Divider>
                    <div className="link-panel">
                      <div className="link-panel-header">
                        <Tag color="lime">{t('pages.subconverter.feedUrl')}</Tag>
                        <Tooltip title={t('copy')}>
                          <Button size="small" icon={<CopyOutlined />} onClick={() => onCopy(infoFeedUrl)} />
                        </Tooltip>
                      </div>
                      <code className="link-panel-text">{infoFeedUrl}</code>
                    </div>
                  </Space>
                ),
              },
              {
                key: 'ips',
                label: t('pages.subconverter.boundIps'),
                children: (
                  <Space direction="vertical" size={12} className="subconverter-stack">
                    <div className="subconverter-info-actions">
                      <Button danger icon={<DeleteOutlined />} disabled={boundIps.length === 0} onClick={() => onClearBoundIps(infoRecord)}>
                        {t('pages.subconverter.clearIps')}
                      </Button>
                    </div>
                    {isMobile ? renderBoundIpCards() : (
                      <Table
                        rowKey="id"
                        size="small"
                        columns={boundIpColumns}
                        dataSource={boundIps}
                        pagination={false}
                        scroll={{ x: 620 }}
                      />
                    )}
                  </Space>
                ),
              },
            ]}
          />
        ) : (
          <div className="subconverter-info-loading" />
        )}
      </Spin>
    </Modal>
  );
}
