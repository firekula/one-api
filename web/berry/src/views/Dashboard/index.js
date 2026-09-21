import { useEffect, useState } from 'react';
import { Button, FormControl, Grid, InputLabel, MenuItem, Select, Typography } from '@mui/material';
import { DatePicker, LocalizationProvider } from '@mui/x-date-pickers';
import { AdapterDayjs } from '@mui/x-date-pickers/AdapterDayjs';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import { gridSpacing } from 'store/constant';
import StatisticalLineChartCard from './component/StatisticalLineChartCard';
import StatisticalBarChart from './component/StatisticalBarChart';
import { generateChartOptions, getDateRange } from 'utils/chart';
import { API } from 'utils/api';
import { showError, calculateQuota, renderNumber, isAdmin } from 'utils/common';
import UserCard from 'ui-component/cards/UserCard';

// 默认区间：本地今天往前 6 天，按整日对齐（与后端默认的 7 天区间一致）
const initFilters = () => ({
  start: dayjs().subtract(6, 'day').startOf('day').toDate(),
  end: dayjs().endOf('day').toDate(),
  granularity: 'day',
  scope: 'self',
  username: '',
  token_name: '',
  model_name: ''
});

// 候选值来自日志表：错误类型日志的 token_name / model_name 可能是空串，
// 直接渲染会出现一个空白选项，这里统一剔除空白值并去重。
const sanitizeCandidates = (list) =>
  Array.isArray(list) ? [...new Set(list.filter((value) => typeof value === 'string' && value.trim() !== ''))] : [];

const toUnixSeconds = (date) => Math.floor(dayjs(date).valueOf() / 1000);

