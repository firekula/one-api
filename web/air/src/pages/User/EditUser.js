import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { API, isMobile, showError, showSuccess } from '../../helpers';
import { renderQuotaWithPrompt } from '../../helpers/render';
import Title from '@douyinfe/semi-ui/lib/es/typography/title';
import { Button, Divider, Input, Select, SideSheet, Space, Spin, Typography } from '@douyinfe/semi-ui';

// 单日上限三态：follow 提交 0（跟随全局默认）、exempt 提交 -1（豁免）、custom 提交不小于 1 的正整数
const dailyLimitModeOptions = [
  { label: '跟随全局默认', value: 'follow' },
  { label: '豁免（不限制）', value: 'exempt' },
  { label: '自定义', value: 'custom' }
];

// 由后端原始值推导三态，必须在任何钳制之前调用
const resolveLimitMode = (value) => {
  if (value === null || value === undefined || value === 0) {
    return 'follow';
  }
  return value < 0 ? 'exempt' : 'custom';
};

// 把三态编码成后端约定：follow = 0、exempt = -1、custom >= 1
const encodeLimitMode = (mode, value) => {
  if (mode === 'follow') {
    return 0;
  }
  if (mode === 'exempt') {
    return -1;
  }
  return Math.max(parseInt(value) || 0, 1);
};

// 切换模式时归一化数字输入，保证「界面显示的值」与「保存下去的值」一致：
// - 离开「自定义」：归零，避免卸载的输入里残留旧值；
// - 进入「自定义」：当前值若不是不小于 1（跟随与豁免的展示值都被钳制成 0），先播种 1，
//   否则界面显示 0 而保存会静默存下 1。
const resolveModeSwitchValue = (nextMode, currentValue) => {
  if (nextMode !== 'custom') {
    return 0;
  }
  return Number(currentValue) >= 1 ? currentValue : 1;
};

