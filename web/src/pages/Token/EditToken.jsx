import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Form,
  Header,
  Label,
  Message,
  Table,
} from 'semantic-ui-react';
import {
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom';
import {
  API,
  copy,
  showError,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../../helpers';
import { renderAmountEquivalentPrompt } from '../../helpers/render';

const EditToken = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const params = useParams();
  const tokenId = params.id;
  const isCreateMode = tokenId === undefined;
  const isDetailMode = !isCreateMode;
  const returnPath = (() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  })();
  const [loading, setLoading] = useState(isDetailMode);
  const [modelOptions, setModelOptions] = useState([]);
  const [allModelsSelected, setAllModelsSelected] = useState(isCreateMode);
  const [modelKeyword, setModelKeyword] = useState('');
  const [detailEditingSection, setDetailEditingSection] = useState('');
  const originInputs = {
    name: '',
    remain_quota: isDetailMode ? 0 : 500000,
    expired_time: '',
    unlimited_quota: false,
    models: [],
    subnet: '',
    status: 1,
    created_time: 0,
    key: '',
    used_quota: 0,
  };
  const [inputs, setInputs] = useState(originInputs);
  const [persistedInputs, setPersistedInputs] = useState(originInputs);
  const {
    name,
    remain_quota: remainingYYC,
    expired_time,
    unlimited_quota: hasUnlimitedYYCLimit,
  } = inputs;
  const navigate = useNavigate();
  const allModelValues = modelOptions.map((option) => option.value);
  const filteredModelOptions = modelOptions.filter((option) =>
    option.value.toLowerCase().includes(modelKeyword.trim().toLowerCase())
  );
  const basicReadonly = isDetailMode && detailEditingSection !== 'basic';
  const modelsReadonly = isDetailMode && detailEditingSection !== 'models';
  const limitsReadonly = isDetailMode && detailEditingSection !== 'limits';
  const isEveryModelSelected = modelOptions.length > 0 && (
    allModelsSelected || inputs.models.length === modelOptions.length
  );
  const selectedModels = isEveryModelSelected ? allModelValues : inputs.models;

  const renderStatus = (status) => {
    switch (status) {
      case 1:
        return (
          <Label basic color='green' className='router-tag'>
            {t('token.table.status_enabled')}
          </Label>
        );
      case 2:
        return (
          <Label basic color='red' className='router-tag'>
            {t('token.table.status_disabled')}
          </Label>
        );
      case 3:
        return (
          <Label basic color='yellow' className='router-tag'>
            {t('token.table.status_expired')}
          </Label>
        );
      case 4:
        return (
          <Label basic color='grey' className='router-tag'>
            {t('token.table.status_depleted')}
          </Label>
        );
      default:
        return (
          <Label basic color='black' className='router-tag'>
            {t('token.table.status_unknown')}
          </Label>
        );
    }
  };

  const renderShortToken = (key) => {
    const raw = typeof key === 'string' ? key.trim() : '';
    if (raw === '') {
      return '-';
    }
    const withPrefix = raw.startsWith('sk-') ? raw : `sk-${raw}`;
    if (withPrefix.length <= 24) {
      return withPrefix;
    }
    return `${withPrefix.slice(0, 12)}...${withPrefix.slice(-8)}`;
  };

  const syncTokenState = useCallback((data) => {
    const normalizedData = {
      ...originInputs,
      ...data,
    };
    normalizedData.remain_quota = Number(
      data?.yyc_remain ?? normalizedData.remain_quota ?? 0
    ) || 0;
    normalizedData.used_quota = Number(
      data?.yyc_used ?? normalizedData.used_quota ?? 0
    ) || 0;
    if (normalizedData.expired_time !== -1) {
      normalizedData.expired_time = timestamp2string(normalizedData.expired_time);
    } else {
      normalizedData.expired_time = '';
    }
    if (
      normalizedData.models === '' ||
      normalizedData.models === null ||
      normalizedData.models === undefined
    ) {
      normalizedData.models = [];
      setAllModelsSelected(true);
    } else {
      normalizedData.models = normalizedData.models.split(',');
      setAllModelsSelected(false);
    }
    setInputs(normalizedData);
    setPersistedInputs(normalizedData);
  }, []);

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const startDetailSectionEdit = useCallback((section) => {
    if (!isDetailMode) {
      return;
    }
    setDetailEditingSection(section);
  }, [isDetailMode]);

  const cancelDetailSectionEdit = useCallback(() => {
    setInputs(persistedInputs);
    setAllModelsSelected(
      Array.isArray(persistedInputs.models)
        ? persistedInputs.models.length === 0
        : true
    );
    setDetailEditingSection('');
  }, [persistedInputs]);

  const handleCopyToken = async () => {
    const raw = typeof inputs.key === 'string' ? inputs.key.trim() : '';
    if (raw === '') {
      return;
    }
    const tokenValue = raw.startsWith('sk-') ? raw : `sk-${raw}`;
    if (await copy(tokenValue)) {
      showSuccess(t('token.messages.copy_success'));
      return;
    }
    showWarning(t('token.messages.copy_failed'));
  };

  const handleCancel = () => {
    if (isCreateMode) {
      navigate('/token');
      return;
    }
    if (detailEditingSection !== '') {
      cancelDetailSectionEdit();
      return;
    }
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/token');
  };

  const handleBack = () => {
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
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

  const toggleUnlimitedYYCLimit = () => {
    setInputs((prev) => ({
      ...prev,
      unlimited_quota: !prev.unlimited_quota,
    }));
  };

  const loadToken = useCallback(async () => {
    try {
      let res = await API.get(`/api/v1/public/token/${tokenId}`);
      const { success, message, data } = res.data || {};
      if (success && data) {
        syncTokenState(data);
      } else {
        showError(message || 'Failed to load token');
      }
    } catch (error) {
      showError(error.message || 'Network error');
    }
    setLoading(false);
  }, [syncTokenState, tokenId]);

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
        if (isCreateMode) {
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
  }, [isCreateMode]);

  useEffect(() => {
    if (isDetailMode) {
      loadToken().catch((error) => {
        showError(error.message || 'Failed to load token');
        setLoading(false);
      });
    }
    loadAvailableModels().catch((error) => {
      showError(error.message || 'Failed to load models');
    });
  }, [isDetailMode, loadAvailableModels, loadToken]);

  const submit = async () => {
    if (isCreateMode && inputs.name === '') return;
    const localInputs = { ...inputs };
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
    if (isDetailMode) {
      res = await API.put(`/api/v1/public/token/`, {
        ...localInputs,
        id: parseInt(tokenId),
      });
    } else {
      res = await API.post(`/api/v1/public/token/`, localInputs);
    }
    const { success, message } = res.data;
    if (success) {
      if (isDetailMode) {
        showSuccess(t('token.edit.messages.update_success'));
        syncTokenState({
          ...inputs,
          id: parseInt(tokenId),
          models: localInputs.models,
          expired_time: localInputs.expired_time,
        });
        setDetailEditingSection('');
      } else {
        showSuccess(t('token.edit.messages.create_success'));
        setInputs(originInputs);
      }
      if (isCreateMode) {
        navigate('/token');
      }
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
          {isCreateMode ? (
            <>
              <Card.Header className='header router-page-title'>
                {t('token.edit.title_create')}
              </Card.Header>
              <div className='router-toolbar router-block-gap-sm'>
                <div className='router-toolbar-start'>
                  <Button
                    className='router-page-button'
                    onClick={handleCancel}
                  >
                    {t('token.edit.buttons.cancel')}
                  </Button>
                </div>
                <div className='router-toolbar-end'>
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
                    required={isCreateMode}
                    readOnly={false}
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
                    readOnly={false}
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
                    readOnly={false}
                  />
                </Form.Field>
                <div className='router-token-expire-actions'>
                  <Button
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(0, 0, 0, 0);
                    }}
                  >
                    {t('token.edit.buttons.never_expire')}
                  </Button>
                  <Button
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(1, 0, 0, 0);
                    }}
                  >
                    {t('token.edit.buttons.expire_1_month')}
                  </Button>
                  <Button
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(0, 1, 0, 0);
                    }}
                  >
                    {t('token.edit.buttons.expire_1_day')}
                  </Button>
                  <Button
                    className='router-inline-button'
                    type='button'
                    onClick={() => {
                      setExpiredTime(0, 0, 1, 0);
                    }}
                  >
                    {t('token.edit.buttons.expire_1_hour')}
                  </Button>
                  <Button
                    className='router-inline-button'
                    type='button'
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
                    label={`${t('token.edit.quota')}${renderAmountEquivalentPrompt(
                      remainingYYC,
                      t
                    )}`}
                    name='remain_quota'
                    placeholder={t('token.edit.quota_placeholder')}
                    onChange={handleInputChange}
                    value={remainingYYC}
                    autoComplete='new-password'
                    type='number'
                    disabled={hasUnlimitedYYCLimit}
                  />
                </Form.Field>
                <Button
                  className='router-inline-button'
                  type='button'
                  onClick={() => {
                    toggleUnlimitedYYCLimit();
                  }}
                >
                  {hasUnlimitedYYCLimit
                    ? t('token.edit.buttons.cancel_unlimited')
                    : t('token.edit.buttons.unlimited_quota')}
                </Button>
              </Form>
            </>
          ) : (
            <div className='router-entity-detail-page'>
              <div className='router-entity-detail-breadcrumb'>
                <Breadcrumb size='small'>
                  <Breadcrumb.Section link onClick={handleBack}>
                    {t('header.token')}
                  </Breadcrumb.Section>
                  <Breadcrumb.Divider icon='right chevron' />
                  <Breadcrumb.Section active>
                    {inputs.name || renderShortToken(inputs.key) || tokenId}
                  </Breadcrumb.Section>
                </Breadcrumb>
              </div>
              <section className='router-entity-detail-section'>
                <div className='router-entity-detail-section-header'>
                  <Header as='h3' className='router-entity-detail-section-title'>
                    {t('common.basic_info')}
                  </Header>
                  <div className='router-toolbar-start'>
                    {renderStatus(Number(inputs.status || 0))}
                    {detailEditingSection === 'basic' ? (
                      <>
                        <Button className='router-page-button' onClick={cancelDetailSectionEdit}>
                          {t('token.edit.buttons.cancel')}
                        </Button>
                        <Button className='router-page-button' positive onClick={submit}>
                          {t('token.edit.buttons.submit')}
                        </Button>
                      </>
                    ) : (
                      <Button
                        className='router-page-button'
                        positive
                        onClick={() => startDetailSectionEdit('basic')}
                        disabled={detailEditingSection !== ''}
                      >
                        {t('token.buttons.edit')}
                      </Button>
                    )}
                  </div>
                </div>
                <Form loading={loading} autoComplete='new-password'>
                  <Form.Field>
                    <Form.Input
                      className='router-section-input'
                      label={t('token.edit.name')}
                      name='name'
                      placeholder={t('token.edit.name_placeholder')}
                      onChange={handleInputChange}
                      value={name}
                      autoComplete='new-password'
                      readOnly={basicReadonly}
                    />
                  </Form.Field>
                  <Form.Group widths='equal'>
                    <Form.Field>
                      <label>{t('token.table.token')}</label>
                      <Form.Input
                        className='router-section-input'
                        value={renderShortToken(inputs.key)}
                        readOnly
                        action={(
                          <Button
                            type='button'
                            icon='copy outline'
                            className='router-page-button'
                            onClick={handleCopyToken}
                            disabled={!inputs.key}
                            aria-label={t('token.buttons.copy')}
                          />
                        )}
                      />
                    </Form.Field>
                    <Form.Input
                      className='router-section-input'
                      label={t('token.table.created_time')}
                      value={
                        inputs.created_time
                          ? timestamp2string(inputs.created_time)
                          : ''
                      }
                      readOnly
                    />
                  </Form.Group>
                </Form>
              </section>
              <section className='router-entity-detail-section'>
                <div className='router-entity-detail-section-header'>
                  <Header as='h3' className='router-entity-detail-section-title'>
                    {t('token.detail.sections.models')}
                  </Header>
                  <div className='router-toolbar-start'>
                    {detailEditingSection === 'models' ? (
                      <>
                        <Button className='router-page-button' onClick={cancelDetailSectionEdit}>
                          {t('token.edit.buttons.cancel')}
                        </Button>
                        <Button className='router-page-button' positive onClick={submit}>
                          {t('token.edit.buttons.submit')}
                        </Button>
                      </>
                    ) : (
                      <Button
                        className='router-page-button'
                        positive
                        onClick={() => startDetailSectionEdit('models')}
                        disabled={detailEditingSection !== ''}
                      >
                        {t('token.buttons.edit')}
                      </Button>
                    )}
                  </div>
                </div>
                <Form loading={loading}>
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
                              disabled={modelsReadonly}
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
                                  disabled={modelsReadonly}
                                />
                              </Table.Cell>
                              <Table.Cell>{option.text}</Table.Cell>
                            </Table.Row>
                          ))
                        )}
                      </Table.Body>
                    </Table>
                  </div>
                </Form>
              </section>
              <section className='router-entity-detail-section'>
                <div className='router-entity-detail-section-header'>
                  <Header as='h3' className='router-entity-detail-section-title'>
                    {t('token.detail.sections.limits')}
                  </Header>
                  <div className='router-toolbar-start'>
                    {detailEditingSection === 'limits' ? (
                      <>
                        <Button className='router-page-button' onClick={cancelDetailSectionEdit}>
                          {t('token.edit.buttons.cancel')}
                        </Button>
                        <Button className='router-page-button' positive onClick={submit}>
                          {t('token.edit.buttons.submit')}
                        </Button>
                      </>
                    ) : (
                      <Button
                        className='router-page-button'
                        positive
                        onClick={() => startDetailSectionEdit('limits')}
                        disabled={detailEditingSection !== ''}
                      >
                        {t('token.buttons.edit')}
                      </Button>
                    )}
                  </div>
                </div>
                <Form loading={loading} autoComplete='new-password'>
                  <Form.Field>
                    <Form.Input
                      className='router-section-input'
                      label={t('token.edit.ip_limit')}
                      name='subnet'
                      placeholder={t('token.edit.ip_limit_placeholder')}
                      onChange={handleInputChange}
                      value={inputs.subnet}
                      autoComplete='new-password'
                      readOnly={limitsReadonly}
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
                      readOnly={limitsReadonly}
                    />
                  </Form.Field>
                  {detailEditingSection === 'limits' ? (
                    <div className='router-token-expire-actions'>
                      <Button
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 0, 0, 0);
                        }}
                      >
                        {t('token.edit.buttons.never_expire')}
                      </Button>
                      <Button
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(1, 0, 0, 0);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_month')}
                      </Button>
                      <Button
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 1, 0, 0);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_day')}
                      </Button>
                      <Button
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 0, 1, 0);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_hour')}
                      </Button>
                      <Button
                        className='router-inline-button'
                        type='button'
                        onClick={() => {
                          setExpiredTime(0, 0, 0, 1);
                        }}
                      >
                        {t('token.edit.buttons.expire_1_minute')}
                      </Button>
                    </div>
                  ) : null}
                  <Message className='router-section-message'>{t('token.edit.quota_notice')}</Message>
                  <Form.Field>
                    <Form.Input
                      className='router-section-input'
                      label={`${t('token.edit.quota')}${renderAmountEquivalentPrompt(
                        remainingYYC,
                        t
                      )}`}
                      name='remain_quota'
                      placeholder={t('token.edit.quota_placeholder')}
                      onChange={handleInputChange}
                      value={remainingYYC}
                      autoComplete='new-password'
                      type='number'
                      disabled={hasUnlimitedYYCLimit || limitsReadonly}
                    />
                  </Form.Field>
                  {detailEditingSection === 'limits' ? (
                    <Button
                      className='router-inline-button'
                      type='button'
                      onClick={() => {
                        toggleUnlimitedYYCLimit();
                      }}
                    >
                      {hasUnlimitedYYCLimit
                        ? t('token.edit.buttons.cancel_unlimited')
                        : t('token.edit.buttons.unlimited_quota')}
                    </Button>
                  ) : null}
                </Form>
              </section>
            </div>
          )}
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditToken;
