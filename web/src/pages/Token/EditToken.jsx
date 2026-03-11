import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Checkbox,
  Form,
  Message,
  Table,
} from 'semantic-ui-react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { renderQuotaWithPrompt } from '../../helpers/render';

const EditToken = () => {
  const { t } = useTranslation();
  const params = useParams();
  const tokenId = params.id;
  const isEdit = tokenId !== undefined;
  const [loading, setLoading] = useState(isEdit);
  const [modelOptions, setModelOptions] = useState([]);
  const [allModelsSelected, setAllModelsSelected] = useState(!isEdit);
  const [modelKeyword, setModelKeyword] = useState('');
  const originInputs = {
    name: '',
    remain_quota: isEdit ? 0 : 500000,
    expired_time: '',
    unlimited_quota: false,
    models: [],
    subnet: '',
  };
  const [inputs, setInputs] = useState(originInputs);
  const { name, remain_quota, expired_time, unlimited_quota } = inputs;
  const navigate = useNavigate();
  const allModelValues = modelOptions.map((option) => option.value);
  const filteredModelOptions = modelOptions.filter((option) =>
    option.value.toLowerCase().includes(modelKeyword.trim().toLowerCase())
  );
  const isEveryModelSelected = modelOptions.length > 0 && (
    allModelsSelected || inputs.models.length === modelOptions.length
  );
  const selectedModels = isEveryModelSelected ? allModelValues : inputs.models;

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };
  const handleCancel = () => {
    navigate('/token');
  };
  const setExpiredTime = (month, day, hour, minute) => {
    let now = new Date();
    let timestamp = now.getTime() / 1000;
    let seconds = month * 30 * 24 * 60 * 60;
    seconds += day * 24 * 60 * 60;
    seconds += hour * 60 * 60;
    seconds += minute * 60;
    if (seconds !== 0) {
      timestamp += seconds;
      setInputs({ ...inputs, expired_time: timestamp2string(timestamp) });
    } else {
      setInputs({ ...inputs, expired_time: '' });
    }
  };

  const setUnlimitedQuota = () => {
    setInputs({ ...inputs, unlimited_quota: !unlimited_quota });
  };

  const loadToken = useCallback(async () => {
    try {
      let res = await API.get(`/api/v1/public/token/${tokenId}`);
      const { success, message, data } = res.data || {};
      if (success && data) {
        if (data.expired_time !== -1) {
          data.expired_time = timestamp2string(data.expired_time);
        } else {
          data.expired_time = '';
        }
        if (
          data.models === '' ||
          data.models === null ||
          data.models === undefined
        ) {
          data.models = [];
          setAllModelsSelected(true);
        } else {
          data.models = data.models.split(',');
          setAllModelsSelected(false);
        }
        setInputs(data);
      } else {
        showError(message || 'Failed to load token');
      }
    } catch (error) {
      showError(error.message || 'Network error');
    }
    setLoading(false);
  }, [tokenId]);

  const loadAvailableModels = useCallback(async () => {
    try {
      let res = await API.get(`/api/v1/public/user/available_models`);
      const { success, message, data } = res.data || {};
      if (success && data) {
        let options = data.map((model) => {
          return {
            key: model,
            text: model,
            value: model,
          };
        });
        setModelOptions(options);
        if (!isEdit) {
          setAllModelsSelected(true);
          setInputs((prev) => ({
            ...prev,
            models: options.map((option) => option.value),
          }));
        }
      } else {
        showError(message || 'Failed to load models');
      }
    } catch (error) {
      showError(error.message || 'Network error');
    }
  }, [isEdit]);

  useEffect(() => {
    if (isEdit) {
      loadToken().catch((error) => {
        showError(error.message || 'Failed to load token');
        setLoading(false);
      });
    }
    loadAvailableModels().catch((error) => {
      showError(error.message || 'Failed to load models');
    });
  }, [isEdit, loadAvailableModels, loadToken]);

  const submit = async () => {
    if (!isEdit && inputs.name === '') return;
    let localInputs = inputs;
    localInputs.remain_quota = parseInt(localInputs.remain_quota);
    if (localInputs.expired_time) {
      let time = Date.parse(localInputs.expired_time);
      if (isNaN(time)) {
        showError(t('token.edit.messages.expire_time_invalid'));
        return;
      }
      localInputs.expired_time = Math.ceil(time / 1000);
    } else {
      localInputs.expired_time = -1;
    }
    if (!isEveryModelSelected && localInputs.models.length === 0 && modelOptions.length > 0) {
      showError(t('token.edit.messages.models_required'));
      return;
    }
    localInputs.models = isEveryModelSelected ? '' : localInputs.models.join(',');
    let res;
    if (isEdit) {
      res = await API.put(`/api/v1/public/token/`, {
        ...localInputs,
        id: parseInt(tokenId),
      });
    } else {
      res = await API.post(`/api/v1/public/token/`, localInputs);
    }
    const { success, message } = res.data;
    if (success) {
      if (isEdit) {
        showSuccess(t('token.edit.messages.update_success'));
      } else {
        showSuccess(t('token.edit.messages.create_success'));
        setInputs(originInputs);
      }
      navigate('/token');
    } else {
      showError(message);
    }
  };

  const toggleAllModels = (_, { checked }) => {
    const normalizedChecked = !!checked;
    setAllModelsSelected(normalizedChecked);
    setInputs((prev) => ({
      ...prev,
      models: normalizedChecked ? allModelValues : [],
    }));
  };

  const toggleModel = (modelName, checked) => {
    const selected = new Set(inputs.models);
    if (checked) {
      selected.add(modelName);
    } else {
      selected.delete(modelName);
    }
    const nextModels = allModelValues.filter((value) => selected.has(value));
    setAllModelsSelected(nextModels.length === allModelValues.length && allModelValues.length > 0);
    setInputs((prev) => ({
      ...prev,
      models: nextModels,
    }));
  };

  const handleModelKeywordChange = (_, { value }) => {
    setModelKeyword(value || '');
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <div className='router-toolbar'>
            <div className='router-toolbar-end'>
              <Button className='router-page-button' onClick={handleCancel}>
                {t('token.edit.buttons.cancel')}
              </Button>
              <Button className='router-page-button' positive onClick={submit}>
                {t('token.edit.buttons.submit')}
              </Button>
            </div>
          </div>
          <Form loading={loading} autoComplete='new-password' className='router-block-top-sm'>
            <Form.Field>
              <Form.Input
                className='router-section-input'
                label={t('token.edit.name')}
                name='name'
                placeholder={t('token.edit.name_placeholder')}
                onChange={handleInputChange}
                value={name}
                autoComplete='new-password'
                required={!isEdit}
              />
            </Form.Field>
            <Form.Field>
              <label>{t('token.edit.models')}</label>
              <Message className='router-section-message'>
                {t('token.edit.models_table_notice')}
              </Message>
              <Form.Input
                className='router-section-input router-token-model-search'
                placeholder={t('token.edit.models_search_placeholder')}
                value={modelKeyword}
                onChange={handleModelKeywordChange}
              />
              <div className='router-token-model-table-wrap'>
                <Table basic='very' compact className='router-list-table router-token-model-table'>
                  <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell collapsing>
                        <Checkbox
                          checked={isEveryModelSelected}
                          label={t('token.edit.models_select_all')}
                          onChange={toggleAllModels}
                        />
                      </Table.HeaderCell>
                      <Table.HeaderCell>{t('token.edit.models_table_name')}</Table.HeaderCell>
                    </Table.Row>
                  </Table.Header>
                  <Table.Body>
                    {modelOptions.length === 0 ? (
                      <Table.Row>
                        <Table.Cell colSpan='2' className='router-empty-cell'>
                          {t('token.edit.models_table_empty')}
                        </Table.Cell>
                      </Table.Row>
                    ) : filteredModelOptions.length === 0 ? (
                      <Table.Row>
                        <Table.Cell colSpan='2' className='router-empty-cell'>
                          {t('token.edit.models_search_empty')}
                        </Table.Cell>
                      </Table.Row>
                    ) : (
                      filteredModelOptions.map((option) => (
                        <Table.Row key={option.value}>
                          <Table.Cell collapsing>
                            <Checkbox
                              checked={selectedModels.includes(option.value)}
                              onChange={(_, data) => toggleModel(option.value, !!data.checked)}
                            />
                          </Table.Cell>
                          <Table.Cell>{option.text}</Table.Cell>
                        </Table.Row>
                      ))
                    )}
                  </Table.Body>
                </Table>
              </div>
            </Form.Field>
            <Form.Field>
              <Form.Input
                className='router-section-input'
                label={t('token.edit.ip_limit')}
                name='subnet'
                placeholder={t('token.edit.ip_limit_placeholder')}
                onChange={handleInputChange}
                value={inputs.subnet}
                autoComplete='new-password'
              />
            </Form.Field>
            <Form.Field>
              <Form.Input
                className='router-section-input'
                label={t('token.edit.expire_time')}
                name='expired_time'
                placeholder={t('token.edit.expire_time_placeholder')}
                onChange={handleInputChange}
                value={expired_time}
                autoComplete='new-password'
                type='datetime-local'
              />
            </Form.Field>
            <div className='router-token-expire-actions'>
              <Button
                className='router-inline-button'
                type={'button'}
                onClick={() => {
                  setExpiredTime(0, 0, 0, 0);
                }}
              >
                {t('token.edit.buttons.never_expire')}
              </Button>
              <Button
                className='router-inline-button'
                type={'button'}
                onClick={() => {
                  setExpiredTime(1, 0, 0, 0);
                }}
              >
                {t('token.edit.buttons.expire_1_month')}
              </Button>
              <Button
                className='router-inline-button'
                type={'button'}
                onClick={() => {
                  setExpiredTime(0, 1, 0, 0);
                }}
              >
                {t('token.edit.buttons.expire_1_day')}
              </Button>
              <Button
                className='router-inline-button'
                type={'button'}
                onClick={() => {
                  setExpiredTime(0, 0, 1, 0);
                }}
              >
                {t('token.edit.buttons.expire_1_hour')}
              </Button>
              <Button
                className='router-inline-button'
                type={'button'}
                onClick={() => {
                  setExpiredTime(0, 0, 0, 1);
                }}
              >
                {t('token.edit.buttons.expire_1_minute')}
              </Button>
            </div>
            <Message className='router-section-message'>{t('token.edit.quota_notice')}</Message>
            <Form.Field>
              <Form.Input
                className='router-section-input'
                label={`${t('token.edit.quota')}${renderQuotaWithPrompt(
                  remain_quota,
                  t
                )}`}
                name='remain_quota'
                placeholder={t('token.edit.quota_placeholder')}
                onChange={handleInputChange}
                value={remain_quota}
                autoComplete='new-password'
                type='number'
                disabled={unlimited_quota}
              />
            </Form.Field>
            <Button
              className='router-inline-button'
              type={'button'}
              onClick={() => {
                setUnlimitedQuota();
              }}
            >
              {unlimited_quota
                ? t('token.edit.buttons.cancel_unlimited')
                : t('token.edit.buttons.unlimited_quota')}
            </Button>
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditToken;
