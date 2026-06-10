import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Checkbox, Form, Input, Modal, Select, Space, Tag } from 'antd';
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';

import { keys } from '@/api/queryKeys';
import { fetchSubconverterAccessLogs } from '@/api/queries/useSubconverter';
import { FileManager, IntlUtil, PromiseUtil } from '@/utils';
import { useDatepicker } from '@/hooks/useDatepicker';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import type { AccessLogRecord } from './types';
import './SubconverterAccessLogModal.css';

interface SubconverterAccessLogModalProps {
  open: boolean;
  onClose: () => void;
}

const DEFAULT_ENDPOINT_OPTIONS = ['full', 'nodes'];
const DEFAULT_RESULT_OPTIONS = [
  'success',
  'ua_rejected',
  'ip_limit_exceeded',
  'subscription_disabled',
  'ip_missing',
  'internal_error',
];
const EMPTY_FILTER_VALUE = '__empty__';

function normalizeFilterValue(value?: string): string {
  return value || EMPTY_FILTER_VALUE;
}

function statusColor(statusCode: number): string {
  return statusCode >= 400 ? 'red' : 'green';
}

function resultColor(result: string): string {
  switch (result) {
    case 'success':
      return 'green';
    case 'ua_rejected':
      return 'orange';
    case 'ip_limit_exceeded':
    case 'internal_error':
      return 'red';
    case 'subscription_disabled':
      return 'default';
    case 'ip_missing':
      return 'gold';
    default:
      return 'blue';
  }
}

function resultClassName(result: string): string {
  switch (result) {
    case 'success':
      return 'log-row-success';
    case 'ua_rejected':
      return 'log-row-warning';
    case 'ip_limit_exceeded':
    case 'internal_error':
      return 'log-row-error';
    case 'subscription_disabled':
      return 'log-row-muted';
    case 'ip_missing':
      return 'log-row-warning';
    default:
      return '';
  }
}

function shortTime(value?: string): string {
  if (!value) return '';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  const ss = String(d.getSeconds()).padStart(2, '0');
  return `${hh}:${mm}:${ss}`;
}

