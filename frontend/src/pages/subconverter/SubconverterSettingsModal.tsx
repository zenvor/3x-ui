import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Modal, Radio, Space, Spin, Switch, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd/es/form';

import type { SubconverterTemplateStatus } from '@/schemas/subconverter';
import { useDatepicker } from '@/hooks/useDatepicker';
import { IntlUtil } from '@/utils';

import type { SettingsValues } from './types';
import UaKeywordEditor from './UaKeywordEditor';
import { DEFAULT_UA_KEYWORDS, normalizeUAKeywords } from './utils';

interface SubconverterSettingsModalProps {
  open: boolean;
  saving: boolean;
  form: FormInstance<SettingsValues>;
  templateStatus?: SubconverterTemplateStatus;
  templateStatusLoading: boolean;
  templateStatusError: boolean;
  templateRefreshing: boolean;
  onRefreshTemplate: () => void;
  onOk: () => void;
  onCancel: () => void;
}

export default function SubconverterSettingsModal({
  open,
  saving,
  form,
  templateStatus,
  templateStatusLoading,
  templateStatusError,
  templateRefreshing,
  onRefreshTemplate,
  onOk,
  onCancel,
}: SubconverterSettingsModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();

  return (
    <Modal
      open={open}
      title={t('pages.subconverter.settings')}
      okText={t('save')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      width={560}
      onOk={onOk}
      onCancel={onCancel}
      destroyOnHidden
    >
      <Form<SettingsValues>
        form={form}
        colon={false}
        labelCol={{ sm: { span: 8 } }}
        wrapperCol={{ sm: { span: 14 } }}
        initialValues={{
          uaFilterEnabled: true,
          uaKeywords: [...DEFAULT_UA_KEYWORDS],
          uaRejectStatus: 403,
        }}
      >
        <Form.Item
          name="uaFilterEnabled"
          label={t('pages.subconverter.uaFilter')}
          valuePropName="checked"
        >
          <Switch />
        </Form.Item>
        <Form.Item
          name="uaKeywords"
          label={t('pages.subconverter.uaKeywords')}
          dependencies={['uaFilterEnabled']}
          rules={[
            ({ getFieldValue }) => ({
              validator(_, value: string[] | undefined) {
                if (!getFieldValue('uaFilterEnabled') || normalizeUAKeywords(value).length > 0) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error(t('pages.subconverter.uaKeywordsRequired')));
              },
            }),
          ]}
        >
          <UaKeywordEditor addLabel={t('add')} />
        </Form.Item>
        <Form.Item name="uaRejectStatus" label={t('pages.subconverter.uaRejectStatus')}>
          <Radio.Group optionType="button" buttonStyle="solid">
            <Radio.Button value={403}>403</Radio.Button>
            <Radio.Button value={404}>404</Radio.Button>
          </Radio.Group>
        </Form.Item>
      </Form>

      <Divider plain />
      <Space orientation="vertical" size={4} style={{ width: '100%' }}>
        <Typography.Text strong>{t('pages.subconverter.template')}</Typography.Text>
        <Typography.Text type="secondary" style={{ wordBreak: 'break-all' }}>
          {templateStatus?.url}
        </Typography.Text>
        <Space size={8}>
          {templateStatusLoading ? (
            <Spin size="small" />
          ) : templateStatusError ? (
            <Typography.Text type="danger">{t('pages.subconverter.loadFailed')}</Typography.Text>
          ) : templateStatus?.cached ? (
            <Typography.Text>
              {t('pages.subconverter.templateUpdatedAt', {
                time: IntlUtil.formatDate(templateStatus.updatedAt, datepicker),
              })}
            </Typography.Text>
          ) : (
            <Typography.Text type="warning">
              {t('pages.subconverter.templateNotCached')}
            </Typography.Text>
          )}
          <Button
            size="small"
            icon={<ReloadOutlined />}
            loading={templateRefreshing}
            onClick={onRefreshTemplate}
          >
            {t('pages.subconverter.templateRefresh')}
          </Button>
        </Space>
      </Space>
    </Modal>
  );
}
