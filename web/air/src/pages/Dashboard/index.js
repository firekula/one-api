import React, {useCallback, useEffect, useRef, useState} from 'react';
import {Button, DatePicker, Form, Layout, Select, Spin} from '@douyinfe/semi-ui';
import VChart from '@visactor/vchart';
import {API, isAdmin, showError, timestamp2string} from '../../helpers';
import {
    getQuotaWithUnit,
    modelColorMap,
    renderNumber,
    renderQuota,
    renderQuotaNumberWithDigit
} from '../../helpers/render';

// 「YYYY-MM-DD HH:mm:ss」这种带空格的写法在部分浏览器里不能被 Date 直接解析，
// 换成 ISO 的「YYYY-MM-DDTHH:mm:ss」后按本地时区解析。
const parseLocalDateTime = (value) => new Date(String(value).replace(' ', 'T'));
const toUnixSeconds = (value) => Math.floor(parseLocalDateTime(value).getTime() / 1000);

const pad = (n) => String(n).padStart(2, '0');

// 默认区间：本地今天往前 6 天，按整日对齐，与后端 GetUserDashboard 的默认 7 天区间
// （今天 00:00:00 ~ 23:59:59）口径一致。
const buildDefaultRange = () => {
    const now = new Date();
    const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 6, 0, 0, 0);
    const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59);
    return {
        start_timestamp: timestamp2string(start.getTime() / 1000),
        end_timestamp: timestamp2string(end.getTime() / 1000)
    };
};

const initialFilters = () => ({
    ...buildDefaultRange(),
    granularity: 'day',
    scope: 'self',
    username: '',
    token_name: '',
    model_name: ''
});

// 范围与用户是令牌候选值的上游：换范围/换用户后原来的令牌选择已经不在候选集里，
// 属于真正有依赖关系的联动，必须一起清掉（不是可见性补丁）。
const scopeChangePatch = (scope) => ({scope, username: '', token_name: ''});
const usernameChangePatch = (username) => ({username, token_name: ''});

// 候选值来自日志表：错误类型日志的 token_name / model_name 是空串，接口又不按日志类型过滤，
// 不处理的话每个下拉框都会多出一个空白选项。统一剔除空白值并去重。
const sanitizeCandidates = (list) =>
    Array.isArray(list)
        ? [...new Set(list.filter((value) => typeof value === 'string' && value.trim() !== ''))]
        : [];

// 候选值只覆盖所选区间（用户变化后令牌还会再收窄），当前选中值可能落在候选之外。
// Semi 的 Select 对列表里没有的值会退化显示原始字符串，这里仍然把选中值补进列表，
// 让它在触发器和下拉列表里都保持可见、可点选（否则用户会以为筛选被清掉了）。
const withSelected = (list, selected) =>
    selected && !list.includes(selected) ? [selected, ...list] : list;

const buildOptions = (list, allLabel = '全部') => [
    {label: allLabel, value: ''},
    ...list.map((value) => ({label: value, value}))
];

