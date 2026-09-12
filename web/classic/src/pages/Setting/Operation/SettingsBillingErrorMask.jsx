/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState, useRef } from 'react';
import { Button, Col, Form, Row, Spin, TagInput, InputNumber, TextArea } from '@douyinfe/semi-ui';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const defaultConfig = {
  enabled: false,
  keywords: [],
  status_code: 200,
  message: '',
};

export default function SettingsBillingErrorMask(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(defaultConfig);
  const refForm = useRef();

  useEffect(() => {
    if (props.options && props.options.BillingErrorMask) {
      try {
        const parsed = JSON.parse(props.options.BillingErrorMask);
        setInputs({
          enabled: Boolean(parsed.enabled),
          keywords: Array.isArray(parsed.keywords) ? parsed.keywords : [],
          status_code: Number(parsed.status_code) || 200,
          message: String(parsed.message || ''),
        });
      } catch (e) {
        setInputs(defaultConfig);
      }
    } else {
      setInputs(defaultConfig);
    }
  }, [props.options]);

  function onSubmit() {
    setLoading(true);
    const value = JSON.stringify({
      enabled: inputs.enabled,
      keywords: inputs.keywords,
      status_code: inputs.status_code,
      message: inputs.message,
    });
    API.put('/api/option/', {
      key: 'BillingErrorMask',
      value,
    })
      .then((res) => {
        if (res.data.success) {
          showSuccess(t('保存成功'));
          props.refresh();
        } else {
          showError(res.data.message);
        }
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('报错掩盖设置')}>
            <Row gutter={16} style={{ marginBottom: 12 }}>
              <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                <Form.Switch
                  field='enabled'
                  label={t('启用报错掩盖功能')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) => {
                    setInputs((prev) => ({ ...prev, enabled: value }));
                  }}
                  extraText={t('启用后，AI 返回的错误信息中将隐藏匹配的敏感关键词')}
                />
              </Col>
            </Row>

            <Row gutter={16} style={{ marginBottom: 12 }}>
              <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                <div style={{ marginBottom: 8 }}>
                  <span style={{ fontWeight: 600, fontSize: 14 }}>
                    {t('敏感关键词列表')}
                  </span>
                </div>
                <TagInput
                  value={inputs.keywords}
                  onChange={(value) => {
                    setInputs((prev) => ({ ...prev, keywords: value }));
                  }}
                  placeholder={t('输入关键词后回车添加')}
                  style={{ width: '100%' }}
                />
                <div
                  style={{
                    fontSize: 12,
                    color: 'var(--semi-color-text-2)',
                    marginTop: 4,
                  }}
                >
                  {t('报错消息中包含这些关键词时将触发掩盖规则')}
                </div>
              </Col>
            </Row>

            <Row gutter={16} style={{ marginBottom: 12 }}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  field='status_code'
                  label={t('掩盖后状态码')}
                  value={inputs.status_code}
                  onChange={(value) => {
                    setInputs((prev) => ({
                      ...prev,
                      status_code: Number(value) || 200,
                    }));
                  }}
                  min={100}
                  max={599}
                  precision={0}
                  style={{ width: '100%' }}
                />
              </Col>
            </Row>

            <Row gutter={16}>
              <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                <Form.TextArea
                  field='message'
                  label={t('掩盖后提示消息')}
                  value={inputs.message}
                  onChange={(value) => {
                    setInputs((prev) => ({ ...prev, message: value }));
                  }}
                  placeholder={t('自定义错误提示消息，留空则使用默认提示')}
                  style={{ fontFamily: 'JetBrains Mono, Consolas' }}
                  autosize={{ minRows: 3, maxRows: 6 }}
                />
              </Col>
            </Row>

            <Row>
              <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                <Button size='default' type='primary' onClick={onSubmit}>
                  {t('保存报错掩盖设置')}
                </Button>
              </Col>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
