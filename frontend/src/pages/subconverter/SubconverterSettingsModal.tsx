import { useTranslation } from 'react-i18next';
import { Form, Modal, Radio, Switch } from 'antd';
import type { FormInstance } from 'antd/es/form';

import type { SettingsValues } from './types';
import UaKeywordEditor from './UaKeywordEditor';
import { DEFAULT_UA_KEYWORDS, normalizeUAKeywords } from './utils';

interface SubconverterSettingsModalProps {
  open: boolean;
  saving: boolean;
  form: FormInstance<SettingsValues>;
  onOk: () => void;
  onCancel: () => void;
}

export default function SubconverterSettingsModal({
  open,
  saving,
  form,
  onOk,
  onCancel,
}: SubconverterSettingsModalProps) {
  const { t } = useTranslation();

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
        initialValues={{ uaFilterEnabled: true, uaKeywords: [...DEFAULT_UA_KEYWORDS], uaRejectStatus: 403 }}
      >
        <Form.Item name="uaFilterEnabled" label={t('pages.subconverter.uaFilter')} valuePropName="checked">
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
    </Modal>
  );
}
