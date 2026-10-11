import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Modal, Radio, Space, Spin, Switch, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd/es/form';

import type {
  SubconverterTemplateStatus,
  SubconverterTemplateRecord,
  SubconverterTemplateInput,
  TemplateVariant,
} from '@/schemas/subconverter';
import { useDatepicker } from '@/hooks/useDatepicker';
import { IntlUtil } from '@/utils';

import type { SettingsValues } from './types';
import UaKeywordEditor from './UaKeywordEditor';
import SubconverterTemplateManager, { templateDisplayName } from './SubconverterTemplateManager';
import { DEFAULT_UA_KEYWORDS, normalizeUAKeywords } from './utils';

interface SubconverterSettingsModalProps {
  open: boolean;
  saving: boolean;
  form: FormInstance<SettingsValues>;
  savedTemplateVariant: TemplateVariant;
  templateStatus?: SubconverterTemplateStatus;
  templateStatusLoading: boolean;
  templateStatusError: boolean;
  templateRefreshing: boolean;
  onRefreshTemplate: () => void;
  templates?: SubconverterTemplateRecord[];
  templatesLoading?: boolean;
  templatesError?: boolean;
  catalogPending?: boolean;
  onSaveTemplate?: (id: string | undefined, input: SubconverterTemplateInput) => Promise<unknown>;
  onDeleteTemplate?: (id: string) => Promise<unknown>;
  onRefreshNamedTemplate?: (id: string) => Promise<unknown>;
  onOk: () => void;
  onCancel: () => void;
}

