import { useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import {
  ContainerOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  InfoCircleOutlined,
  KeyOutlined,
} from '@ant-design/icons';
import { Switch, Tag, Tooltip } from 'antd';

import type { SubscriptionRecord } from './types';
import { buildFeedUrl, formatIpLimitUsage, ipLimitTagColor } from './utils';

interface SubconverterMobileListProps {
  rows: SubscriptionRecord[];
  togglingId: number | null;
  renderInboundTags: (record: SubscriptionRecord) => ReactNode;
  onInfo: (record: SubscriptionRecord) => void;
  onEdit: (record: SubscriptionRecord) => void;
  onCopy: (text: string) => void;
  onResetToken: (record: SubscriptionRecord) => void;
  onToggleEnabled: (record: SubscriptionRecord, checked: boolean) => void;
  onRemove: (record: SubscriptionRecord) => void;
}

export default function SubconverterMobileList({
  rows,
  togglingId,
  renderInboundTags,
  onInfo,
  onEdit,
  onCopy,
  onResetToken,
  onToggleEnabled,
  onRemove,
}: SubconverterMobileListProps) {
  const { t } = useTranslation();

  if (rows.length === 0) {
    return (
      <div className="subconverter-card-empty">
        <ContainerOutlined style={{ fontSize: 28, opacity: 0.5 }} />
        <div>{t('noData')}</div>
      </div>
    );
  }

  return (
    <div className="subconverter-cards">
      {rows.map((record) => (
        <div key={record.id} className="subconverter-card">
          <div className="subconverter-card-head">
            <span className="subconverter-card-id">#{record.id}</span>
            <span className="subconverter-card-title">{record.remark || '—'}</span>
            <div className="subconverter-card-actions">
              <Tooltip title={t('info')}>
                <InfoCircleOutlined className="row-action-trigger" onClick={() => onInfo(record)} />
              </Tooltip>
              <Tooltip title={t('edit')}>
                <EditOutlined className="row-action-trigger" onClick={() => onEdit(record)} />
              </Tooltip>
              <Tooltip title={t('pages.subconverter.copyFeedUrl')}>
                <CopyOutlined className="row-action-trigger" onClick={() => onCopy(buildFeedUrl(record.token))} />
              </Tooltip>
              <Tooltip title={t('pages.subconverter.resetToken')}>
                <KeyOutlined className="row-action-trigger danger" onClick={() => onResetToken(record)} />
              </Tooltip>
              <Switch
                checked={record.enable}
                size="small"
                loading={togglingId === record.id}
                onChange={(checked) => onToggleEnabled(record, checked)}
              />
              <Tooltip title={t('delete')}>
                <DeleteOutlined className="row-action-trigger danger" onClick={() => onRemove(record)} />
              </Tooltip>
            </div>
          </div>
          <div className="subconverter-card-meta">
            <div className="subconverter-card-row">
              <span>{t('pages.subconverter.inbounds')}</span>
              <div>{renderInboundTags(record)}</div>
            </div>
            <div className="subconverter-card-row">
              <span>{t('pages.subconverter.completedSubscriptions')}</span>
              <Tag color="cyan">{record.stats?.completedCount || 0}</Tag>
            </div>
            <div className="subconverter-card-row">
              <span>{t('pages.subconverter.maxIps')}</span>
              <Tag color={ipLimitTagColor(record)}>
                {formatIpLimitUsage(record)}
              </Tag>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