const EditUser = (props) => {
  const userId = props.editingUser.id;
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
    daily_quota_limit: 0
  });
  const [dailyTokenMode, setDailyTokenMode] = useState('follow');
  const [dailyQuotaMode, setDailyQuotaMode] = useState('follow');
  const [groupOptions, setGroupOptions] = useState([]);
  const { username, display_name, password, github_id, wechat_id, telegram_id, email, quota, group,
    daily_token_limit, daily_quota_limit } = inputs;
  const handleInputChange = (name, value) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };
  const handleDailyTokenModeChange = (nextMode) => {
    setDailyTokenMode(nextMode);
    setInputs((inputs) => ({
      ...inputs,
      daily_token_limit: resolveModeSwitchValue(nextMode, inputs.daily_token_limit)
    }));
  };
  const handleDailyQuotaModeChange = (nextMode) => {
    setDailyQuotaMode(nextMode);
    setInputs((inputs) => ({
      ...inputs,
      daily_quota_limit: resolveModeSwitchValue(nextMode, inputs.daily_quota_limit)
    }));
  };
  const fetchGroups = async () => {
    try {
      let res = await API.get(`/api/group/`);
      setGroupOptions(res.data.data.map((group) => ({
        label: group,
        value: group
      })));
    } catch (error) {
      showError(error.message);
    }
  };
  const navigate = useNavigate();
  const handleCancel = () => {
    props.handleClose();
  };
  const loadUser = async () => {
    setLoading(true);
    let res = undefined;
    if (userId) {
      res = await API.get(`/api/user/${userId}`);
    } else {
      res = await API.get(`/api/user/self`);
    }
    const { success, message, data } = res.data;
    if (success) {
      data.password = '';
      // 先按后端原始值判定三态，再做钳制：若先钳制，存量为 -1 的豁免会被算成 0（跟随全局），
      // 用户下一次保存就把豁免静默改掉了。
      setDailyTokenMode(resolveLimitMode(data.daily_token_limit));
      setDailyQuotaMode(resolveLimitMode(data.daily_quota_limit));
      // 输入框只在「自定义」模式下使用，展示的必须是正数：负值/空值一律钳制成 0
      setInputs({
        ...data,
        daily_token_limit: Math.max(data.daily_token_limit || 0, 0),
        daily_quota_limit: Math.max(data.daily_quota_limit || 0, 0)
      });
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
  }, [props.editingUser.id]);

  const submit = async () => {
    setLoading(true);
    // 三态编码：follow 必须显式发 0。这两个字段在模型里是 *int64，正是为了绕开 GORM 结构体
    // Updates 跳过零值；发 null 或省略键会让上一次设置的上限静默留存在库里。
    // 模式本身只是组件 state（不在 inputs 里），所以不会有界面专用的键混进请求体。
    const data = {
      ...inputs,
      daily_token_limit: encodeLimitMode(dailyTokenMode, inputs.daily_token_limit),
      daily_quota_limit: encodeLimitMode(dailyQuotaMode, inputs.daily_quota_limit)
    };
    let res = undefined;
    if (userId) {
      data.id = parseInt(userId);
      if (typeof data.quota === 'string') {
        data.quota = parseInt(data.quota);
      }
      res = await API.put(`/api/user/`, data);
    } else {
      res = await API.put(`/api/user/self`, data);
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess('用户信息更新成功！');
      props.refresh();
      props.handleClose();
    } else {
      showError(message);
    }
    setLoading(false);
  };

  return (
    <>
      <SideSheet
        placement={'right'}
        title={<Title level={3}>{'编辑用户'}</Title>}
        headerStyle={{ borderBottom: '1px solid var(--semi-color-border)' }}
        bodyStyle={{ borderBottom: '1px solid var(--semi-color-border)' }}
        visible={props.visible}
        footer={
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Space>
              <Button theme="solid" size={'large'} onClick={submit}>提交</Button>
              <Button theme="solid" size={'large'} type={'tertiary'} onClick={handleCancel}>取消</Button>
            </Space>
          </div>
        }
        closeIcon={null}
        onCancel={() => handleCancel()}
        width={isMobile() ? '100%' : 600}
      >
        <Spin spinning={loading}>
          <div style={{ marginTop: 20 }}>
            <Typography.Text>用户名</Typography.Text>
          </div>
          <Input
            label="用户名"
            name="username"
            placeholder={'请输入新的用户名'}
            onChange={value => handleInputChange('username', value)}
            value={username}
            autoComplete="new-password"
          />
          <div style={{ marginTop: 20 }}>
            <Typography.Text>密码</Typography.Text>
          </div>
          <Input
            label="密码"
            name="password"
            type={'password'}
            placeholder={'请输入新的密码，最短 8 位'}
            onChange={value => handleInputChange('password', value)}
            value={password}
            autoComplete="new-password"
          />
          <div style={{ marginTop: 20 }}>
            <Typography.Text>显示名称</Typography.Text>
          </div>
          <Input
            label="显示名称"
            name="display_name"
            placeholder={'请输入新的显示名称'}
            onChange={value => handleInputChange('display_name', value)}
            value={display_name}
            autoComplete="new-password"
          />
          {
            userId && <>
              <div style={{ marginTop: 20 }}>
                <Typography.Text>分组</Typography.Text>
              </div>
              <Select
                placeholder={'请选择分组'}
                name="group"
                fluid
                search
                selection
                allowAdditions
                additionLabel={'请在系统设置页面编辑分组倍率以添加新的分组：'}
                onChange={value => handleInputChange('group', value)}
                value={inputs.group}
                autoComplete="new-password"
                optionList={groupOptions}
              />
              <div style={{ marginTop: 20 }}>
                <Typography.Text>{`剩余额度${renderQuotaWithPrompt(quota)}`}</Typography.Text>
              </div>
              <Input
                name="quota"
                placeholder={'请输入新的剩余额度'}
                onChange={value => handleInputChange('quota', value)}
                value={quota}
                type={'number'}
                autoComplete="new-password"
              />
              <div style={{ marginTop: 20 }}>
                <Typography.Text>{'单日 Token 上限'}</Typography.Text>
              </div>
              <Select
                placeholder={'请选择单日 Token 上限'}
                name="daily_token_mode"
                fluid
                onChange={value => handleDailyTokenModeChange(value)}
                value={dailyTokenMode}
                autoComplete="new-password"
                optionList={dailyLimitModeOptions}
              />
              {
                dailyTokenMode === 'custom' && <>
                  <div style={{ marginTop: 20 }}>
                    <Typography.Text>{'单日 Token 上限值'}</Typography.Text>
                  </div>
                  <Input
                    name="daily_token_limit"
                    placeholder={'请输入单日 Token 上限，不小于 1'}
                    onChange={value => handleInputChange('daily_token_limit', value)}
                    value={daily_token_limit}
                    type={'number'}
                    min={1}
                    autoComplete="new-password"
                  />
                </>
              }
              <div style={{ marginTop: 20 }}>
                <Typography.Text>{'单日额度上限'}</Typography.Text>
              </div>
              <Select
                placeholder={'请选择单日额度上限'}
                name="daily_quota_mode"
                fluid
                onChange={value => handleDailyQuotaModeChange(value)}
                value={dailyQuotaMode}
                autoComplete="new-password"
                optionList={dailyLimitModeOptions}
              />
              {
                dailyQuotaMode === 'custom' && <>
                  <div style={{ marginTop: 20 }}>
                    <Typography.Text>{`单日额度上限值${renderQuotaWithPrompt(daily_quota_limit)}`}</Typography.Text>
                  </div>
                  <Input
                    name="daily_quota_limit"
                    placeholder={'请输入单日额度上限，不小于 1'}
                    onChange={value => handleInputChange('daily_quota_limit', value)}
                    value={daily_quota_limit}
                    type={'number'}
                    min={1}
                    autoComplete="new-password"
                  />
                </>
              }
            </>
          }
          <Divider style={{ marginTop: 20 }}>以下信息不可修改</Divider>
          <div style={{ marginTop: 20 }}>
            <Typography.Text>已绑定的 GitHub 账户</Typography.Text>
          </div>
          <Input
            name="github_id"
            value={github_id}
            autoComplete="new-password"
            placeholder="此项只读，需要用户通过个人设置页面的相关绑定按钮进行绑定，不可直接修改"
            readonly
          />
          <div style={{ marginTop: 20 }}>
            <Typography.Text>已绑定的微信账户</Typography.Text>
          </div>
          <Input
            name="wechat_id"
            value={wechat_id}
            autoComplete="new-password"
            placeholder="此项只读，需要用户通过个人设置页面的相关绑定按钮进行绑定，不可直接修改"
            readonly
          />
          <Input
            name="telegram_id"
            value={telegram_id}
            autoComplete="new-password"
            placeholder="此项只读，需要用户通过个人设置页面的相关绑定按钮进行绑定，不可直接修改"
            readonly
          />
          <div style={{ marginTop: 20 }}>
            <Typography.Text>已绑定的邮箱账户</Typography.Text>
          </div>
          <Input
            name="email"
            value={email}
            autoComplete="new-password"
            placeholder="此项只读，需要用户通过个人设置页面的相关绑定按钮进行绑定，不可直接修改"
            readonly
          />
        </Spin>
      </SideSheet>
    </>
  );
};

export default EditUser;
