import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../helpers';
import {
  AppAlert,
  AppButton,
  AppDivider,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppSpin,
  AppTextarea,
} from '../router-ui';

const optionKeys = [
  'Notice',
  'About',
  'HomePageContent',
  'Footer',
];

const OtherSetting = ({ section = '' }) => {
  const { t } = useTranslation();
  let [inputs, setInputs] = useState({
    Notice: '',
    About: '',
    HomePageContent: '',
    Footer: '',
  });
  let [loading, setLoading] = useState(false);
  const normalizedSection = (section || '').trim().toLowerCase();
  const showAllSections =
    normalizedSection === '' || normalizedSection === 'all';
  const sectionVisible = {
    notice: showAllSections || normalizedSection === 'notice',
    content: showAllSections || normalizedSection === 'content',
  };

  const getOptions = useCallback(async () => {
    const res = await API.get('/api/v1/admin/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (optionKeys.includes(item.key)) {
          newInputs[item.key] = item.value;
        }
      });
      setInputs(newInputs);
    } else {
      showError(message);
    }
  }, []);

  useEffect(() => {
    getOptions().then();
  }, [getOptions]);

  const updateOption = async (key, value) => {
    setLoading(true);
    const res = await API.put('/api/v1/admin/option/', {
      key,
      value,
    });
    const { success, message } = res.data;
    if (success) {
      setInputs((inputs) => ({ ...inputs, [key]: value }));
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const handleInputChange = async (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const submitAbout = async () => {
    await updateOption('About', inputs.About);
  };

  const submitNotice = async () => {
    await updateOption('Notice', inputs.Notice);
  };

  const submitOption = async (key) => {
    await updateOption(key, inputs[key]);
  };

  return (
    <AppSpin spinning={loading}>
      <div className='router-settings-system-block'>
          {sectionVisible.notice ? (
            <>
              <AppFilterHeader
                title={t('setting.system.notice', '站点公告')}
                titleClassName='router-ui-section-title'
                className='router-toolbar-compact'
              />
              <div className='router-settings-section-body'>
                <AppFormRow>
                  <AppField>
                    <AppTextarea
                      className='router-section-textarea router-code-textarea router-code-textarea-sm'
                      name='Notice'
                      value={inputs.Notice}
                      onChange={handleInputChange}
                      placeholder={t(
                        'setting.system.notice_placeholder',
                        '支持 Markdown',
                      )}
                    />
                  </AppField>
                </AppFormRow>
                <AppFormActions align='start'>
                  <AppButton
                    type='button'
                    className='router-section-button'
                    onClick={submitNotice}
                  >
                    {t('setting.system.buttons.save')}
                  </AppButton>
                </AppFormActions>

                <AppAlert
                  className='router-section-message router-settings-inline-message'
                  type='info'
                  showIcon
                  title={t('setting.other.copyright.notice')}
                />
              </div>
              {showAllSections && sectionVisible.content ? <AppDivider /> : null}
            </>
          ) : null}

          {sectionVisible.content ? (
            <>
              <AppFilterHeader
                title={t('setting.other.content.title')}
                titleClassName='router-ui-section-title'
                className='router-toolbar-compact'
              />
              <div className='router-settings-section-body'>
                <div className='router-settings-page-block'>
                  <AppFormRow>
                    <AppField label={t('setting.other.content.homepage.title')}>
                      <AppTextarea
                        className='router-section-textarea router-code-textarea router-code-textarea-md'
                        placeholder={t('setting.other.content.homepage.placeholder')}
                        value={inputs.HomePageContent}
                        name='HomePageContent'
                        onChange={handleInputChange}
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormActions align='start'>
                    <AppButton
                      type='button'
                      className='router-section-button'
                      onClick={() => submitOption('HomePageContent')}
                    >
                      {t('setting.other.content.buttons.save_homepage')}
                    </AppButton>
                  </AppFormActions>
                </div>
                <div className='router-settings-page-block'>
                  <AppFormRow>
                    <AppField label={t('setting.other.content.about.title')}>
                      <AppTextarea
                        className='router-section-textarea router-code-textarea router-code-textarea-md'
                        placeholder={t('setting.other.content.about.placeholder')}
                        value={inputs.About}
                        name='About'
                        onChange={handleInputChange}
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormActions align='start'>
                    <AppButton
                      type='button'
                      className='router-section-button'
                      onClick={submitAbout}
                    >
                      {t('setting.other.content.buttons.save_about')}
                    </AppButton>
                  </AppFormActions>
                </div>
                <div className='router-settings-page-block'>
                  <AppFormRow>
                    <AppField label={t('setting.other.content.footer.title')}>
                      <AppTextarea
                        className='router-section-textarea router-code-textarea router-code-textarea-sm'
                        placeholder={t('setting.other.content.footer.placeholder')}
                        value={inputs.Footer}
                        name='Footer'
                        onChange={handleInputChange}
                      />
                    </AppField>
                  </AppFormRow>
                  <AppFormActions align='start'>
                    <AppButton
                      type='button'
                      className='router-section-button'
                      onClick={() => submitOption('Footer')}
                    >
                      {t('setting.other.content.buttons.save_footer')}
                    </AppButton>
                  </AppFormActions>
                </div>
              </div>
            </>
          ) : null}
      </div>
    </AppSpin>
  );
};

export default OtherSetting;