export default function SubconverterSettingsModal({
  open,
  saving,
  form,
  savedTemplateVariant,
  templateStatus,
  templateStatusLoading,
  templateStatusError,
  templateRefreshing,
  onRefreshTemplate,
  templates,
  templatesLoading = false,
  templatesError = false,
  catalogPending = false,
  onSaveTemplate,
  onDeleteTemplate,
  onRefreshNamedTemplate,
  onOk,
  onCancel,
}: SubconverterSettingsModalProps) {
  const { t } = useTranslation();
  const { datepicker } = useDatepicker();
  const selectedVariant = Form.useWatch('templateVariant', form) ?? savedTemplateVariant;
  const activeVariant =
    !templateStatusLoading && !templateStatusError
      ? (templateStatus?.variant ?? savedTemplateVariant)
      : savedTemplateVariant;
  const variantChanged = selectedVariant !== activeVariant;
  const busy = saving || templateRefreshing || catalogPending;
  // Hide cached query data while fetching. A fresh response also reflects
  // switches made by another administrator while this dialog is open.
  const currentStatus = !templateStatusLoading && !templateStatusError ? templateStatus : undefined;
  const nameForID = (id: string) =>
    templateDisplayName(
      {
        id,
        name:
          templates?.find((item) => item.id === id)?.name ??
          currentStatus?.name ??
          t('pages.subconverter.templateLoading'),
      },
      t,
    );

  return (
    <Modal
      open={open}
      title={t('pages.subconverter.settings')}
      okText={t('save')}
      cancelText={t('cancel')}
      confirmLoading={saving}
      okButtonProps={{
        disabled: templateRefreshing || catalogPending || templatesLoading || templatesError,
      }}
      cancelButtonProps={{ disabled: busy }}
      closable={!busy}
      keyboard={!busy}
      mask={{ closable: !busy }}
      width={680}
      styles={{ body: { maxHeight: '70vh', overflowY: 'auto' } }}
      onOk={onOk}
      onCancel={onCancel}
      destroyOnHidden
    >
      <Form<SettingsValues>
        form={form}
        disabled={busy}
        colon={false}
        labelCol={{ sm: { span: 8 } }}
        wrapperCol={{ sm: { span: 14 } }}
        initialValues={{
          uaFilterEnabled: true,
          uaKeywords: [...DEFAULT_UA_KEYWORDS],
          uaRejectStatus: 403,
          templateVariant: savedTemplateVariant,
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
        <Divider plain />
        <Typography.Text strong>{t('pages.subconverter.template')}</Typography.Text>
        <Form.Item
          name="templateVariant"
          label={t('pages.subconverter.templateVariant')}
          extra={t(
            selectedVariant === 'general'
              ? 'pages.subconverter.templateGeneralHint'
              : selectedVariant === 'aggressive'
                ? 'pages.subconverter.templateAggressiveHint'
                : 'pages.subconverter.templateCustomHint',
          )}
          style={{ marginTop: 12 }}
        >
          <Radio.Group
            name="templateVariant"
            disabled={busy || templatesLoading || templatesError}
            style={{ display: 'flex', flexDirection: 'column', gap: 8 }}
          >
            <Radio value="general">{t('pages.subconverter.templateGeneral')}</Radio>
            <Radio value="aggressive">{t('pages.subconverter.templateAggressive')}</Radio>
            {templates
              ?.filter((item) => !item.builtin)
              .map((item) => (
                <Radio key={item.id} value={item.id}>
                  {item.name}
                </Radio>
              ))}
          </Radio.Group>
        </Form.Item>
        <Space orientation="vertical" size={4} style={{ width: '100%' }}>
          <Typography.Text>
            {t('pages.subconverter.templateActive', {
              version: nameForID(activeVariant),
            })}
          </Typography.Text>
          <Typography.Text type="secondary" style={{ wordBreak: 'break-all' }}>
            {currentStatus?.url}
          </Typography.Text>
          <Space size={8} align="start" wrap>
            {templateStatusLoading || (!templateStatusError && !currentStatus) ? (
              <Spin size="small" />
            ) : templateStatusError ? (
              <Typography.Text type="danger">{t('pages.subconverter.loadFailed')}</Typography.Text>
            ) : (
              <Space orientation="vertical" size={4} role="status">
                <Typography.Text>
                  {currentStatus?.checkedAt
                    ? t('pages.subconverter.templateCheckedAt', {
                        time: IntlUtil.formatDate(currentStatus.checkedAt, datepicker),
                      })
                    : t('pages.subconverter.templateCheckUnknown')}
                </Typography.Text>
                {currentStatus?.cached ? (
                  <Typography.Text>
                    {t('pages.subconverter.templateUpdatedAt', {
                      time: IntlUtil.formatDate(currentStatus.updatedAt, datepicker),
                    })}
                  </Typography.Text>
                ) : (
                  <Typography.Text type="warning">
                    {t('pages.subconverter.templateNotCached')}
                  </Typography.Text>
                )}
                {currentStatus?.checkedAt && currentStatus.checkResult ? (
                  <Typography.Text type="success">
                    {t(
                      `pages.subconverter.templateCheck${currentStatus.checkResult === 'updated' ? 'Updated' : 'Unchanged'}`,
                    )}
                  </Typography.Text>
                ) : null}
              </Space>
            )}
            <Button
              size="small"
              icon={<ReloadOutlined />}
              loading={templateRefreshing}
              disabled={busy || variantChanged}
              onClick={onRefreshTemplate}
            >
              {t('pages.subconverter.templateRefresh')}
            </Button>
          </Space>
          {variantChanged ? (
            <Typography.Text type="warning" role="status">
              {t('pages.subconverter.templateSaveFirst')}
            </Typography.Text>
          ) : null}
          <Typography.Text type="secondary">
            {t('pages.subconverter.templateApplyHint')}
          </Typography.Text>
        </Space>
      </Form>
      {templates !== undefined && onSaveTemplate && onDeleteTemplate && onRefreshNamedTemplate ? (
        <>
          <Divider />
          {templatesLoading ? (
            <Spin size="small" />
          ) : templatesError ? (
            <Typography.Text type="danger">
              {t('pages.subconverter.templateListFailed')}
            </Typography.Text>
          ) : null}
          <SubconverterTemplateManager
            templates={templatesLoading || templatesError ? [] : templates}
            activeId={activeVariant}
            disabled={saving || templateRefreshing || templatesLoading || templatesError}
            pending={catalogPending}
            onSave={(id, input) => onSaveTemplate(id, input)}
            onDelete={async (id) => {
              await onDeleteTemplate(id);
              if (form.getFieldValue('templateVariant') === id)
                form.setFieldValue('templateVariant', activeVariant);
            }}
            onRefresh={onRefreshNamedTemplate}
          />
        </>
      ) : null}
    </Modal>
  );
}
