import { useCallback, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import {
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  FilterOutlined,
  InfoCircleOutlined,
  RetweetOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { Button, Input, Popover, Radio, Select, Space, Switch, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { useMediaQuery } from '@/hooks/useMediaQuery';
import { ColorUtils, SizeFormatter } from '@/utils';
import { InfinityIcon } from '@/components/ui';
import type { InboundOption, SubscriptionRecord } from './types';
import {
  buildFeedUrl,
  clientTrafficTotal,
  clientTrafficUsed,
  filterSubscriptions,
  formatIpLimitUsage,
  getCommonClientDetails,
  getSubscriptionProtocolOptions,
  ipLimitSortValue,
  ipLimitTagColor,
} from './utils';
import SubconverterMobileList from './SubconverterMobileList';

interface SubconverterSubscriptionListProps {
  rows: SubscriptionRecord[];
  pageSize: number;
  inboundById: Map<number, InboundOption>;
  supportedInbounds: InboundOption[];
  inboundTagLabel: (id: number) => string;
  renderInboundTags: (record: SubscriptionRecord) => ReactNode;
  togglingId: number | null;
  onInfo: (record: SubscriptionRecord) => void;
  onEdit: (record: SubscriptionRecord) => void;
  onCopy: (text: string) => void;
  onResetToken: (record: SubscriptionRecord) => void;
  onToggleEnabled: (record: SubscriptionRecord, checked: boolean) => void;
  onRemove: (record: SubscriptionRecord) => void;
}

export default function SubconverterSubscriptionList({
  rows,
  pageSize,
  inboundById,
  supportedInbounds,
  inboundTagLabel,
  renderInboundTags,
  togglingId,
  onInfo,
  onEdit,
  onCopy,
  onResetToken,
  onToggleEnabled,
  onRemove,
}: SubconverterSubscriptionListProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [searchKey, setSearchKey] = useState('');
  const [filterBy, setFilterBy] = useState('');
  const [filterMode, setFilterMode] = useState(false);
  const [protocolFilter, setProtocolFilter] = useState<string | undefined>();
  const [inboundFilter, setInboundFilter] = useState<number | undefined>();

  const protocolOptions = useMemo(() => {
    return getSubscriptionProtocolOptions(supportedInbounds);
  }, [supportedInbounds]);

  const filteredRows = useMemo(() => {
    return filterSubscriptions(rows, inboundById, {
      filterMode,
      filterBy,
      searchKey,
      protocolFilter,
      inboundFilter,
    });
  }, [filterBy, filterMode, inboundById, inboundFilter, protocolFilter, rows, searchKey]);

  const resolveSubscriptionClient = useCallback((record: SubscriptionRecord) => {
    if (!record.trafficStats) return undefined;
    const inbounds = record.inbounds || [];
    const inboundIds = inbounds.map((item) => item.inboundId).filter((id) => inboundById.has(id));
    if (inboundIds.length === 0) return undefined;

    const clientDetails = getCommonClientDetails(inboundIds, inboundById);
    const clientEmail = String(inbounds.find((item) => item.clientEmail)?.clientEmail || '').trim();
    if (clientEmail) {
      return clientDetails.find((client) => client.email === clientEmail);
    }
    return clientDetails.length === 1 ? clientDetails[0] : undefined;
  }, [inboundById]);

  const renderClient = useCallback((record: SubscriptionRecord) => {
    const client = resolveSubscriptionClient(record);
    if (!client) return <span className="subconverter-muted">-</span>;
    return (
      <Tooltip title={client.email}>
        <Tag color="green">{client.email}</Tag>
      </Tooltip>
    );
  }, [resolveSubscriptionClient]);

  const renderTraffic = useCallback((record: SubscriptionRecord) => {
    const client = resolveSubscriptionClient(record);
    if (!client) return <span className="subconverter-muted">-</span>;

    const used = clientTrafficUsed(client);
    const total = clientTrafficTotal(client);
    return (
      <Popover
        content={(
          <table cellPadding={2}>
            <tbody>
              <tr>
                <td>↑ {SizeFormatter.sizeFormat(client.up || 0)}</td>
                <td>↓ {SizeFormatter.sizeFormat(client.down || 0)}</td>
              </tr>
              {total > 0 && used < total && (
                <tr>
                  <td>{t('remained')}</td>
                  <td>{SizeFormatter.sizeFormat(total - used)}</td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      >
        <Tag color={ColorUtils.usageColor(used, 0, total)}>
          {SizeFormatter.sizeFormat(used)} /
          {' '}
          {total > 0 ? SizeFormatter.sizeFormat(total) : <InfinityIcon />}
        </Tag>
      </Popover>
    );
  }, [resolveSubscriptionClient, t]);

  const columns = useMemo<ColumnsType<SubscriptionRecord>>(() => [
    {
      title: t('pages.clients.actions'),
      key: 'actions',
      width: 184,
      align: 'left',
      render: (_, record) => (
        <Space size={4}>
          <Tooltip title={t('info')}>
            <Button size="small" type="text" icon={<InfoCircleOutlined />} aria-label={t('info')} onClick={() => onInfo(record)} />
          </Tooltip>
          <Tooltip title={t('edit')}>
            <Button size="small" type="text" icon={<EditOutlined />} aria-label={t('edit')} onClick={() => onEdit(record)} />
          </Tooltip>
          <Tooltip title={t('pages.subconverter.copyFeedUrl')}>
            <Button size="small" type="text" icon={<CopyOutlined />} aria-label={t('pages.subconverter.copyFeedUrl')} onClick={() => onCopy(buildFeedUrl(record.token))} />
          </Tooltip>
          <Tooltip title={t('pages.subconverter.resetToken')}>
            <Button size="small" type="text" icon={<RetweetOutlined />} aria-label={t('pages.subconverter.resetToken')} onClick={() => onResetToken(record)} />
          </Tooltip>
          <Tooltip title={t('delete')}>
            <Button size="small" type="text" danger icon={<DeleteOutlined />} aria-label={t('delete')} onClick={() => onRemove(record)} />
          </Tooltip>
        </Space>
      ),
    },
    {
      title: t('enable'),
      dataIndex: 'enable',
      width: 88,
      align: 'center',
      render: (_, record) => (
        <Switch
          checked={record.enable}
          loading={togglingId === record.id}
          onChange={(checked) => onToggleEnabled(record, checked)}
        />
      ),
    },
    {
      title: t('remark'),
      dataIndex: 'remark',
      ellipsis: true,
      width: 180,
    },
    {
      title: t('pages.subconverter.inbounds'),
      dataIndex: 'inbounds',
      width: 320,
      render: (_, record) => renderInboundTags(record),
    },
    {
      title: t('pages.subconverter.client'),
      key: 'client',
      width: 180,
      render: (_, record) => renderClient(record),
    },
    {
      title: t('pages.inbounds.traffic'),
      key: 'traffic',
      width: 120,
      align: 'center',
      render: (_, record) => renderTraffic(record),
    },
    {
      title: t('pages.subconverter.completedSubscriptions'),
      dataIndex: ['stats', 'completedCount'],
      width: 112,
      align: 'center',
      sorter: (a, b) => (a.stats?.completedCount || 0) - (b.stats?.completedCount || 0),
      showSorterTooltip: false,
      render: (_value, record) => record.stats?.completedCount || 0,
    },
    {
      title: t('pages.subconverter.maxIps'),
      dataIndex: 'limitIp',
      width: 104,
      align: 'center',
      sorter: (a, b) => {
        const limitDiff = ipLimitSortValue(a) - ipLimitSortValue(b);
        if (limitDiff !== 0) return limitDiff;
        return (a.boundIpCount || 0) - (b.boundIpCount || 0);
      },
      showSorterTooltip: false,
      render: (_value, record) => (
        <Tag color={ipLimitTagColor(record)}>{formatIpLimitUsage(record)}</Tag>
      ),
    },
  ], [onCopy, onEdit, onInfo, onRemove, onResetToken, onToggleEnabled, renderClient, renderInboundTags, renderTraffic, t, togglingId]);

  const handleProtocolChange = useCallback((value?: string) => {
    setProtocolFilter(value);
    if (value && inboundFilter) {
      const inbound = inboundById.get(inboundFilter);
      if (!inbound || inbound.protocol !== value) setInboundFilter(undefined);
    }
  }, [inboundById, inboundFilter]);

  return (
    <Space orientation="vertical" size={12} className="subconverter-stack">
      <div className="subconverter-toolbar">
        <Switch
          checked={filterMode}
          checkedChildren={<SearchOutlined />}
          unCheckedChildren={<FilterOutlined />}
          onChange={(checked) => {
            setFilterMode(checked);
            setSearchKey('');
            setFilterBy('');
          }}
        />
        {filterMode ? (
          <Radio.Group
            value={filterBy}
            onChange={(event) => setFilterBy(event.target.value)}
            optionType="button"
            buttonStyle="solid"
            size={isMobile ? 'small' : 'middle'}
          >
            <Radio.Button value="">{t('none')}</Radio.Button>
            <Radio.Button value="enabled">{t('enabled')}</Radio.Button>
            <Radio.Button value="disabled">{t('disabled')}</Radio.Button>
          </Radio.Group>
        ) : (
          <Input
            allowClear
            value={searchKey}
            prefix={<SearchOutlined />}
            placeholder={t('pages.subconverter.searchPlaceholder')}
            size={isMobile ? 'small' : 'middle'}
            className="subconverter-search"
            onChange={(event) => setSearchKey(event.target.value)}
          />
        )}
        <Select
          value={protocolFilter}
          onChange={handleProtocolChange}
          allowClear
          placeholder={t('pages.inbounds.protocol')}
          size={isMobile ? 'small' : 'middle'}
          style={{ width: 150 }}
          options={protocolOptions.map((protocol) => ({ value: protocol, label: protocol }))}
        />
        <Select
          value={inboundFilter}
          onChange={(value) => setInboundFilter(value)}
          allowClear
          showSearch
          optionFilterProp="label"
          placeholder={t('pages.subconverter.inbounds')}
          size={isMobile ? 'small' : 'middle'}
          style={{ minWidth: 160, maxWidth: 260 }}
          options={supportedInbounds
            .filter((inbound) => !protocolFilter || inbound.protocol === protocolFilter)
            .map((inbound) => ({
              value: inbound.id,
              label: inboundTagLabel(inbound.id),
            }))}
        />
      </div>

      {isMobile ? (
        <SubconverterMobileList
          rows={filteredRows}
          togglingId={togglingId}
          renderInboundTags={renderInboundTags}
          renderClient={renderClient}
          renderTraffic={renderTraffic}
          onInfo={onInfo}
          onEdit={onEdit}
          onCopy={onCopy}
          onResetToken={onResetToken}
          onToggleEnabled={onToggleEnabled}
          onRemove={onRemove}
        />
      ) : (
        <Table
          rowKey="id"
          size="small"
          columns={columns}
          dataSource={filteredRows}
          scroll={{ x: 1300 }}
          pagination={filteredRows.length > pageSize ? {
            pageSize,
            showSizeChanger: true,
            pageSizeOptions: ['10', '25', '50', '100', '200'],
          } : false}
        />
      )}
    </Space>
  );
}
