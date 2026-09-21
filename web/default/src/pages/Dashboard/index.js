import React, {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Button, Card, Form, Grid} from 'semantic-ui-react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {API} from '../../helpers/api';
import {isAdmin, timestamp2string} from '../../helpers/utils';
import './Dashboard.css';

// 在 Dashboard 组件内添加自定义配置
const chartConfig = {
  lineChart: {
    style: {
      background: '#fff',
      borderRadius: '8px',
    },
    line: {
      strokeWidth: 2,
      dot: false,
      activeDot: { r: 4 },
    },
    grid: {
      vertical: false,
      horizontal: true,
      opacity: 0.1,
    },
  },
  colors: {
    requests: '#4318FF',
    quota: '#00B5D8',
    tokens: '#6C63FF',
  },
  barColors: [
    '#4318FF', // 深紫色
    '#00B5D8', // 青色
    '#6C63FF', // 紫色
    '#05CD99', // 绿色
    '#FFB547', // 橙色
    '#FF5E7D', // 粉色
    '#41B883', // 翠绿
    '#7983FF', // 淡紫
    '#FF8F6B', // 珊瑚色
    '#49BEFF', // 天蓝
  ],
};

// 「YYYY-MM-DD HH:mm:ss」这种带空格的写法在 Safari 里无法被 Date 直接解析，
// 换成 ISO 的「YYYY-MM-DDTHH:mm:ss」形式后按本地时区解析。
const parseLocalDateTime = (value) => new Date(String(value).replace(' ', 'T'));
const toUnixSeconds = (value) =>
  Math.floor(parseLocalDateTime(value).getTime() / 1000);

// 候选值来自日志表：错误类型日志的 token_name / model_name 可能是空串，
// 直接渲染会出现一个空白选项，这里统一剔除空白值并去重。
const sanitizeCandidates = (list) =>
  Array.isArray(list)
    ? [...new Set(list.filter((v) => typeof v === 'string' && v.trim() !== ''))]
    : [];

