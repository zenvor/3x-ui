import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Col, Form, Input, InputNumber, Modal, Row, Select, Space, Switch, Tag } from 'antd';
import type { FormInstance } from 'antd/es/form';

import { SelectAllClearButtons } from '@/components/form';
import type { FormValues, InboundOption } from './types';
import { INBOUND_TAG_COLOR } from './utils';

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
  const selectedInboundIds = useMemo(() => watchedInboundIds || [], [watchedInboundIds]);
  const cdnTls = useMemo(() => watchedCdnTls || {}, [watchedCdnTls]);
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

  return (
    <Modal
      open={open}
      title={editingId == null ? t('pages.subconverter.create') : t('pages.subconverter.edit')}
      okText={t('confirm')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      width={760}
      onOk={onOk}
      onCancel={onCancel}
      destroyOnHidden
    >
      <Form<FormValues>
        form={form}
        colon={false}
        labelCol={{ sm: { span: 8 } }}
        wrapperCol={{ sm: { span: 14 } }}
        initialValues={{ remark: '', limitIp: 0, enable: true, inboundIds: [], cdnTls: {} }}
      >
        <Row gutter={12}>
          <Col xs={24} md={12}>
            <Form.Item {...cdnFieldLayout} name="remark" label={t('remark')}>
              <Input placeholder={t('remark')} />
            </Form.Item>
          </Col>
          <Col xs={24} md={6}>
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
                  placement="topLeft"
                  listHeight={220}
                  showSearch
                  filterOption={(input, option) =>
                    ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase())}
                />
              </Form.Item>
            </Form.Item>
          </Col>
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