const formatBucket = (date, granularity) =>
    granularity === 'hour'
        ? `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:00`
        : `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;

// 桶网格由请求区间与粒度算出，按日历步进（不是固定毫秒步长，夏令时不会把标签推偏）；
// guard 兜住异常区间造成的死循环。
const buildBuckets = (start, end, granularity) => {
    const buckets = [];
    const cursor = parseLocalDateTime(start);
    if (Number.isNaN(cursor.getTime())) {
        return buckets;
    }
    if (granularity === 'hour') {
        cursor.setMinutes(0, 0, 0);
    } else {
        cursor.setHours(0, 0, 0, 0);
    }
    const endDate = parseLocalDateTime(end);
    if (Number.isNaN(endDate.getTime())) {
        return buckets;
    }
    let guard = 0;
    while (cursor <= endDate && guard < 9000) {
        buckets.push(formatBucket(cursor, granularity));
        if (granularity === 'hour') {
            cursor.setHours(cursor.getHours() + 1);
        } else {
            cursor.setDate(cursor.getDate() + 1);
        }
        guard += 1;
    }
    return buckets;
};

// 与后端 GetUserDashboard 的上限保持一致：小时 90 天，天 366 天。
// 越界请求后端会返回 400（axios 拦截器只提示 statusCode），这里先挡下来并给出具体原因。
const maxRangeDays = (granularity) => (granularity === 'hour' ? 90 : 366);

const validateRange = (startTimestamp, endTimestamp, granularity) => {
    if (!Number.isFinite(startTimestamp) || !Number.isFinite(endTimestamp)) {
        return '时间格式不正确';
    }
    if (startTimestamp > endTimestamp) {
        return '起始时间不能晚于结束时间';
    }
    if (endTimestamp - startTimestamp > maxRangeDays(granularity) * 86400) {
        return granularity === 'hour' ? '小时粒度最多查询 90 天' : '天粒度最多查询 366 天';
    }
    return '';
};

// 图表 spec 每次重建而不是就地改一个共享对象：updateSpec 拿到的是全新的 spec，
// 不会出现上一份数据残留或依赖对象引用不变的坑。
const createLineSpec = (values, subtext) => ({
    type: 'bar',
    data: [
        {
            id: 'barData',
            values
        }
    ],
    xField: 'Time',
    yField: 'Usage',
    seriesField: 'Model',
    stack: true,
    legends: {
        visible: true
    },
    title: {
        visible: true,
        text: '模型消耗分布',
        subtext
    },
    bar: {
        // The state style of bar
        state: {
            hover: {
                stroke: '#000',
                lineWidth: 1
            }
        }
    },
    tooltip: {
        mark: {
            content: [
                {
                    key: datum => datum['Model'],
                    value: datum => renderQuotaNumberWithDigit(parseFloat(datum['Usage']), 4)
                }
            ]
        },
        dimension: {
            content: [
                {
                    key: datum => datum['Model'],
                    value: datum => datum['Usage']
                }
            ],
            updateContent: array => {
                // sort by value
                array.sort((a, b) => b.value - a.value);
                // add $
                let sum = 0;
                for (let i = 0; i < array.length; i++) {
                    sum += parseFloat(array[i].value);
                    array[i].value = renderQuotaNumberWithDigit(parseFloat(array[i].value), 4);
                }
                // add to first
                array.unshift({
                    key: '总计',
                    value: renderQuotaNumberWithDigit(sum, 4)
                });
                return array;
            }
        }
    },
    color: {
        specified: modelColorMap
    }
});

const createPieSpec = (values, subtext) => ({
    type: 'pie',
    data: [
        {
            id: 'id0',
            values
        }
    ],
    outerRadius: 0.8,
    innerRadius: 0.5,
    padAngle: 0.6,
    valueField: 'value',
    categoryField: 'type',
    pie: {
        style: {
            cornerRadius: 10
        },
        state: {
            hover: {
                outerRadius: 0.85,
                stroke: '#000',
                lineWidth: 1
            },
            selected: {
                outerRadius: 0.85,
                stroke: '#000',
                lineWidth: 1
            }
        }
    },
    title: {
        visible: true,
        text: '模型调用次数占比',
        subtext
    },
    legends: {
        visible: true,
        orient: 'left'
    },
    label: {
        visible: true
    },
    tooltip: {
        mark: {
            content: [
                {
                    key: datum => datum['type'],
                    value: datum => renderNumber(datum['value'])
                }
            ]
        }
    },
    color: {
        specified: modelColorMap
    }
});

// 每个桶都补一行 0 值，柱状图的 x 轴才会覆盖整个请求区间（点数过多时退化成只画有数据的桶）
const MAX_FILLED_POINTS = 4000;

// 桶轴 = 请求区间算出的网格 ∪ 响应里实际回传的标签。
// 网格是用浏览器本地日历算的，后端却是按服务器本地时间分桶，两者不一致时回传的标签
// 可能整批落在网格之外；只认网格会把数据整批丢掉（小时粒度 + 不足一天时最明显：图表全空
// 但日志明明存在）。两种标签（YYYY-MM-DD 与 YYYY-MM-DD HH:00）都是零填充，直接字典序排序即可。
const buildBucketAxis = (grid, rows) => {
    const merged = new Set(grid);
    rows.forEach((row) => {
        if (typeof row.Day === 'string' && row.Day !== '') {
            merged.add(row.Day);
        }
    });
    return [...merged].sort();
};

// 筛选栏里「标签 + 单个受控组件」的一格。控件直接读 filters，不再经过 Form.* 的 field 包装。
const FilterItem = ({label, children}) => (
    <div style={{display: 'inline-flex', alignItems: 'center', marginRight: 16, marginBottom: 12}}>
        <Form.Label style={{marginRight: 8, whiteSpace: 'nowrap'}}>{label}</Form.Label>
        {children}
    </div>
);

const Dashboard = () => {
    const isAdminUser = isAdmin();
    const [filters, setFiltersState] = useState(initialFilters);
    const [candidates, setCandidates] = useState({users: [], tokens: [], models: []});
    const [loading, setLoading] = useState(false);
    const [loaded, setLoaded] = useState(false);
    const [hasData, setHasData] = useState(false);
    const lineChartRef = useRef(null);
    const pieChartRef = useRef(null);

    const setFilters = (patch) => setFiltersState((prev) => ({...prev, ...patch}));

    // vchart 是命令式的：实例只在挂载时建一次，之后靠 updateSpec + reLayout 更新。
    // 实例存在 ref 里而不是 state 里，筛选变化时能直接拿到实例，不必等重渲染。
    const createCharts = () => {
        if (!lineChartRef.current && document.getElementById('model_data')) {
            lineChartRef.current = new VChart(createLineSpec([], '总计：0'), {dom: 'model_data'});
            lineChartRef.current.renderAsync();
        }
        if (!pieChartRef.current && document.getElementById('model_pie')) {
            pieChartRef.current = new VChart(createPieSpec([{type: '暂无数据', value: 0}], '总计：0'), {dom: 'model_pie'});
            pieChartRef.current.renderAsync();
        }
    };

    const applyData = useCallback((rows) => {
        createCharts();
        const grid = buildBuckets(filters.start_timestamp, filters.end_timestamp, filters.granularity);
        // 不按标签相等丢弃行：网格只决定补 0 的骨架，响应里出现的标签一律并入 x 轴
        const buckets = buildBucketAxis(grid, rows);
        const models = [...new Set(rows.map((row) => row.ModelName))];

        let quotaSum = 0;
        let requestSum = 0;
        const modelCount = new Map();
        const barQuota = new Map();
        rows.forEach((row) => {
            quotaSum += row.Quota;
            requestSum += row.RequestCount;
            modelCount.set(row.ModelName, (modelCount.get(row.ModelName) || 0) + row.RequestCount);
            const key = `${row.Day}||${row.ModelName}`;
            barQuota.set(key, (barQuota.get(key) || 0) + parseFloat(getQuotaWithUnit(row.Quota)));
        });

        const fillGrid = buckets.length * models.length <= MAX_FILLED_POINTS;
        const lineValues = [];
        buckets.forEach((bucket) => {
            models.forEach((model) => {
                const usage = barQuota.get(`${bucket}||${model}`);
                if (usage === undefined) {
                    if (fillGrid) {
                        lineValues.push({Time: bucket, Model: model, Usage: 0});
                    }
                    return;
                }
                lineValues.push({Time: bucket, Model: model, Usage: usage});
            });
        });

        const pieValues = [...modelCount.entries()]
            .map(([type, value]) => ({type, value}))
            .sort((a, b) => b.value - a.value);

        if (lineChartRef.current) {
            lineChartRef.current.updateSpec(createLineSpec(lineValues, `总计：${renderQuota(quotaSum, 2)}`));
            lineChartRef.current.reLayout();
        }
        if (pieChartRef.current) {
            pieChartRef.current.updateSpec(
                createPieSpec(
                    pieValues.length ? pieValues : [{type: '暂无数据', value: 0}],
                    `总计：${renderNumber(requestSum)}`
                )
            );
            pieChartRef.current.reLayout();
        }
        // 空状态只看响应本身：响应非空但标签与网格不一致时，上面已经把标签并进 x 轴，
        // 不能再报「暂无数据」
        setHasData(rows.length > 0);
    }, [filters]);

    const loadDashboardData = useCallback(async () => {
        const startTimestamp = toUnixSeconds(filters.start_timestamp);
        const endTimestamp = toUnixSeconds(filters.end_timestamp);
        const invalidReason = validateRange(startTimestamp, endTimestamp, filters.granularity);
        if (invalidReason) {
            // 区间不合法就不发请求，图表停留在上一次有效结果上
            showError(invalidReason);
            return;
        }
        const params = {
            start_timestamp: startTimestamp,
            end_timestamp: endTimestamp,
            granularity: filters.granularity,
            scope: isAdminUser ? filters.scope : 'self',
            username: isAdminUser && filters.scope === 'all' ? filters.username : '',
            token_name: filters.token_name,
            model_name: filters.model_name
        };
        setLoading(true);
        try {
            const res = await API.get('/api/user/dashboard', {params});
            const {success, message, data} = res.data;
            if (!success) {
                showError(message);
                return;
            }
            applyData(Array.isArray(data) ? data : []);
        } catch (error) {
            // axios 拦截器已经提示过了，这里保留上一次的图表数据
        } finally {
            setLoaded(true);
            setLoading(false);
        }
    }, [filters, isAdminUser, applyData]);

    const loadCandidates = useCallback(async () => {
        const startTimestamp = toUnixSeconds(filters.start_timestamp);
        const endTimestamp = toUnixSeconds(filters.end_timestamp);
        if (!Number.isFinite(startTimestamp) || !Number.isFinite(endTimestamp)) {
            return;
        }
        const params = {
            start_timestamp: startTimestamp,
            end_timestamp: endTimestamp,
            // 非管理员的 username 后端会忽略；候选值只跟区间与用户有关
            username: isAdminUser && filters.scope === 'all' ? filters.username : ''
        };
        try {
            const res = await API.get('/api/log/filters', {params});
            const {success, data} = res.data;
            if (!success) {
                return;
            }
            const result = data || {};
            setCandidates({
                users: sanitizeCandidates(result.users),
                tokens: sanitizeCandidates(result.tokens),
                models: sanitizeCandidates(result.models)
            });
        } catch (error) {
            // 候选值拉取失败不阻塞总览，退化成只有「全部」可选
            setCandidates({users: [], tokens: [], models: []});
        }
    }, [filters.start_timestamp, filters.end_timestamp, filters.scope, filters.username, isAdminUser]);

    // 挂载时建实例、卸载时释放（与 @visactor/react-vchart 的官方封装同样的生命周期处理）
    useEffect(() => {
        createCharts();
        return () => {
            [lineChartRef, pieChartRef].forEach((ref) => {
                if (ref.current) {
                    ref.current.release();
                    ref.current = null;
                }
            });
        };
    }, []);

    // 区间 / 粒度 / 范围 / 用户 / 令牌 / 模型任一变化都重新取数并更新图表
    useEffect(() => {
        loadDashboardData();
    }, [loadDashboardData]);

    // 候选值只取决于区间 / 范围 / 用户：令牌与模型的选择不会改变候选集
    useEffect(() => {
        loadCandidates();
    }, [loadCandidates]);

    return (
        <>
            <Layout>
                <Layout.Header>
                    <h3>总览</h3>
                </Layout.Header>
                <Layout.Content>
                    {/* 筛选栏：控件一律直接受控于 filters。
                        不用 Form.* 的 field 包装，是因为 withField 会用表单内部状态覆盖
                        传入的 value 且不再回读 props —— 那样程序化清空筛选（切范围/切用户）
                        只在 state 里生效，界面还显示着旧值，出现「看起来生效其实没生效」的筛选。 */}
                    <div style={{marginTop: 10}}>
                        <FilterItem label='起始时间'>
                            <DatePicker type='dateTime' style={{width: 272}}
                                        value={filters.start_timestamp}
                                        onChange={(date, dateString) => {
                                            if (dateString) {
                                                setFilters({start_timestamp: dateString});
                                            }
                                        }}/>
                        </FilterItem>
                        <FilterItem label='结束时间'>
                            <DatePicker type='dateTime' style={{width: 272}}
                                        value={filters.end_timestamp}
                                        onChange={(date, dateString) => {
                                            if (dateString) {
                                                setFilters({end_timestamp: dateString});
                                            }
                                        }}/>
                        </FilterItem>
                        <FilterItem label='时间粒度'>
                            <Select style={{width: 176}}
                                    value={filters.granularity}
                                    optionList={
                                        [
                                            {label: '小时', value: 'hour'},
                                            {label: '天', value: 'day'}
                                        ]
                                    }
                                    onChange={(value) => setFilters({granularity: value})}/>
                        </FilterItem>
                        {
                            isAdminUser && <>
                                <FilterItem label='数据范围'>
                                    <Select style={{width: 176}}
                                            value={filters.scope}
                                            optionList={
                                                [
                                                    {label: '仅自己', value: 'self'},
                                                    {label: '全站', value: 'all'}
                                                ]
                                            }
                                            onChange={(value) => setFilters(scopeChangePatch(value))}/>
                                </FilterItem>
                                {
                                    filters.scope === 'all' &&
                                    <FilterItem label='用户名称'>
                                        <Select style={{width: 176}}
                                                value={filters.username}
                                                filter
                                                placeholder={'全部'}
                                                optionList={buildOptions(withSelected(candidates.users, filters.username))}
                                                onChange={(value) => setFilters(usernameChangePatch(value))}/>
                                    </FilterItem>
                                }
                            </>
                        }
                        <FilterItem label='令牌名称'>
                            <Select style={{width: 176}}
                                    value={filters.token_name}
                                    filter
                                    placeholder={'全部'}
                                    optionList={buildOptions(withSelected(candidates.tokens, filters.token_name))}
                                    onChange={(value) => setFilters({token_name: value})}/>
                        </FilterItem>
                        <FilterItem label='模型名称'>
                            <Select style={{width: 176}}
                                    value={filters.model_name}
                                    filter
                                    placeholder={'全部'}
                                    optionList={buildOptions(withSelected(candidates.models, filters.model_name))}
                                    onChange={(value) => setFilters({model_name: value})}/>
                        </FilterItem>
                        <Button label='查询' type="primary" className="btn-margin-right"
                                onClick={loadDashboardData} loading={loading}>查询</Button>
                    </div>
                    {
                        !loading && loaded && !hasData &&
                        <div style={{
                            textAlign: 'center',
                            color: 'var(--semi-color-text-2)',
                            marginBottom: 10
                        }}>所选区间内暂无数据
                        </div>
                    }
                    <Spin spinning={loading}>
                        <div style={{height: 500}}>
                            <div id="model_pie" style={{width: '100%', minWidth: 100}}></div>
                        </div>
                        <div style={{height: 500}}>
                            <div id="model_data" style={{width: '100%', minWidth: 100}}></div>
                        </div>
                    </Spin>
                </Layout.Content>
            </Layout>
        </>
    );
};


export default Dashboard;