const Dashboard = () => {
  const userIsAdmin = isAdmin();
  const [isLoading, setLoading] = useState(true);
  const [statisticalData, setStatisticalData] = useState([]);
  const [requestChart, setRequestChart] = useState(null);
  const [quotaChart, setQuotaChart] = useState(null);
  const [tokenChart, setTokenChart] = useState(null);
  const [users, setUsers] = useState([]);
  const [filters, setFilters] = useState(initFilters);
  const [candidates, setCandidates] = useState({ users: [], tokens: [], models: [] });

  // 空值表示不过滤（后端只对非空值追加条件），时间戳为 NaN 时同样不提交
  const buildParams = (extra = {}) => {
    const params = {
      start_timestamp: toUnixSeconds(filters.start),
      end_timestamp: toUnixSeconds(filters.end),
      granularity: filters.granularity,
      scope: userIsAdmin ? filters.scope : 'self',
      username: userIsAdmin && filters.scope === 'all' ? filters.username : '',
      token_name: filters.token_name,
      model_name: filters.model_name,
      ...extra
    };
    Object.keys(params).forEach((key) => {
      const value = params[key];
      if (value === '' || value === null || value === undefined || (typeof value === 'number' && !Number.isFinite(value))) {
        delete params[key];
      }
    });
    return params;
  };

  const userDashboard = async () => {
    try {
      const res = await API.get('/api/user/dashboard', { params: buildParams() });
      const { success, message, data } = res.data;
      if (success) {
        if (data) {
          // 折线图与柱状图共用同一份桶集合
          const dates = getDateRange(filters.start, filters.end, filters.granularity);
          const lineData = getLineDataGroup(data, dates);
          setRequestChart(getLineCardOption(lineData, 'RequestCount'));
          setQuotaChart(getLineCardOption(lineData, 'Quota'));
          setTokenChart(getLineCardOption(lineData, 'PromptTokens'));
          setStatisticalData(getBarDataGroup(data, dates));
        }
      } else {
        showError(message);
      }
    } catch (error) {
      // 请求失败时保留上一次的图表数据，只保证 loading 会结束
    } finally {
      setLoading(false);
    }
  };

  const loadCandidates = async () => {
    try {
      const params = buildParams();
      // 候选值只取决于区间与用户，粒度/令牌/模型对该接口没有意义
      delete params.granularity;
      delete params.token_name;
      delete params.model_name;
      const res = await API.get('/api/log/filters', { params });
      const { success, data } = res.data;
      if (success) {
        const result = data || {};
        setCandidates({
          users: sanitizeCandidates(result.users),
          tokens: sanitizeCandidates(result.tokens),
          models: sanitizeCandidates(result.models)
        });
      }
    } catch (error) {
      // 候选值拉取失败不能阻塞总览，退化成只有「全部」可选的空列表
      setCandidates({ users: [], tokens: [], models: [] });
    }
  };

  const loadUser = async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      setUsers(data);
    } else {
      showError(message);
    }
  };

  // 区间与粒度变化时刷新总览
  useEffect(() => {
    userDashboard();
  }, [filters]);

  useEffect(() => {
    loadUser();
  }, []);

  // 候选值只跟区间 / 用户 / 范围有关：令牌与模型的选择不影响候选集
  useEffect(() => {
    loadCandidates();
  }, [filters.start, filters.end, filters.username, filters.scope]);

  const setQuickRange = (daysAgo) => {
    setFilters({
      ...filters,
      start: dayjs().subtract(daysAgo, 'day').startOf('day').toDate(),
      end: dayjs().endOf('day').toDate()
    });
  };

  return (
    <Grid container spacing={gridSpacing}>
      <Grid item xs={12}>
        <Grid container spacing={gridSpacing} alignItems="center">
          <Grid item>
            <Button variant="outlined" onClick={() => setQuickRange(0)}>
              今天
            </Button>
            <Button variant="outlined" onClick={() => setQuickRange(6)}>
              近 7 天
            </Button>
            <Button variant="outlined" onClick={() => setQuickRange(29)}>
              近 30 天
            </Button>
          </Grid>
          <Grid item>
            <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale={'zh-cn'}>
              <DatePicker
                label="起始日期"
                value={dayjs(filters.start)}
                onChange={(v) => v && setFilters({ ...filters, start: v.startOf('day').toDate() })}
              />
            </LocalizationProvider>
          </Grid>
          <Grid item>
            <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale={'zh-cn'}>
              <DatePicker
                label="结束日期"
                value={dayjs(filters.end)}
                onChange={(v) => v && setFilters({ ...filters, end: v.endOf('day').toDate() })}
              />
            </LocalizationProvider>
          </Grid>
          <Grid item>
            <FormControl size="small">
              <InputLabel>粒度</InputLabel>
              <Select
                label="粒度"
                value={filters.granularity}
                onChange={(e) => setFilters({ ...filters, granularity: e.target.value })}
              >
                <MenuItem value="day">天</MenuItem>
                <MenuItem value="hour">小时</MenuItem>
              </Select>
            </FormControl>
          </Grid>
        </Grid>
      </Grid>
      <Grid item xs={12}>
        <Grid container spacing={gridSpacing} alignItems="center">
          {userIsAdmin && (
            <Grid item>
              <FormControl size="small" sx={{ minWidth: 120 }}>
                <InputLabel>数据范围</InputLabel>
                <Select
                  label="数据范围"
                  value={filters.scope}
                  onChange={(e) => setFilters({ ...filters, scope: e.target.value, username: '', token_name: '' })}
                >
                  <MenuItem value="self">仅自己</MenuItem>
                  <MenuItem value="all">全站</MenuItem>
                </Select>
              </FormControl>
            </Grid>
          )}
          {userIsAdmin && filters.scope === 'all' && (
            <Grid item>
              <FormControl size="small" sx={{ minWidth: 160 }}>
                <InputLabel>用户</InputLabel>
                <Select
                  label="用户"
                  value={filters.username}
                  onChange={(e) => setFilters({ ...filters, username: e.target.value, token_name: '' })}
                  MenuProps={{ PaperProps: { style: { maxHeight: 200 } } }}
                >
                  <MenuItem value="">全部</MenuItem>
                  {candidates.users.map((name) => (
                    <MenuItem key={name} value={name}>
                      {name}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
          )}
          <Grid item>
            <FormControl size="small" sx={{ minWidth: 160 }}>
              <InputLabel>令牌</InputLabel>
              <Select
                label="令牌"
                value={filters.token_name}
                onChange={(e) => setFilters({ ...filters, token_name: e.target.value })}
                MenuProps={{ PaperProps: { style: { maxHeight: 200 } } }}
              >
                <MenuItem value="">全部</MenuItem>
                {candidates.tokens.map((name) => (
                  <MenuItem key={name} value={name}>
                    {name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>
          <Grid item>
            <FormControl size="small" sx={{ minWidth: 160 }}>
              <InputLabel>模型</InputLabel>
              <Select
                label="模型"
                value={filters.model_name}
                onChange={(e) => setFilters({ ...filters, model_name: e.target.value })}
                MenuProps={{ PaperProps: { style: { maxHeight: 200 } } }}
              >
                <MenuItem value="">全部</MenuItem>
                {candidates.models.map((name) => (
                  <MenuItem key={name} value={name}>
                    {name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Grid>
        </Grid>
      </Grid>
      <Grid item xs={12}>
        <Grid container spacing={gridSpacing}>
          <Grid item lg={4} xs={12}>
            <StatisticalLineChartCard
              isLoading={isLoading}
              title="今日请求量"
              chartData={requestChart?.chartData}
              todayValue={requestChart?.todayValue}
            />
          </Grid>
          <Grid item lg={4} xs={12}>
            <StatisticalLineChartCard
              isLoading={isLoading}
              title="今日消费"
              chartData={quotaChart?.chartData}
              todayValue={quotaChart?.todayValue}
            />
          </Grid>
          <Grid item lg={4} xs={12}>
            <StatisticalLineChartCard
              isLoading={isLoading}
              title="今日 token"
              chartData={tokenChart?.chartData}
              todayValue={tokenChart?.todayValue}
            />
          </Grid>
        </Grid>
      </Grid>
      <Grid item xs={12}>
        <Grid container spacing={gridSpacing}>
          <Grid item lg={8} xs={12}>
            <StatisticalBarChart isLoading={isLoading} chartDatas={statisticalData} />
          </Grid>
          <Grid item lg={4} xs={12}>
            <UserCard>
              <Grid container spacing={gridSpacing} justifyContent="center" alignItems="center" paddingTop={'20px'}>
                <Grid item xs={4}>
                  <Typography variant="h4">余额：</Typography>
                </Grid>
                <Grid item xs={8}>
                  <Typography variant="h3"> {users?.quota ? '$' + calculateQuota(users.quota) : '未知'}</Typography>
                </Grid>
                <Grid item xs={4}>
                  <Typography variant="h4">已使用：</Typography>
                </Grid>
                <Grid item xs={8}>
                  <Typography variant="h3"> {users?.used_quota ? '$' + calculateQuota(users.used_quota) : '未知'}</Typography>
                </Grid>
                <Grid item xs={4}>
                  <Typography variant="h4">调用次数：</Typography>
                </Grid>
                <Grid item xs={8}>
                  <Typography variant="h3"> {users?.request_count || '未知'}</Typography>
                </Grid>
              </Grid>
            </UserCard>
          </Grid>
        </Grid>
      </Grid>
    </Grid>
  );
};
export default Dashboard;

function getLineDataGroup(statisticalData, dates) {
  let groupedData = statisticalData.reduce((acc, cur) => {
    if (!acc[cur.Day]) {
      acc[cur.Day] = {
        date: cur.Day,
        RequestCount: 0,
        Quota: 0,
        PromptTokens: 0,
        CompletionTokens: 0
      };
    }
    acc[cur.Day].RequestCount += cur.RequestCount;
    acc[cur.Day].Quota += cur.Quota;
    acc[cur.Day].PromptTokens += cur.PromptTokens;
    acc[cur.Day].CompletionTokens += cur.CompletionTokens;
    return acc;
  }, {});
  // 桶集合由请求区间与粒度生成：不在集合里的行（越界 / 时区与后端不一致）不参与出图，
  // 保证折线图与柱状图用的是同一份桶
  return dates.map((day) => {
    if (!groupedData[day]) {
      return {
        date: day,
        RequestCount: 0,
        Quota: 0,
        PromptTokens: 0,
        CompletionTokens: 0
      };
    } else {
      return groupedData[day];
    }
  });
}

function getBarDataGroup(data, dates) {
  const result = [];
  const map = new Map();

  for (const item of data) {
    if (!map.has(item.ModelName)) {
      const newData = { name: item.ModelName, data: new Array(dates.length).fill(0) };
      map.set(item.ModelName, newData);
      result.push(newData);
    }
    const index = dates.indexOf(item.Day);
    // 不在桶集合里的行直接跳过：既不能落到 indexOf 的 -1 上覆盖最后一个桶，
    // 也不能被静默丢弃在错误的下标处
    if (index === -1) {
      continue;
    }
    map.get(item.ModelName).data[index] = calculateQuota(item.Quota, 3);
  }

  return { data: result, xaxis: dates };
}

function getLineCardOption(lineDataGroup, field) {
  let todayValue = 0;
  let chartData = null;
  const lastItem = lineDataGroup.length - 1;
  let lineData = lineDataGroup.map((item, index) => {
    let tmp = {
      date: item.date,
      value: item[field]
    };
    switch (field) {
      case 'Quota':
        tmp.value = calculateQuota(item.Quota, 3);
        break;
      case 'PromptTokens':
        tmp.value += item.CompletionTokens;
        break;
    }

    if (index == lastItem) {
      todayValue = tmp.value;
    }
    return tmp;
  });

  switch (field) {
    case 'RequestCount':
      chartData = generateChartOptions(lineData, '次');
      todayValue = renderNumber(todayValue);
      break;
    case 'Quota':
      chartData = generateChartOptions(lineData, '美元');
      todayValue = '$' + renderNumber(todayValue);
      break;
    case 'PromptTokens':
      chartData = generateChartOptions(lineData, '');
      todayValue = renderNumber(todayValue);
      break;
  }

  return { chartData: chartData, todayValue: todayValue };
}