export default function SubconverterAccessLogModal({ open, onClose }: SubconverterAccessLogModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const { isMobile } = useMediaQuery();
  const [rows, setRows] = useState('100');
  const [filter, setFilter] = useState('');
  const [selectedEndpoints, setSelectedEndpoints] = useState<string[]>(DEFAULT_ENDPOINT_OPTIONS);
  const [selectedResults, setSelectedResults] = useState<string[]>(DEFAULT_RESULT_OPTIONS);
  const endpointFilterTouchedRef = useRef(false);
  const resultFilterTouchedRef = useRef(false);
  const logsQuery = useQuery({
    queryKey: keys.subconverter.logs(rows),
    queryFn: () => fetchSubconverterAccessLogs(rows),
    enabled: open,
  });
  const logs = useMemo(() => logsQuery.data ?? [], [logsQuery.data]);
  const loading = logsQuery.isFetching;
  const { refetch } = logsQuery;

  const endpointLabel = useCallback((value: string) => {
    if (value === EMPTY_FILTER_VALUE) return '-';
    if (value === 'full' || value === 'nodes') {
      return t(`pages.subconverter.endpointValues.${value}`);
    }
    return value || '-';
  }, [t]);

  const resultLabel = useCallback((value: string) => {
    if (value === EMPTY_FILTER_VALUE) return '-';
    switch (value) {
      case 'success':
      case 'ua_rejected':
      case 'ip_limit_exceeded':
      case 'subscription_disabled':
      case 'ip_missing':
      case 'internal_error':
        return t(`pages.subconverter.accessResult.${value}`);
      default:
        return value || '-';
    }
  }, [t]);

  const endpointOptions = useMemo(() => {
    const values = [...DEFAULT_ENDPOINT_OPTIONS];
    const seen = new Set(values);
    for (const log of logs) {
      const value = normalizeFilterValue(log.endpoint);
      if (seen.has(value)) continue;
      seen.add(value);
      values.push(value);
    }
    return values;
  }, [logs]);

  const resultOptions = useMemo(() => {
    const values = [...DEFAULT_RESULT_OPTIONS];
    const seen = new Set(values);
    for (const log of logs) {
      const value = normalizeFilterValue(log.result);
      if (seen.has(value)) continue;
      seen.add(value);
      values.push(value);
    }
    return values;
  }, [logs]);

  const filteredLogs = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const endpointSet = new Set(selectedEndpoints);
    const resultSet = new Set(selectedResults);
    return logs.filter((log) => {
      if (endpointOptions.length > 0 && !endpointSet.has(normalizeFilterValue(log.endpoint))) return false;
      if (resultOptions.length > 0 && !resultSet.has(normalizeFilterValue(log.result))) return false;
      if (!q) return true;
      return [
        `#${log.subscriptionId}`,
        log.subscriptionRemark || '',
        endpointLabel(log.endpoint),
        String(log.statusCode),
        resultLabel(log.result),
        log.ip || '',
        log.userAgent || '',
        IntlUtil.formatDate(log.accessedAt, datepicker) || '',
      ].some((value) => value.toLowerCase().includes(q));
    });
  }, [
    datepicker,
    endpointLabel,
    endpointOptions,
    filter,
    logs,
    resultLabel,
    resultOptions,
    selectedEndpoints,
    selectedResults,
  ]);

  const refresh = useCallback(async () => {
    await refetch();
    await PromiseUtil.sleep(300);
  }, [refetch]);

  useEffect(() => {
    if (endpointOptions.length === 0) return;
    setSelectedEndpoints((prev) => {
      if (!endpointFilterTouchedRef.current) {
        return endpointOptions;
      }
      return prev.filter((value) => endpointOptions.includes(value));
    });
  }, [endpointOptions]);

  useEffect(() => {
    if (resultOptions.length === 0) return;
    setSelectedResults((prev) => {
      if (!resultFilterTouchedRef.current) {
        return resultOptions;
      }
      return prev.filter((value) => resultOptions.includes(value));
    });
  }, [resultOptions]);

  const fullDate = useCallback((value?: string) => IntlUtil.formatDate(value, datepicker), [datepicker]);

  const subscriptionLabel = useCallback((log: AccessLogRecord) => (
    `#${log.subscriptionId}${log.subscriptionRemark ? ` ${log.subscriptionRemark}` : ''}`
  ), []);

  const toggleEndpointFilter = useCallback((value: string, checked: boolean) => {
    endpointFilterTouchedRef.current = true;
    setSelectedEndpoints((prev) => {
      if (checked) return prev.includes(value) ? prev : [...prev, value];
      return prev.filter((item) => item !== value);
    });
  }, []);

  const toggleResultFilter = useCallback((value: string, checked: boolean) => {
    resultFilterTouchedRef.current = true;
    setSelectedResults((prev) => {
      if (checked) return prev.includes(value) ? prev : [...prev, value];
      return prev.filter((item) => item !== value);
    });
  }, []);

  const download = useCallback(() => {
    if (!Array.isArray(filteredLogs) || filteredLogs.length === 0) {
      FileManager.downloadTextFile('', 'subconverter-access.log');
      return;
    }
    const lines = filteredLogs.map((log) => [
      fullDate(log.accessedAt),
      subscriptionLabel(log),
      `endpoint=${endpointLabel(log.endpoint)}`,
      `status=${log.statusCode}`,
      `result=${resultLabel(log.result)}`,
      `ip=${log.ip || ''}`,
      `ua=${log.userAgent || ''}`,
    ].join(' '));
    FileManager.downloadTextFile(lines.join('\n'), 'subconverter-access.log');
  }, [endpointLabel, filteredLogs, fullDate, resultLabel, subscriptionLabel]);

  return (
    <Modal
      open={open}
      footer={null}
      width={isMobile ? '100vw' : '80vw'}
      style={isMobile ? { top: 0, paddingBottom: 0, maxWidth: '100vw' } : undefined}
      className={isMobile ? 'subconverter-access-log-modal-mobile' : undefined}
      onCancel={onClose}
      title={(
        <span className="subconverter-access-log-title">
          {t('pages.subconverter.accessLogs')}
          <SyncOutlined
            spin={loading}
            className="subconverter-access-log-reload"
            role="button"
            aria-label={t('refresh')}
            onClick={refresh}
          />
        </span>
      )}
    >
      <Form layout="inline" className="subconverter-access-log-toolbar">
        <Form.Item className="rows-item">
          <Select value={rows} size="small" style={{ width: 70 }} onChange={setRows}>
            <Select.Option value="10">10</Select.Option>
            <Select.Option value="20">20</Select.Option>
            <Select.Option value="50">50</Select.Option>
            <Select.Option value="100">100</Select.Option>
            <Select.Option value="500">500</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item label={t('filter')} className="filter-item">
          <Input
            value={filter}
            size="small"
            onChange={(event) => setFilter(event.target.value)}
            onKeyUp={(event) => {
              if (event.key === 'Enter') refresh();
            }}
          />
        </Form.Item>
        {(endpointOptions.length > 0 || resultOptions.length > 0) && (
          <Form.Item className="result-filter-item">
            <div className="subconverter-access-log-filter-options">
              {endpointOptions.map((endpoint) => (
                <Checkbox
                  key={`endpoint-${endpoint}`}
                  checked={selectedEndpoints.includes(endpoint)}
                  onChange={(event) => toggleEndpointFilter(endpoint, event.target.checked)}
                >
                  {endpointLabel(endpoint)}
                </Checkbox>
              ))}
              {resultOptions.map((result) => (
                <Checkbox
                  key={`result-${result}`}
                  checked={selectedResults.includes(result)}
                  onChange={(event) => toggleResultFilter(result, event.target.checked)}
                >
                  {resultLabel(result)}
                </Checkbox>
              ))}
            </div>
          </Form.Item>
        )}
        <Form.Item className="download-item">
          <Button size="small" type="primary" onClick={download} icon={<DownloadOutlined />} aria-label={t('download')} />
        </Form.Item>
      </Form>

      <div className={`subconverter-access-log-container ${isMobile ? 'is-mobile' : ''}`}>
        {filteredLogs.length === 0 ? (
          <div className="subconverter-access-log-empty">{t('noData')}</div>
        ) : isMobile ? (
          filteredLogs.map((log) => (
            <div key={log.id} className="subconverter-access-log-card">
              <div className="subconverter-access-log-card-head">
                <span className="subconverter-access-log-time" title={fullDate(log.accessedAt)}>
                  {shortTime(log.accessedAt)}
                </span>
                <Space size={[4, 4]} wrap>
                  <Tag color={log.endpoint === 'nodes' ? 'purple' : 'green'}>{endpointLabel(log.endpoint)}</Tag>
                  <Tag color={statusColor(log.statusCode)}>{log.statusCode}</Tag>
                </Space>
              </div>
              <div className="subconverter-access-log-subscription">{subscriptionLabel(log)}</div>
              <div className="subconverter-access-log-route">
                <span>{log.ip || '-'}</span>
                <span className="subconverter-access-log-separator">/</span>
                <Tag color={resultColor(log.result)}>{resultLabel(log.result)}</Tag>
              </div>
              <div className="subconverter-access-log-ua">{log.userAgent || '-'}</div>
            </div>
          ))
        ) : (
          <table className="subconverter-access-log-table">
            <thead>
              <tr>
                <th>{t('pages.subconverter.accessedAt')}</th>
                <th>{t('pages.subconverter.subscription')}</th>
                <th>{t('pages.subconverter.endpoint')}</th>
                <th>{t('pages.subconverter.statusCode')}</th>
                <th>{t('pages.subconverter.result')}</th>
                <th>IP</th>
                <th>UA</th>
              </tr>
            </thead>
            <tbody>
              {filteredLogs.map((log) => (
                <tr key={log.id} className={resultClassName(log.result)}>
                  <td><b>{fullDate(log.accessedAt)}</b></td>
                  <td>{subscriptionLabel(log)}</td>
                  <td>{endpointLabel(log.endpoint)}</td>
                  <td>{log.statusCode}</td>
                  <td>{resultLabel(log.result)}</td>
                  <td>{log.ip || '-'}</td>
                  <td className="subconverter-access-log-ua-cell">{log.userAgent || '-'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Modal>
  );
}
