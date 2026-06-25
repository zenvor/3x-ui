import { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Col, Form, Input, InputNumber, Modal, Row, Select, Space, Switch, Tag } from 'antd';
import type { FormInstance } from 'antd/es/form';

import { SelectAllClearButtons } from '@/components/form';
import { SizeFormatter } from '@/utils';
import type { FormValues, InboundOption } from './types';
import {
  clientTrafficTotal,
  clientTrafficUsed,
  getCommonClientDetails,
  getCommonClientEmails,
  INBOUND_TAG_COLOR,
  isClientDepleted,
} from './utils';

interface SubconverterSubscriptionModalProps {
  open: boolean;
  editingId: number | null;
  saving: boolean;
  form: FormInstance<FormValues>;
  supportedInbounds: InboundOption[];
  inboundSelectLabel: (id: number) => string;
  inboundTagLabel: (id: number) => string;
  canConfigureCdnTls: (id: number) => boolean;
  isCdnTlsRequired: (id: number) => boolean;
  onOk: () => void;
  onCancel: () => void;
}

export default function SubconverterSubscriptionModal({
  open,
  editingId,
  saving,
  form,
  supportedInbounds,
  inboundSelectLabel,
  inboundTagLabel,
  canConfigureCdnTls,
  isCdnTlsRequired,
  onOk,
  onCancel,
}: SubconverterSubscriptionModalProps) {
  const { t } = useTranslation();
  const watchedInboundIds = Form.useWatch('inboundIds', form);
  const watchedCdnTls = Form.useWatch('cdnTls', form);
  const watchedClientEmail = Form.useWatch('clientEmail', form);
  const watchedTrafficStats = Form.useWatch('trafficStats', form);
  const selectedInboundIds = useMemo(() => watchedInboundIds || [], [watchedInboundIds]);
  const trafficStatsEnabled = !!watchedTrafficStats;
  const cdnTls = useMemo(() => watchedCdnTls || {}, [watchedCdnTls]);
  const inboundById = useMemo(() => {
    const out = new Map<number, InboundOption>();
    for (const inbound of supportedInbounds) out.set(inbound.id, inbound);
    return out;
  }, [supportedInbounds]);
  const commonClientEmails = useMemo(
    () => getCommonClientEmails(selectedInboundIds, inboundById),
    [inboundById, selectedInboundIds],
  );
  const commonClientDetails = useMemo(
    () => getCommonClientDetails(selectedInboundIds, inboundById),
    [inboundById, selectedInboundIds],
  );
  const clientOptions = useMemo(
    () => commonClientEmails.map((email) => ({ value: email, label: email })),
    [commonClientEmails],
  );
  const hasSelectedInbounds = selectedInboundIds.length > 0;
  const hasNoCommonClient = trafficStatsEnabled && hasSelectedInbounds && commonClientEmails.length === 0;
  const requiresClientChoice = trafficStatsEnabled && commonClientEmails.length > 1;
  const selectedClientEmail = String(watchedClientEmail || '').trim();
  const clientSelectionInvalid = hasNoCommonClient || (requiresClientChoice && !selectedClientEmail);
  const selectedClientDetail = useMemo(() => {
    const email = selectedClientEmail
      || (commonClientEmails.length === 1 ? commonClientEmails[0] : '')
      || (commonClientDetails.length === 1 ? commonClientDetails[0]?.email : '');
    if (!email) return undefined;
    return commonClientDetails.find((client) => client.email === email);
  }, [commonClientDetails, commonClientEmails, selectedClientEmail]);
  const clientDisplayEmail = selectedClientEmail
    || selectedClientDetail?.email
    || (commonClientEmails.length === 1 ? commonClientEmails[0] : '');
  const trafficLimitDisplay = useMemo(() => {
    if (!selectedClientDetail) return undefined;
    const total = clientTrafficTotal(selectedClientDetail);
    const used = clientTrafficUsed(selectedClientDetail);
    const remaining = total > 0 ? Math.max(total - used, 0) : -1;
    const totalText = total > 0 ? SizeFormatter.sizeFormat(total) : '∞';
    return {
      up: SizeFormatter.sizeFormat(selectedClientDetail.up || 0),
      down: SizeFormatter.sizeFormat(selectedClientDetail.down || 0),
      used: SizeFormatter.sizeFormat(used),
      total: totalText,
      remaining,
      remainingText: remaining < 0 ? '∞' : SizeFormatter.sizeFormat(remaining),
    };
  }, [selectedClientDetail]);
  const clientProblem = useMemo(() => {
    if (!hasNoCommonClient) return undefined;
    const depleted = commonClientDetails.find((client) => isClientDepleted(client));
    if (depleted) return { client: depleted, label: t('depleted'), color: 'red' };
    const disabled = commonClientDetails.find((client) => client.enable === false);
    if (disabled) return { client: disabled, label: t('disabled'), color: 'default' };
    const missingId = commonClientDetails.find((client) => client.hasId === false);
    if (missingId) return { client: missingId, label: 'ID', color: 'orange' };
    return undefined;
  }, [commonClientDetails, hasNoCommonClient, t]);
  const inboundOptions = useMemo(
    () => supportedInbounds.map((inbound) => ({
      value: inbound.id,
      label: inboundSelectLabel(inbound.id),
      title: inboundTagLabel(inbound.id),
    })),
    [inboundSelectLabel, inboundTagLabel, supportedInbounds],
  );
  const cdnTlsInboundIds = useMemo(
    () => selectedInboundIds.filter((id) => canConfigureCdnTls(id)),
    [canConfigureCdnTls, selectedInboundIds],
  );
  const cdnFieldLayout = { labelCol: { span: 24 }, wrapperCol: { span: 24 } };

  useEffect(() => {
    const current = String(form.getFieldValue('clientEmail') || '').trim();
    if (!trafficStatsEnabled) {
      if (current) {
        form.setFieldsValue({ clientEmail: undefined });
      }
      return;
    }
    if (commonClientEmails.length === 1) {
      if (current !== commonClientEmails[0]) {
        form.setFieldsValue({ clientEmail: commonClientEmails[0] });
      }
      return;
    }
    if (current && !commonClientEmails.includes(current)) {
      form.setFieldsValue({ clientEmail: undefined });
    }
  }, [commonClientEmails, form, trafficStatsEnabled]);

  return (
    <Modal
      open={open}
      title={editingId == null ? t('pages.subconverter.create') : t('pages.subconverter.edit')}
      okText={t('confirm')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      width={760}
      okButtonProps={{ disabled: clientSelectionInvalid }}
      onOk={onOk}
      onCancel={onCancel}
      destroyOnHidden
    >
      <Form<FormValues>
        form={form}
        colon={false}
        labelCol={{ sm: { span: 8 } }}
        wrapperCol={{ sm: { span: 14 } }}
        initialValues={{ remark: '', limitIp: 0, enable: true, trafficStats: false, inboundIds: [], cdnTls: {} }}
      >
        <Row gutter={12}>
          <Col xs={24} md={12}>
            <Form.Item {...cdnFieldLayout} name="remark" label={t('remark')}>
              <Input placeholder={t('remark')} />
            </Form.Item>
          </Col>
          <Col xs={24} md={4}>
            <Form.Item {...cdnFieldLayout} name="enable" label={t('enable')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col xs={24} md={6}>
            <Form.Item
              {...cdnFieldLayout}
              name="limitIp"
              label={t('pages.subconverter.maxIps')}
              tooltip={t('pages.subconverter.maxIpsHint')}
            >
              <InputNumber min={0} className="subconverter-full-width" />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item
              {...cdnFieldLayout}
              label={t('pages.subconverter.trafficStats')}
              tooltip={t('pages.subconverter.trafficStatsHint')}
            >
              <Space wrap size={12}>
                <Form.Item name="trafficStats" valuePropName="checked" noStyle>
                  <Switch />
                </Form.Item>
                {(requiresClientChoice || trafficLimitDisplay) && (
                  <div className="subconverter-traffic-summary">
                    {(requiresClientChoice || clientDisplayEmail) && (
                      <div className="subconverter-traffic-row">
                        <span className="subconverter-traffic-label">{t('pages.subconverter.client')}</span>
                        {requiresClientChoice ? (
                          <Form.Item name="clientEmail" noStyle>
                            <Select
                              className="subconverter-traffic-client-select"
                              placeholder={t('pages.subconverter.clientPlaceholder')}
                              options={clientOptions}
                              showSearch
                              optionFilterProp="label"
                            />
                          </Form.Item>
                        ) : (
                          <Tag color="green" className="subconverter-traffic-tag">{clientDisplayEmail}</Tag>
                        )}
                      </div>
                    )}
                    {trafficLimitDisplay && (
                      <>
                        <div className="subconverter-traffic-row">
                          <span className="subconverter-traffic-label">{t('pages.inbounds.traffic')}</span>
                          <Tag className="subconverter-traffic-tag">
                            ↑ {trafficLimitDisplay.up}
                            {' '}/ ↓ {trafficLimitDisplay.down}
                          </Tag>
                          <span className="subconverter-traffic-hint">
                            {trafficLimitDisplay.used} / {trafficLimitDisplay.total}
                          </span>
                        </div>
                        <div className="subconverter-traffic-row">
                          <span className="subconverter-traffic-label">{t('pages.clients.remaining')}</span>
                          <Tag
                            className="subconverter-traffic-tag"
                            color={trafficLimitDisplay.remaining === 0 ? 'red' : undefined}
                          >
                            {trafficLimitDisplay.remainingText}
                          </Tag>
                        </div>
                      </>
                    )}
                  </div>
                )}
                {selectedClientDetail && isClientDepleted(selectedClientDetail) && (
                  <Tag color="red">{t('depleted')}</Tag>
                )}
                {selectedClientDetail && !isClientDepleted(selectedClientDetail) && selectedClientDetail.enable === false && (
                  <Tag>{t('disabled')}</Tag>
                )}
              </Space>
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item
              {...cdnFieldLayout}
              label={t('pages.subconverter.inbounds')}
              required
            >
              <SelectAllClearButtons
                options={inboundOptions}
                value={selectedInboundIds}
                onChange={(value) => form.setFieldsValue({ inboundIds: value })}
              />
              <Form.Item
                name="inboundIds"
                noStyle
                rules={[{ required: true, message: t('pages.subconverter.inboundsRequired') }]}
              >
                <Select
                  mode="multiple"
                  placeholder={t('pages.subconverter.inboundsPlaceholder')}
                  options={inboundOptions}
                  optionFilterProp="label"
                  maxTagCount="responsive"
                  className="subconverter-inbound-select"
                  placement="topLeft"
                  listHeight={220}
                  showSearch
                  filterOption={(input, option) =>
                    ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase())}
                />
              </Form.Item>
            </Form.Item>
          </Col>
          {trafficStatsEnabled && hasSelectedInbounds && hasNoCommonClient && (
            <Col span={24}>
              <Alert
                type="error"
                showIcon
                message={clientProblem ? (
                  <Space wrap>
                    <span>{t('pages.clients.client')}: {clientProblem.client.email}</span>
                    <Tag color={clientProblem.color}>{clientProblem.label}</Tag>
                  </Space>
                ) : t('pages.subconverter.commonClientRequired')}
              />
            </Col>
          )}
          {cdnTlsInboundIds.length > 0 && (
            <Col span={24}>
              <Space orientation="vertical" size={12} className="subconverter-cdn-overrides">
                {cdnTlsInboundIds.map((id) => {
                  const key = String(id);
                  const required = isCdnTlsRequired(id);
                  const enabled = required || !!cdnTls?.[key]?.enabled;
                  return (
                    <div className="subconverter-cdn-override" key={key}>
                      <Space orientation="vertical" size={8} className="subconverter-cdn-override-inner">
                        <div className="subconverter-cdn-override-head">
                          <Tag color={INBOUND_TAG_COLOR}>{inboundTagLabel(id)}</Tag>
                        </div>
                        <Row gutter={12}>
                          <Col xs={24} sm={12}>
                            <Form.Item
                              {...cdnFieldLayout}
                              name={['cdnTls', key, 'enabled']}
                              label={t('pages.subconverter.cdnTlsOverride')}
                              valuePropName="checked"
                              initialValue={required}
                            >
                              <Switch disabled={required} />
                            </Form.Item>
                          </Col>
                        </Row>
                        {enabled && (
                          <Row gutter={12}>
                            <Col xs={24} sm={12}>
                              <Form.Item
                                {...cdnFieldLayout}
                                name={['cdnTls', key, 'server']}
                                label={t('pages.subconverter.cdnServer')}
                                rules={[{
                                  validator: (_, value) => {
                                    if (!enabled || String(value || '').trim()) return Promise.resolve();
                                    return Promise.reject(new Error(t('pages.subconverter.cdnServerRequired')));
                                  },
                                }]}
                              >
                                <Input placeholder={t('pages.subconverter.cdnServerPlaceholder')} />
                              </Form.Item>
                            </Col>
                            <Col xs={24} sm={12}>
                              <Form.Item
                                {...cdnFieldLayout}
                                name={['cdnTls', key, 'port']}
                                label={t('pages.subconverter.cdnPort')}
                                initialValue={443}
                              >
                                <InputNumber min={1} max={65535} placeholder="443" className="subconverter-full-width" />
                              </Form.Item>
                            </Col>
                            <Col xs={24} sm={12}>
                              <Form.Item {...cdnFieldLayout} name={['cdnTls', key, 'serverName']} label={t('pages.subconverter.cdnServerName')}>
                                <Input placeholder={t('pages.subconverter.cdnServerNamePlaceholder')} />
                              </Form.Item>
                            </Col>
                          </Row>
                        )}
                      </Space>
                    </div>
                  );
                })}
              </Space>
            </Col>
          )}
        </Row>
      </Form>
    </Modal>
  );
}
