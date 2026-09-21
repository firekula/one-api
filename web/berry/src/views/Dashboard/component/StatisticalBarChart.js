import PropTypes from 'prop-types';

// material-ui
import { Grid, Typography } from '@mui/material';

// third-party
import Chart from 'react-apexcharts';

// project imports
import SkeletonTotalGrowthBarChart from 'ui-component/cards/Skeleton/TotalGrowthBarChart';
import MainCard from 'ui-component/cards/MainCard';
import { gridSpacing } from 'store/constant';
import { Box } from '@mui/material';

// ==============================|| DASHBOARD DEFAULT - TOTAL GROWTH BAR CHART ||============================== //

const StatisticalBarChart = ({ isLoading, chartDatas }) => {
  // 每次渲染都构造新的 options / series 对象。
  // react-apexcharts 是用 JSON.stringify 比对 props 的：如果继续原地改模块级单例，
  // prevProps.options 与 props.options 是同一个对象、序列化结果必然相同，
  // 它就会判定「只有 series 变了」而只调 updateSeries —— 而 updateSeries 不会更新
  // xaxis.categories，于是切换区间或粒度后柱子数量变了、x 轴标签还停在旧的桶集合上。
  // 每帧给出新对象，才会走 updateOptions 把 x 轴一起更新。
  const chartData = {
    ...baseChartOptions,
    options: {
      ...baseChartOptions.options,
      xaxis: { ...baseChartOptions.options.xaxis, categories: chartDatas?.xaxis || [] }
    },
    series: chartDatas?.data
  };

  return (
    <>
      {isLoading ? (
        <SkeletonTotalGrowthBarChart />
      ) : (
        <MainCard>
          <Grid container spacing={gridSpacing}>
            <Grid item xs={12}>
              <Grid container alignItems="center" justifyContent="space-between">
                <Grid item>
                  <Typography variant="h3">统计</Typography>
                </Grid>
              </Grid>
            </Grid>
            <Grid item xs={12}>
              {chartData.series ? (
                <Chart {...chartData} />
              ) : (
                <Box
                  sx={{
                    minHeight: '490px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center'
                  }}
                >
                  <Typography variant="h3" color={'#697586'}>
                    暂无数据
                  </Typography>
                </Box>
              )}
            </Grid>
          </Grid>
        </MainCard>
      )}
    </>
  );
};

StatisticalBarChart.propTypes = {
  isLoading: PropTypes.bool,
  chartDatas: PropTypes.object
};

export default StatisticalBarChart;

// 基础配置：桶集合（xaxis.categories）与 series 每次渲染单独拼装，这里不再持有它们
const baseChartOptions = {
  height: 480,
  type: 'bar',
  options: {
    colors: [
      '#008FFB',
      '#00E396',
      '#FEB019',
      '#FF4560',
      '#775DD0',
      '#55efc4',
      '#81ecec',
      '#74b9ff',
      '#a29bfe',
      '#00b894',
      '#00cec9',
      '#0984e3',
      '#6c5ce7',
      '#ffeaa7',
      '#fab1a0',
      '#ff7675',
      '#fd79a8',
      '#fdcb6e',
      '#e17055',
      '#d63031',
      '#e84393'
    ],
    chart: {
      id: 'bar-chart',
      stacked: true,
      toolbar: {
        show: true
      },
      zoom: {
        enabled: true
      }
    },
    responsive: [
      {
        breakpoint: 480,
        options: {
          legend: {
            position: 'bottom',
            offsetX: -10,
            offsetY: 0
          }
        }
      }
    ],
    plotOptions: {
      bar: {
        horizontal: false,
        columnWidth: '50%'
      }
    },
    xaxis: {
      type: 'category',
      categories: []
    },
    legend: {
      show: true,
      fontSize: '14px',
      fontFamily: `'Roboto', sans-serif`,
      position: 'bottom',
      offsetX: 20,
      labels: {
        useSeriesColors: false
      },
      markers: {
        width: 16,
        height: 16,
        radius: 5
      },
      itemMargin: {
        horizontal: 15,
        vertical: 8
      }
    },
    fill: {
      type: 'solid'
    },
    dataLabels: {
      enabled: false
    },
    grid: {
      show: true
    },
    tooltip: {
      theme: 'dark',
      fixed: {
        enabled: false
      },
      y: {
        formatter: function (val) {
          return '$' + val;
        }
      },
      marker: {
        show: false
      }
    }
  },
  series: []
};
