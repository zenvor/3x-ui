import { useCallback, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  FilterOutlined,
  InfoCircleOutlined,
  QrcodeOutlined,
  RetweetOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { Button, Input, Popover, Radio, Select, Space, Switch, Table, Tag, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { useMediaQuery } from '@/hooks/useMediaQuery';
import { SizeFormatter } from '@/utils';
import { InfinityIcon } from '@/components/ui';
import { QrPanel } from '@/pages/inbounds/qr';
import type { InboundOption, SubscriptionRecord } from './types';
import {
  buildFeedUrl,
  clientTrafficTotal,
  clientTrafficUsed,
  filterSubscriptions,
  formatIpLimitUsage,
  getSubscriptionProtocolOptions,
  INBOUND_TAG_COLOR,
  ipLimitSortValue,
  ipLimitTagColor,
  resolveSubscriptionClient,
} from './utils';
import SubconverterMobileList from './SubconverterMobileList';

const ACTION_ICON_STYLE = { fontSize: 18 };
const INBOUND_CHIP_LIMIT = 1;
const TABLE_SCROLL_X = 974;

interface SubconverterSubscriptionListProps {
  rows: SubscriptionRecord[];
  pageSize: number;
  inboundById: Map<number, InboundOption>;
  supportedInbounds: InboundOption[];
  inboundTagLabel: (id: number) => string;
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

  const renderTraffic = useCallback((record: SubscriptionRecord) => {
    const client = resolveSubscriptionClient(record, inboundById);
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
        <Tag color="purple">
          {SizeFormatter.sizeFormat(used)} /
          {' '}
          {total > 0 ? SizeFormatter.sizeFormat(total) : <InfinityIcon />}
        </Tag>
      </Popover>
    );
  }, [inboundById, t]);

  const renderTableInboundTags = useCallback((record: SubscriptionRecord) => {
    const inboundIds = (record.inbounds || [])
      .filter((item) => inboundById.has(item.inboundId))
      .sort((a, b) => (a.sortOrder || 0) - (b.sortOrder || 0))
      .map((item) => item.inboundId);
    if (inboundIds.length === 0) return <span className="subconverter-muted">—</span>;

    const visible = inboundIds.slice(0, INBOUND_CHIP_LIMIT);
    const overflow = inboundIds.slice(INBOUND_CHIP_LIMIT);
    const chip = (id: number) => {
      const label = inboundTagLabel(id);
      return (
        <Tooltip key={id} title={label}>
          <Tag color={INBOUND_TAG_COLOR} style={{ margin: 2 }}>
            {label}
          </Tag>
        </Tooltip>
      );
    };

    return (
      <>
        {visible.map((id) => chip(id))}
        {overflow.length > 0 && (
          <Popover
            trigger="click"
            placement="bottomRight"
            content={
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4, maxWidth: 280, maxHeight: 280, overflowY: 'auto' }}>
                {overflow.map((id) => chip(id))}
              </div>
            }
          >
            <Button
              type="text"
              size="small"
              className="subconverter-chip-more-button"
              aria-label={`${t('more')} ${overflow.length}`}
            >
              +{overflow.length}
            </Button>
          </Popover>
        )}
      </>
    );
  }, [inboundById, inboundTagLabel, t]);

  const columns = useMemo<ColumnsType<SubscriptionRecord>>(() => [
    {
      title: t('pages.clients.actions'),
      key: 'actions',
      width: 200,
      render: (_, record) => (
        <Space size={4}>
          <Popover
            trigger="click"
            placement="bottom"
            destroyOnHidden
            content={<QrPanel value={buildFeedUrl(record.token)} remark={record.remark || record.token} size={220} />}
          >
            <Tooltip title={t('pages.clients.qrCode')}>
              <Button size="small" type="text" style={ACTION_ICON_STYLE} icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
            </Tooltip>
          </Popover>
          <Tooltip title={t('copy')}>
            <Button size="small" type="text" style={ACTION_ICON_STYLE} icon={<CopyOutlined />} aria-label={t('pages.subconverter.copyFeedUrl')} onClick={() => onCopy(buildFeedUrl(record.token))} />
          </Tooltip>
          <Tooltip title={t('info')}>
            <Button size="small" type="text" style={ACTION_ICON_STYLE} icon={<InfoCircleOutlined />} aria-label={t('info')} onClick={() => onInfo(record)} />
          </Tooltip>
          <Tooltip title={t('pages.subconverter.resetToken')}>
            <Button size="small" type="text" style={ACTION_ICON_STYLE} icon={<RetweetOutlined />} aria-label={t('pages.subconverter.resetToken')} onClick={() => onResetToken(record)} />
          </Tooltip>
          <Tooltip title={t('edit')}>
            <Button size="small" type="text" style={ACTION_ICON_STYLE} icon={<EditOutlined />} aria-label={t('edit')} onClick={() => onEdit(record)} />
          </Tooltip>
          <Tooltip title={t('delete')}>
            <Button size="small" type="text" danger style={ACTION_ICON_STYLE} icon={<DeleteOutlined />} aria-label={t('delete')} onClick={() => onRemove(record)} />
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
          size="small"
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
      width: 170,
      render: (_, record) => renderTableInboundTags(record),
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
  ], [onCopy, onEdit, onInfo, onRemove, onResetToken, onToggleEnabled, renderTableInboundTags, renderTraffic, t, togglingId]);

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
          renderInboundTags={renderTableInboundTags}
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
          scroll={{ x: TABLE_SCROLL_X }}
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
