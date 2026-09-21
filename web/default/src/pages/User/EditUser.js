import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Card } from 'semantic-ui-react';
import { useParams, useNavigate } from 'react-router-dom';
import { API, showError, showSuccess } from '../../helpers';
import { renderQuota, renderQuotaWithPrompt } from '../../helpers/render';

const EditUser = () => {
  const { t } = useTranslation();
  const params = useParams();
  const userId = params.id;
  const [loading, setLoading] = useState(true);
  const [inputs, setInputs] = useState({
    username: '',
    display_name: '',
    password: '',
    github_id: '',
    wechat_id: '',
    email: '',
    quota: 0,
    group: 'default',
    daily_token_limit: 0,
    daily_quota_limit: 0,
  });
  const [dailyTokenMode, setDailyTokenMode] = useState('follow');
  const [dailyQuotaMode, setDailyQuotaMode] = useState('follow');
  const [groupOptions, setGroupOptions] = useState([]);
  const {
    username,
    display_name,
    password,
    github_id,
    wechat_id,
    email,
    quota,
    group,
  } = inputs;
  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };
  const fetchGroups = async () => {
    try {
      let res = await API.get(`/api/group/`);
      setGroupOptions(
        res.data.data.map((group) => ({
          key: group,
          text: group,
          value: group,
        }))
      );
    } catch (error) {
      showError(error.message);
    }
  };
  const navigate = useNavigate();
  const handleCancel = () => {
    navigate('/setting');
  };
  const loadUser = async () => {
    let res = undefined;
    if (userId) {
      res = await API.get(`/api/user/${userId}`);
    } else {
      res = await API.get(`/api/user/self`);
    }
    const { success, message, data } = res.data;
    if (success) {
      data.password = '';
      const dailyTokenMode =
        data.daily_token_limit === null || data.daily_token_limit === 0
          ? 'follow'
          : data.daily_token_limit < 0
          ? 'exempt'
          : 'custom';
      const dailyQuotaMode =
        data.daily_quota_limit === null || data.daily_quota_limit === 0
          ? 'follow'
          : data.daily_quota_limit < 0
          ? 'exempt'
          : 'custom';
      setInputs({
        ...data,
        daily_token_limit: Math.max(data.daily_token_limit || 0, 0),
        daily_quota_limit: Math.max(data.daily_quota_limit || 0, 0),
      });
      setDailyTokenMode(dailyTokenMode);
      setDailyQuotaMode(dailyQuotaMode);
    } else {
      showError(message);
    }
    setLoading(false);
  };
  useEffect(() => {
    loadUser().then();
    if (userId) {
      fetchGroups().then();
    }
  }, []);

  const submit = async () => {
    let res = undefined;
    if (userId) {
      let data = { ...inputs, id: parseInt(userId) };
      if (typeof data.quota === 'string') {
        data.quota = parseInt(data.quota);
      }
      // 三态编码：follow 必须显式发 0，发 null 会被后端 GORM 的零值跳过而改不回去
      data.daily_token_limit =
        dailyTokenMode === 'follow'
          ? 0
          : dailyTokenMode === 'exempt'
          ? -1
          : Math.max(parseInt(inputs.daily_token_limit) || 0, 1);
      data.daily_quota_limit =
        dailyQuotaMode === 'follow'
          ? 0
          : dailyQuotaMode === 'exempt'
          ? -1
          : Math.max(parseInt(inputs.daily_quota_limit) || 0, 1);
      res = await API.put(`/api/user/`, data);
    } else {
      res = await API.put(`/api/user/self`, inputs);
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('user.messages.update_success'));
    } else {
      showError(message);
    }
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header'>{t('user.edit.title')}</Card.Header>
          <Form loading={loading} autoComplete='new-password'>
            <Form.Field>
              <Form.Input
                label={t('user.edit.username')}
                name='username'
                placeholder={t('user.edit.username_placeholder')}
                onChange={handleInputChange}
                value={username}
                autoComplete='new-password'
              />
            </Form.Field>
            <Form.Field>
              <Form.Input
                label={t('user.edit.password')}
                name='password'
                type={'password'}
                placeholder={t('user.edit.password_placeholder')}
                onChange={handleInputChange}
                value={password}
                autoComplete='new-password'
              />
            </Form.Field>
            <Form.Field>
              <Form.Input
                label={t('user.edit.display_name')}
                name='display_name'
                placeholder={t('user.edit.display_name_placeholder')}
                onChange={handleInputChange}
                value={display_name}
                autoComplete='new-password'
              />
            </Form.Field>
            {userId && (
              <>
                <Form.Field>
                  <Form.Dropdown
                    label={t('user.edit.group')}
                    placeholder={t('user.edit.group_placeholder')}
                    name='group'
                    fluid
                    search
                    selection
                    allowAdditions
                    additionLabel={t('user.edit.group_addition')}
                    onChange={handleInputChange}
                    value={inputs.group}
                    autoComplete='new-password'
                    options={groupOptions}
                  />
                </Form.Field>
                <Form.Field>
                  <Form.Input
                    label={`${t('user.edit.quota')}${renderQuotaWithPrompt(
                      quota,
                      t
                    )}`}
                    name='quota'
                    placeholder={t('user.edit.quota_placeholder')}
                    onChange={handleInputChange}
                    value={quota}
                    type={'number'}
                    autoComplete='new-password'
                  />
                </Form.Field>
                <Form.Group widths='equal'>
                  <Form.Select
                    label={t('user.edit.daily_token_limit')}
                    options={[
                      {
                        key: 'follow',
                        text: t('user.edit.limit_mode_follow_global'),
                        value: 'follow',
                      },
                      {
                        key: 'exempt',
                        text: t('user.edit.limit_mode_exempt'),
                        value: 'exempt',
                      },
                      {
                        key: 'custom',
                        text: t('user.edit.limit_mode_custom'),
                        value: 'custom',
                      },
                    ]}
                    value={dailyTokenMode}
                    onChange={(e, { value }) => setDailyTokenMode(value)}
                  />
                  {dailyTokenMode === 'custom' && (
                    <Form.Input
                      label={t('user.edit.daily_token_limit_value')}
                      name='daily_token_limit'
                      type='number'
                      min='1'
                      value={inputs.daily_token_limit}
                      onChange={handleInputChange}
                    />
                  )}
                </Form.Group>
                <Form.Group widths='equal'>
                  <Form.Select
                    label={t('user.edit.daily_quota_limit')}
                    options={[
                      {
                        key: 'follow',
                        text: t('user.edit.limit_mode_follow_global'),
                        value: 'follow',
                      },
                      {
                        key: 'exempt',
                        text: t('user.edit.limit_mode_exempt'),
                        value: 'exempt',
                      },
                      {
                        key: 'custom',
                        text: t('user.edit.limit_mode_custom'),
                        value: 'custom',
                      },
                    ]}
                    value={dailyQuotaMode}
                    onChange={(e, { value }) => setDailyQuotaMode(value)}
                  />
                  {dailyQuotaMode === 'custom' && (
                    <Form.Input
                      label={`${t('user.edit.daily_quota_limit_value')}${renderQuotaWithPrompt(
                        inputs.daily_quota_limit,
                        t
                      )}`}
                      name='daily_quota_limit'
                      type='number'
                      min='1'
                      value={inputs.daily_quota_limit}
                      onChange={handleInputChange}
                    />
                  )}
                </Form.Group>
              </>
            )}
            <Form.Field>
              <Form.Input
                label={t('user.edit.github_id')}
                name='github_id'
                value={github_id}
                autoComplete='new-password'
                placeholder={t('user.edit.github_id_placeholder')}
                readOnly
              />
            </Form.Field>
            <Form.Field>
              <Form.Input
                label={t('user.edit.wechat_id')}
                name='wechat_id'
                value={wechat_id}
                autoComplete='new-password'
                placeholder={t('user.edit.wechat_id_placeholder')}
                readOnly
              />
            </Form.Field>
            <Form.Field>
              <Form.Input
                label={t('user.edit.email')}
                name='email'
                value={email}
                autoComplete='new-password'
                placeholder={t('user.edit.email_placeholder')}
                readOnly
              />
            </Form.Field>
            <Button onClick={handleCancel}>
              {t('user.edit.buttons.cancel')}
            </Button>
            <Button positive onClick={submit}>
              {t('user.edit.buttons.submit')}
            </Button>
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default EditUser;
