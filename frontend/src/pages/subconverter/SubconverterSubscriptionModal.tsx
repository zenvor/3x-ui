import { useTranslation } from 'react-i18next';
import { ApiOutlined } from '@ant-design/icons';
import { Col, Form, Input, InputNumber, Modal, Row, Select, Space, Switch, Tooltip } from 'antd';
import type { FormInstance } from 'antd/es/form';

import type { FormValues, InboundOption } from './types';

interface SubconverterSubscriptionModalProps {
  open: boolean;
  editingId: number | null;
  saving: boolean;
  form: FormInstance<FormValues>;
  supportedInbounds: InboundOption[];
  inboundSelectLabel: (id: number) => string;
  inboundTagLabel: (id: number) => string;
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
  onOk,
  onCancel,
}: SubconverterSubscriptionModalProps) {
  const { t } = useTranslation();

  return (
    <Modal
      open={open}
      title={editingId == null ? t('pages.subconverter.create') : t('pages.subconverter.edit')}
      okText={t('confirm')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      width={640}
      onOk={onOk}
      onCancel={onCancel}
      destroyOnHidden
    >
      <Form<FormValues>
        form={form}
        colon={false}
        labelCol={{ sm: { span: 8 } }}
        wrapperCol={{ sm: { span: 14 } }}
        initialValues={{ remark: '', limitIp: 1, enable: true, inboundIds: [] }}
      >
        <Row>
          <Col span={24}>
            <Form.Item name="enable" label={t('enable')} valuePropName="checked">
              <Switch />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item name="remark" label={t('remark')}>
              <Input />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item
              name="limitIp"
              label={(
                <Space size={4}>
                  <span>{t('pages.subconverter.maxIps')}</span>
                  <Tooltip title={t('pages.subconverter.maxIpsHint')}>
                    <ApiOutlined className="subconverter-help-icon" />
                  </Tooltip>
                </Space>
              )}
            >
              <InputNumber min={0} />
            </Form.Item>
          </Col>
          <Col span={24}>
            <Form.Item
              name="inboundIds"
              label={t('pages.subconverter.inbounds')}
              rules={[{ required: true, message: t('pages.subconverter.inboundsRequired') }]}
            >
              <Select
                mode="multiple"
                placeholder={t('pages.subconverter.inboundsPlaceholder')}
                options={supportedInbounds.map((inbound) => ({
                  value: inbound.id,
                  label: inboundSelectLabel(inbound.id),
                  title: inboundTagLabel(inbound.id),
                }))}
                optionFilterProp="label"
                showSearch
                filterOption={(input, option) =>
                  ((option?.label as string) || '').toLowerCase().includes(input.toLowerCase())}
              />
            </Form.Item>
          </Col>
        </Row>
      </Form>
    </Modal>
  );
}