const Dashboard = () => {
  const { t } = useTranslation();
  const [data, setData] = useState([]);
  const [summaryData, setSummaryData] = useState({
    todayRequests: 0,
    todayQuota: 0,
    todayTokens: 0,
  });

  const isAdminUser = isAdmin();
  const [filters, setFilters] = useState({
    start: timestamp2string(Date.now() / 1000 - 86400 * 6),
    end: timestamp2string(Date.now() / 1000),
    granularity: 'day',
    scope: 'self',
    username: '',
    token_name: '',
    model_name: '',
  });
  const [candidates, setCandidates] = useState({
    users: [],
    tokens: [],
    models: [],
  });

  useEffect(() => {
    fetchDashboardData();
  }, [filters]);

  useEffect(() => {
    fetchCandidates();
    // 用户名变化时令牌候选值要跟着收窄
  }, [filters.start, filters.end, filters.username, filters.scope]);

  const buildQuery = (params) => {
    const search = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== '' && value !== undefined && value !== null) {
        search.append(key, value);
      }
    });
    return search.toString();
  };

  const fetchCandidates = async () => {
    try {
      const query = buildQuery({
        start_timestamp: toUnixSeconds(filters.start),
        end_timestamp: toUnixSeconds(filters.end),
        username: filters.scope === 'all' ? filters.username : '',
      });
      const response = await API.get(`/api/log/filters?${query}`);
      if (response.data.success) {
        const result = response.data.data || {};
        setCandidates({
          users: sanitizeCandidates(result.users),
          tokens: sanitizeCandidates(result.tokens),
          models: sanitizeCandidates(result.models),
        });
      }
    } catch (error) {
      // 候选值拉取失败不阻塞总览本身，退化为只保留「全部」
      setCandidates({ users: [], tokens: [], models: [] });
    }
  };

  const fetchDashboardData = async () => {
    try {
      const query = buildQuery({
        start_timestamp: toUnixSeconds(filters.start),
        end_timestamp: toUnixSeconds(filters.end),
        granularity: filters.granularity,
        scope: filters.scope,
        username: filters.scope === 'all' ? filters.username : '',
        token_name: filters.token_name,
        model_name: filters.model_name,
      });
      const response = await API.get(`/api/user/dashboard?${query}`);
      if (response.data.success) {
        const dashboardData = response.data.data || [];
        setData(dashboardData);
        calculateSummary(dashboardData);
      }
    } catch (error) {
      setData([]);
      calculateSummary([]);
    }
  };

  const setQuickRange = (daysAgo) => {
    const pad = (n) => String(n).padStart(2, '0');
    const formatDay = (d) =>
      `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    const end = new Date();
    const start = new Date();
    start.setDate(start.getDate() - daysAgo);
    setFilters({
      ...filters,
      start: `${formatDay(start)} 00:00:00`,
      end: `${formatDay(end)} 23:59:59`,
    });
  };

  const calculateSummary = (dashboardData) => {
    if (!Array.isArray(dashboardData) || dashboardData.length === 0) {
      setSummaryData({
        todayRequests: 0,
        todayQuota: 0,
        todayTokens: 0,
      });
      return;
    }

    const pad = (n) => String(n).padStart(2, '0');
    const now = new Date();
    const today = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(
      now.getDate()
    )}`;
    // 用前缀匹配：day 桶是「今天」，hour 桶是「今天 HH:00」，
    // 两种粒度下都能正确汇总出今日数据。
    const todayData = dashboardData.filter((item) =>
      String(item.Day || '').startsWith(today)
    );

    const summary = {
      todayRequests: todayData.reduce(
        (sum, item) => sum + item.RequestCount,
        0
      ),
      todayQuota:
        todayData.reduce((sum, item) => sum + item.Quota, 0) / 1000000,
      todayTokens: todayData.reduce(
        (sum, item) => sum + item.PromptTokens + item.CompletionTokens,
        0
      ),
    };

    setSummaryData(summary);
  };

  // 按请求区间与粒度生成桶标签，本地时区，与后端分桶口径一致。
  // 用 Date 加法而非固定毫秒步长，避免夏令时导致偏移。
  const buildBuckets = () => {
    const pad = (n) => String(n).padStart(2, '0');
    const format = (d) =>
      filters.granularity === 'hour'
        ? `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(
            d.getDate()
          )} ${pad(d.getHours())}:00`
        : `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

    const buckets = [];
    const cursor = parseLocalDateTime(filters.start);
    if (filters.granularity === 'hour') {
      cursor.setMinutes(0, 0, 0);
    } else {
      cursor.setHours(0, 0, 0, 0);
    }
    const end = parseLocalDateTime(filters.end);
    let guard = 0;
    while (cursor <= end && guard < 4000) {
      buckets.push(format(cursor));
      if (filters.granularity === 'hour') {
        cursor.setHours(cursor.getHours() + 1);
      } else {
        cursor.setDate(cursor.getDate() + 1);
      }
      guard += 1;
    }
    return buckets;
  };

  // 处理数据以供折线图使用，按请求区间与粒度补齐缺失的桶
  const processTimeSeriesData = () => {
    const dailyData = {};
    buildBuckets().forEach((bucket) => {
      dailyData[bucket] = { date: bucket, requests: 0, quota: 0, tokens: 0 };
    });
    data.forEach((item) => {
      // 后端返回的桶理论上都在范围内，越界时跳过而不是抛异常
      if (!dailyData[item.Day]) {
        return;
      }
      dailyData[item.Day].requests += item.RequestCount;
      dailyData[item.Day].quota += item.Quota / 1000000;
      dailyData[item.Day].tokens += item.PromptTokens + item.CompletionTokens;
    });
    return Object.values(dailyData).sort((a, b) =>
      a.date.localeCompare(b.date)
    );
  };

  // 处理数据以供堆叠柱状图使用，桶集合与折线图保持一致
  const processModelData = () => {
    const buckets = buildBuckets();
    const models = [...new Set(data.map((item) => item.ModelName))];
    const timeData = {};
    buckets.forEach((bucket) => {
      timeData[bucket] = { date: bucket };
      models.forEach((model) => {
        timeData[bucket][model] = 0;
      });
    });
    data.forEach((item) => {
      if (!timeData[item.Day]) {
        return;
      }
      timeData[item.Day][item.ModelName] =
        item.PromptTokens + item.CompletionTokens;
    });
    return Object.values(timeData).sort((a, b) =>
      a.date.localeCompare(b.date)
    );
  };

  // 获取所有唯一的模型名称；只统计落在当前桶集合内的数据行，
  // 保证图例与堆叠柱状图的桶集合一致（越界行在两图中都不出现）
  const getUniqueModels = () => {
    const buckets = new Set(buildBuckets());
    return [
      ...new Set(
        data
          .filter((item) => buckets.has(item.Day))
          .map((item) => item.ModelName)
      ),
    ];
  };

  const timeSeriesData = processTimeSeriesData();
  const modelData = processModelData();
  const models = getUniqueModels();

  // 生成随机颜色
  const getRandomColor = (index) => {
    return chartConfig.barColors[index % chartConfig.barColors.length];
  };

  // 添加一个日期格式化函数
  const formatDate = (dateStr) => {
    // 兼容 day（YYYY-MM-DD）与 hour（YYYY-MM-DD HH:00）两种桶标签
    if (dateStr.length > 10) {
      return dateStr.slice(5, 16);
    }
    const date = new Date(dateStr);
    return date.toLocaleDateString('zh-CN', {
      month: 'numeric',
      day: 'numeric',
    });
  };

  // 修改所有 XAxis 配置
  const xAxisConfig = {
    dataKey: 'date',
    axisLine: false,
    tickLine: false,
    tick: {
      fontSize: 12,
      fill: '#A3AED0',
      textAnchor: 'middle', // 文本居中对齐
    },
    tickFormatter: formatDate,
    interval: 0,
    minTickGap: 5,
    padding: { left: 30, right: 30 }, // 增加两侧的内边距，确保首尾标签完整显示
  };

  return (
    <div className='dashboard-container'>
      {/* 筛选栏：快捷区间 + 起止日期 + 粒度 + 范围/用户/令牌/模型 */}
      <Card fluid className='chart-card'>
        <Card.Content>
          <Form size='small'>
            <Form.Group inline>
              <Button
                size='small'
                basic
                onClick={() => setQuickRange(0)}
                content={t('dashboard.filters.today')}
              />
              <Button
                size='small'
                basic
                onClick={() => setQuickRange(6)}
                content={t('dashboard.filters.last_7_days')}
              />
              <Button
                size='small'
                basic
                onClick={() => setQuickRange(29)}
                content={t('dashboard.filters.last_30_days')}
              />
              <Form.Input
                label={t('dashboard.filters.start')}
                type='date'
                value={filters.start.slice(0, 10)}
                onChange={(e) =>
                  setFilters({ ...filters, start: `${e.target.value} 00:00:00` })
                }
              />
              <Form.Input
                label={t('dashboard.filters.end')}
                type='date'
                value={filters.end.slice(0, 10)}
                onChange={(e) =>
                  setFilters({ ...filters, end: `${e.target.value} 23:59:59` })
                }
              />
              <Form.Select
                label={t('dashboard.filters.granularity')}
                options={[
                  {
                    key: 'day',
                    text: t('dashboard.filters.granularity_day'),
                    value: 'day',
                  },
                  {
                    key: 'hour',
                    text: t('dashboard.filters.granularity_hour'),
                    value: 'hour',
                  },
                ]}
                value={filters.granularity}
                onChange={(e, { value }) =>
                  setFilters({ ...filters, granularity: value })
                }
              />
            </Form.Group>
            <Form.Group inline>
              {isAdminUser && (
                <Form.Select
                  label={t('dashboard.filters.scope')}
                  options={[
                    {
                      key: 'self',
                      text: t('dashboard.filters.scope_self'),
                      value: 'self',
                    },
                    {
                      key: 'all',
                      text: t('dashboard.filters.scope_all'),
                      value: 'all',
                    },
                  ]}
                  value={filters.scope}
                  onChange={(e, { value }) =>
                    setFilters({
                      ...filters,
                      scope: value,
                      username: '',
                      token_name: '',
                    })
                  }
                />
              )}
              {isAdminUser && filters.scope === 'all' && (
                <Form.Select
                  label={t('dashboard.filters.username')}
                  options={[
                    { key: '', text: t('dashboard.filters.all'), value: '' },
                    ...candidates.users.map((name) => ({
                      key: name,
                      text: name,
                      value: name,
                    })),
                  ]}
                  value={filters.username}
                  onChange={(e, { value }) =>
                    setFilters({ ...filters, username: value, token_name: '' })
                  }
                />
              )}
              <Form.Select
                label={t('dashboard.filters.token_name')}
                options={[
                  { key: '', text: t('dashboard.filters.all'), value: '' },
                  ...candidates.tokens.map((name) => ({
                    key: name,
                    text: name,
                    value: name,
                  })),
                ]}
                value={filters.token_name}
                onChange={(e, { value }) =>
                  setFilters({ ...filters, token_name: value })
                }
              />
              <Form.Select
                label={t('dashboard.filters.model_name')}
                options={[
                  { key: '', text: t('dashboard.filters.all'), value: '' },
                  ...candidates.models.map((name) => ({
                    key: name,
                    text: name,
                    value: name,
                  })),
                ]}
                value={filters.model_name}
                onChange={(e, { value }) =>
                  setFilters({ ...filters, model_name: value })
                }
              />
            </Form.Group>
          </Form>
        </Card.Content>
      </Card>

      {/* 三个并排的折线图 */}
      <Grid columns={3} stackable className='charts-grid'>
        <Grid.Column>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                {t('dashboard.charts.requests.title')}
                {/* <span className='stat-value'>{summaryData.todayRequests}</span> */}
              </Card.Header>
              <div className='chart-container'>
                <ResponsiveContainer
                  width='100%'
                  height={120}
                  margin={{ left: 10, right: 10 }} // 调整容器边距
                >
                  <LineChart data={timeSeriesData}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      vertical={chartConfig.lineChart.grid.vertical}
                      horizontal={chartConfig.lineChart.grid.horizontal}
                      opacity={chartConfig.lineChart.grid.opacity}
                    />
                    <XAxis {...xAxisConfig} />
                    <YAxis hide={true} />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      formatter={(value) => [
                        value,
                        t('dashboard.charts.requests.tooltip'),
                      ]}
                      labelFormatter={(label) =>
                        `${t(
                          'dashboard.statistics.tooltip.date'
                        )}: ${formatDate(label)}`
                      }
                    />
                    <Line
                      type='monotone'
                      dataKey='requests'
                      stroke={chartConfig.colors.requests}
                      strokeWidth={chartConfig.lineChart.line.strokeWidth}
                      dot={chartConfig.lineChart.line.dot}
                      activeDot={chartConfig.lineChart.line.activeDot}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </Card.Content>
          </Card>
        </Grid.Column>

        <Grid.Column>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                {t('dashboard.charts.quota.title')}
                {/* <span className='stat-value'>
                  ${summaryData.todayQuota.toFixed(3)}
                </span> */}
              </Card.Header>
              <div className='chart-container'>
                <ResponsiveContainer
                  width='100%'
                  height={120}
                  margin={{ left: 10, right: 10 }} // 调整容器边距
                >
                  <LineChart data={timeSeriesData}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      vertical={chartConfig.lineChart.grid.vertical}
                      horizontal={chartConfig.lineChart.grid.horizontal}
                      opacity={chartConfig.lineChart.grid.opacity}
                    />
                    <XAxis {...xAxisConfig} />
                    <YAxis hide={true} />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      formatter={(value) => [
                        value.toFixed(6),
                        t('dashboard.charts.quota.tooltip'),
                      ]}
                      labelFormatter={(label) =>
                        `${t(
                          'dashboard.statistics.tooltip.date'
                        )}: ${formatDate(label)}`
                      }
                    />
                    <Line
                      type='monotone'
                      dataKey='quota'
                      stroke={chartConfig.colors.quota}
                      strokeWidth={chartConfig.lineChart.line.strokeWidth}
                      dot={chartConfig.lineChart.line.dot}
                      activeDot={chartConfig.lineChart.line.activeDot}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </Card.Content>
          </Card>
        </Grid.Column>

        <Grid.Column>
          <Card fluid className='chart-card'>
            <Card.Content>
              <Card.Header>
                {t('dashboard.charts.tokens.title')}
                {/* <span className='stat-value'>{summaryData.todayTokens}</span> */}
              </Card.Header>
              <div className='chart-container'>
                <ResponsiveContainer
                  width='100%'
                  height={120}
                  margin={{ left: 10, right: 10 }} // 调整容器边距
                >
                  <LineChart data={timeSeriesData}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      vertical={chartConfig.lineChart.grid.vertical}
                      horizontal={chartConfig.lineChart.grid.horizontal}
                      opacity={chartConfig.lineChart.grid.opacity}
                    />
                    <XAxis {...xAxisConfig} />
                    <YAxis hide={true} />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      formatter={(value) => [
                        value,
                        t('dashboard.charts.tokens.tooltip'),
                      ]}
                      labelFormatter={(label) =>
                        `${t(
                          'dashboard.statistics.tooltip.date'
                        )}: ${formatDate(label)}`
                      }
                    />
                    <Line
                      type='monotone'
                      dataKey='tokens'
                      stroke={chartConfig.colors.tokens}
                      strokeWidth={chartConfig.lineChart.line.strokeWidth}
                      dot={chartConfig.lineChart.line.dot}
                      activeDot={chartConfig.lineChart.line.activeDot}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </Card.Content>
          </Card>
        </Grid.Column>
      </Grid>

      {/* 模型使用统计 */}
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header>{t('dashboard.statistics.title')}</Card.Header>
          <div className='chart-container'>
            <ResponsiveContainer width='100%' height={300}>
              <BarChart data={modelData}>
                <CartesianGrid
                  strokeDasharray='3 3'
                  vertical={false}
                  opacity={0.1}
                />
                <XAxis {...xAxisConfig} />
                <YAxis
                  axisLine={false}
                  tickLine={false}
                  tick={{ fontSize: 12, fill: '#A3AED0' }}
                />
                <Tooltip
                  contentStyle={{
                    background: '#fff',
                    border: 'none',
                    borderRadius: '4px',
                    boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                  }}
                  labelFormatter={(label) =>
                    `${t('dashboard.statistics.tooltip.date')}: ${formatDate(
                      label
                    )}`
                  }
                />
                <Legend
                  wrapperStyle={{
                    paddingTop: '20px',
                  }}
                />
                {models.map((model, index) => (
                  <Bar
                    key={model}
                    dataKey={model}
                    stackId='a'
                    fill={getRandomColor(index)}
                    name={model}
                    radius={[4, 4, 0, 0]}
                  />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>
        </Card.Content>
      </Card>
    </div>
  );
};

export default Dashboard;
