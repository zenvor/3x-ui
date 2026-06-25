import { useTranslation } from 'react-i18next';
import type { ReactNode } from 'react';
import {
  ContainerOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  InfoCircleOutlined,
  RetweetOutlined,
} from '@ant-design/icons';
import { Button, Switch, Tag, Tooltip } from 'antd';

import type { SubscriptionRecord } from './types';
import { buildFeedUrl, formatIpLimitUsage, ipLimitTagColor } from './utils';

interface SubconverterMobileListProps {
  rows: SubscriptionRecord[];
  togglingId: number | null;
  renderInboundTags: (record: SubscriptionRecord) => ReactNode;
  renderClient: (record: SubscriptionRecord) => ReactNode;
  renderTraffic: (record: SubscriptionRecord) => ReactNode;
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
  renderClient,
  renderTraffic,
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
                <Button
                  type="text"
                  size="small"
                  icon={<InfoCircleOutlined />}
                  className="row-action-trigger"
                  aria-label={t('info')}
                  onClick={() => onInfo(record)}
                />
              </Tooltip>
              <Tooltip title={t('edit')}>
                <Button
                  type="text"
                  size="small"
                  icon={<EditOutlined />}
                  className="row-action-trigger"
                  aria-label={t('edit')}
                  onClick={() => onEdit(record)}
                />
              </Tooltip>
              <Tooltip title={t('pages.subconverter.copyFeedUrl')}>
                <Button
                  type="text"
                  size="small"
                  icon={<CopyOutlined />}
                  className="row-action-trigger"
                  aria-label={t('pages.subconverter.copyFeedUrl')}
                  onClick={() => onCopy(buildFeedUrl(record.token))}
                />
              </Tooltip>
              <Tooltip title={t('pages.subconverter.resetToken')}>
                <Button
                  type="text"
                  size="small"
                  icon={<RetweetOutlined />}
                  className="row-action-trigger"
                  aria-label={t('pages.subconverter.resetToken')}
                  onClick={() => onResetToken(record)}
                />
              </Tooltip>
              <Switch
                checked={record.enable}
                size="small"
                loading={togglingId === record.id}
                aria-label={record.enable ? t('enabled') : t('disabled')}
                onChange={(checked) => onToggleEnabled(record, checked)}
              />
              <Tooltip title={t('delete')}>
                <Button
                  danger
                  type="text"
                  size="small"
                  icon={<DeleteOutlined />}
                  className="row-action-trigger"
                  aria-label={t('delete')}
                  onClick={() => onRemove(record)}
                />
              </Tooltip>
            </div>
          </div>
          <div className="subconverter-card-meta">
            <div className="subconverter-card-row">
              <span>{t('pages.subconverter.inbounds')}</span>
              <div>{renderInboundTags(record)}</div>
            </div>
            <div className="subconverter-card-row">
              <span>{t('pages.subconverter.client')}</span>
              <div>{renderClient(record)}</div>
            </div>
            <div className="subconverter-card-row">
              <span>{t('pages.inbounds.traffic')}</span>
              <div>{renderTraffic(record)}</div>
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
